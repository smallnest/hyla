package beacontower

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestRelayAddrAndPairCodeRoundTrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"127.0.0.1:7474", "127.0.0.1:7474"},
		{"relay.example.com", "relay.example.com:7474"},
		{"tcp://relay.example.com:8443", "tcp://relay.example.com:8443"},
		{"tls://relay.example.com", "tls://relay.example.com:7474"},
		{"wss://relay.example.com", "wss://relay.example.com/ws"},
		{"wss://relay.example.com:8443/custom", "wss://relay.example.com:8443/custom"},
		{"ws://[::1]:7474/ws", "ws://[::1]:7474/ws"},
	} {
		ra, err := ParseRelayAddr(tc.raw)
		if err != nil {
			t.Fatalf("ParseRelayAddr(%q): %v", tc.raw, err)
		}
		if ra.Raw != tc.want {
			t.Errorf("ParseRelayAddr(%q).Raw = %q; want %q", tc.raw, ra.Raw, tc.want)
		}

		code, err := EncodePairCode(PairCode{
			Relay:      tc.raw,
			HostPub:    kp.Pub,
			PSK:        kp.PSK,
			RelayToken: "secret-relay-tok",
		})
		if err != nil {
			t.Fatalf("EncodePairCode(%q): %v", tc.raw, err)
		}
		if !LooksLikePairCode(code) || !ValidPairCode(code) || !ValidPairCode(Scheme+code) {
			t.Fatalf("invalid encoded code %q", code)
		}
		got, err := ParsePairCode(Scheme + code)
		if err != nil {
			t.Fatalf("ParsePairCode(%q): %v", code, err)
		}
		if got.Relay != tc.want || got.HostPub != kp.Pub || got.PSK != kp.PSK || got.RelayToken != "secret-relay-tok" {
			t.Fatalf("roundtrip mismatch: %+v", got)
		}
	}

	for _, bad := range []string{"", "http://example.com", "ftp://x", "tcp://host/path", "relay.example.com:99999", "host with space"} {
		if _, err := ParseRelayAddr(bad); err == nil {
			t.Errorf("ParseRelayAddr(%q) unexpectedly succeeded", bad)
		}
	}
	for _, badCode := range []string{"", "bt1_short", "tc1234567890", Scheme + "bt1_%%%"} {
		if ValidPairCode(badCode) {
			t.Errorf("ValidPairCode(%q) unexpectedly true", badCode)
		}
	}
}

func TestE2EHandshakeAndSecurity(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	replay := NewReplayCache()

	// Normal 1-RTT E2EE handshake + bidirectional traffic + large record chunking.
	cRaw, sRaw := net.Pipe()
	errCh := make(chan error, 2)
	var cConn, sConn net.Conn
	go func() {
		var err error
		sConn, err = ServerE2EHandshake(context.Background(), sRaw, kp, replay)
		errCh <- err
	}()
	go func() {
		var err error
		cConn, err = ClientE2EHandshake(context.Background(), cRaw, kp.Pub, kp.PSK)
		errCh <- err
	}()
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
	defer cConn.Close()
	defer sConn.Close()

	// Send payload larger than maxRecordPlain (32 KiB) to verify chunking & AEAD sequencing.
	big := bytes.Repeat([]byte("beacon-tower-e2ee-frame-"), 2500)
	go func() {
		_, err := cConn.Write(big)
		errCh <- err
	}()
	got := make([]byte, len(big))
	if _, err := io.ReadFull(sConn, got); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, big) {
		t.Fatal("E2EE decrypted payload mismatch")
	}

	// Wrong PSK must fail authentication.
	badPSK := kp.PSK
	badPSK[0] ^= 0xff
	c2, s2 := net.Pipe()
	go func() {
		_, err := ServerE2EHandshake(context.Background(), s2, kp, replay)
		if err == nil {
			errCh <- errors.New("server accepted wrong PSK")
			return
		}
		_ = s2.Close()
		errCh <- nil
	}()
	if _, err := ClientE2EHandshake(context.Background(), c2, kp.Pub, badPSK); err == nil {
		_ = c2.Close()
		t.Fatal("client accepted handshake with wrong PSK")
	}
	_ = c2.Close()
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	// Capture E2E_INIT packet from a valid client and replay it twice to ServerE2EHandshake.
	c3, s3 := net.Pipe()
	captured := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := s3.Read(buf)
		captured <- append([]byte(nil), buf[:n]...)
		_ = s3.Close()
	}()
	_, _ = ClientE2EHandshake(context.Background(), c3, kp.Pub, kp.PSK)
	_ = c3.Close()
	initPkt := <-captured

	runReplayAttempt := func() error {
		cr, sr := net.Pipe()
		go func() {
			_, _ = cr.Write(initPkt)
			_, _ = io.Copy(io.Discard, cr)
		}()
		defer cr.Close()
		conn, err := ServerE2EHandshake(context.Background(), sr, kp, replay)
		if conn != nil {
			_ = conn.Close()
		}
		_ = sr.Close()
		return err
	}
	if err := runReplayAttempt(); err != nil {
		t.Fatalf("first valid replay setup unexpected error: %v", err)
	}
	if err := runReplayAttempt(); err == nil {
		t.Fatal("replayed E2E_INIT packet was accepted a second time")
	}
}

func TestRelayEndToEndTransportsAndMultiplex(t *testing.T) {
	cert, tlsPool := selfSignedCert(t)
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"tcp", "ws", "tls"} {
		t.Run(mode, func(t *testing.T) {
			srvCfg := ServerConfig{
				ListenAddr: "127.0.0.1:0",
				Token:      "relay-secret-123",
			}
			var clientTLS *tls.Config
			if mode == "tls" {
				srvCfg.TLSConfig = &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS13,
				}
				clientTLS = &tls.Config{
					RootCAs:    tlsPool,
					MinVersion: tls.VersionTLS13,
				}
			}

			srv := NewServer(srvCfg)
			rawLn, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ln := rawLn
			if srvCfg.TLSConfig != nil {
				ln = tls.NewListener(rawLn, srvCfg.TLSConfig)
			}
			go func() { _ = srv.Serve(ln) }()
			defer srv.Close()

			addr := rawLn.Addr().String()
			var relayAddr string
			switch mode {
			case "tcp":
				relayAddr = addr
			case "tls":
				relayAddr = "tls://" + addr
			case "ws":
				relayAddr = "ws://" + addr + "/ws"
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			hostLn, err := ListenHost(ctx, HostConfig{
				RelayAddr:  relayAddr,
				RelayToken: "relay-secret-123",
				Keys:       kp,
				TLSConfig:  clientTLS,
			})
			if err != nil {
				t.Fatalf("ListenHost(%s): %v", mode, err)
			}
			defer hostLn.Close()

			// Echo every accepted stream concurrently.
			go func() {
				for {
					c, err := hostLn.Accept()
					if err != nil {
						return
					}
					go func(conn net.Conn) {
						defer conn.Close()
						_, _ = io.Copy(conn, conn)
					}(c)
				}
			}()

			if mode == "ws" {
				resp, err := http.Get("http://" + addr + "/healthz")
				if err != nil {
					t.Fatal(err)
				}
				var st struct {
					OK    bool `json:"ok"`
					Hosts int  `json:"hosts"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
					resp.Body.Close()
					t.Fatal(err)
				}
				resp.Body.Close()
				if !st.OK || st.Hosts != 1 {
					t.Fatalf("healthz unexpected: %+v", st)
				}
			}

			pairCode, err := hostLn.PairCode()
			if err != nil {
				t.Fatal(err)
			}

			pool := NewClientPoolWithTLS(clientTLS)
			defer pool.CloseAll()

			// Multiplex 16 concurrent streams over the single pooled encrypted tunnel,
			// with payload exceeding 256 KiB initial window to verify window updates.
			const streams = 16
			const payloadSize = 300 << 10
			var wg sync.WaitGroup
			wg.Add(streams)
			errs := make(chan error, streams)
			for i := range streams {
				go func(idx int) {
					defer wg.Done()
					openCtx, cancelOpen := context.WithTimeout(ctx, 10*time.Second)
					defer cancelOpen()
					conn, err := pool.Open(openCtx, pairCode, "hyla")
					if err != nil {
						errs <- fmt.Errorf("stream %d open: %w", idx, err)
						return
					}
					defer conn.Close()

					want := bytes.Repeat([]byte{byte('a' + (idx % 26))}, payloadSize)
					writeErr := make(chan error, 1)
					go func() {
						_, werr := conn.Write(want)
						writeErr <- werr
					}()
					got := make([]byte, len(want))
					if _, rerr := io.ReadFull(conn, got); rerr != nil {
						errs <- fmt.Errorf("stream %d read: %w", idx, rerr)
						return
					}
					if werr := <-writeErr; werr != nil {
						errs <- fmt.Errorf("stream %d write: %w", idx, werr)
						return
					}
					if !bytes.Equal(got, want) {
						errs <- fmt.Errorf("stream %d payload mismatch", idx)
						return
					}
				}(i)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
		})
	}
}

func TestRelayAuthRejectionAndAntiHijack(t *testing.T) {
	srv := NewServer(ServerConfig{ListenAddr: "127.0.0.1:0", Token: "correct-token"})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	relayAddr := ln.Addr().String()
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Host with wrong relay token must fail registration.
	if _, err := ListenHost(context.Background(), HostConfig{
		RelayAddr:  relayAddr,
		RelayToken: "wrong-token",
		Keys:       kp,
	}); err == nil {
		t.Fatal("ListenHost with wrong token unexpectedly succeeded")
	}

	// Register legitimate host.
	hostLn, err := ListenHost(context.Background(), HostConfig{
		RelayAddr:  relayAddr,
		RelayToken: "correct-token",
		Keys:       kp,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hostLn.Close()

	// Hijack attempt claiming host's public key with a bogus proof must be rejected
	// and must not evict the legitimate registered host.
	hijackRaw, err := DialRelay(context.Background(), relayAddr)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readServerHello(hijackRaw); err != nil {
		t.Fatal(err)
	}
	var forged [64]byte
	copy(forged[:32], kp.Pub[:])
	forged[63] = 0x42
	if err := writeCtrlMsg(hijackRaw, msgRegister, forged[:]); err != nil {
		t.Fatal(err)
	}
	if err := readOKOrErr(hijackRaw); err == nil {
		_ = hijackRaw.Close()
		t.Fatal("relay accepted forged REGISTER proof")
	}
	_ = hijackRaw.Close()

	// Wrong token in pair code must be rejected by relay.
	badCode, err := EncodePairCode(PairCode{
		Relay:      relayAddr,
		HostPub:    kp.Pub,
		PSK:        kp.PSK,
		RelayToken: "bad-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	pool := NewClientPool()
	defer pool.CloseAll()
	if _, err := pool.Open(context.Background(), badCode, "hyla"); err == nil {
		t.Fatal("client with wrong relay token unexpectedly opened stream")
	}

	// Legitimate client should still reach the host (confirming anti-hijack kept registration intact).
	goodCode, err := hostLn.PairCode()
	if err != nil {
		t.Fatal(err)
	}
	doneAccept := make(chan net.Conn, 1)
	go func() {
		c, _ := hostLn.Accept()
		doneAccept <- c
	}()
	c, err := pool.Open(context.Background(), goodCode, "hyla")
	if err != nil {
		t.Fatalf("legitimate client failed after hijack attempt: %v", err)
	}
	s := <-doneAccept
	if s == nil {
		t.Fatal("host did not accept legitimate connection")
	}
	_ = c.Close()
	_ = s.Close()
}

func TestRelayReconnectAndPocketService(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Bind fixed local port so host reconnects to the restarted relay on the same address.
	probeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probeLn.Addr().String()
	_ = probeLn.Close()

	srv1 := NewServer(ServerConfig{ListenAddr: addr})
	ln1, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv1.Serve(ln1) }()

	pocketMsg := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostLn, err := ListenHost(ctx, HostConfig{
		RelayAddr: addr,
		Keys:      kp,
		OnPocket: func(c net.Conn) {
			defer c.Close()
			buf, _ := io.ReadAll(c)
			pocketMsg <- string(buf)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hostLn.Close()

	code, err := hostLn.PairCode()
	if err != nil {
		t.Fatal(err)
	}
	pool := NewClientPool()
	defer pool.CloseAll()

	// Verify secondary "pocket" target routing over the same E2EE mux.
	pConn, err := pool.Open(ctx, code, "pocket")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pConn.Write([]byte("pocket-ping"))
	_ = pConn.Close()
	if got := <-pocketMsg; got != "pocket-ping" {
		t.Fatalf("pocket got %q", got)
	}

	// Stop relay 1 and start relay 2 on the same address; wait for host reconnect.
	_ = srv1.Close()
	pool.CloseAll()

	srv2 := NewServer(ServerConfig{ListenAddr: addr})
	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv2.Serve(ln2) }()
	defer srv2.Close()

	deadline := time.Now().Add(8 * time.Second)
	for {
		if hosts, _ := srv2.Stats(); hosts == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("host failed to reconnect to restarted relay in time")
		}
		time.Sleep(50 * time.Millisecond)
	}

	doneAccept := make(chan net.Conn, 1)
	go func() {
		c, _ := hostLn.Accept()
		doneAccept <- c
	}()
	c2, err := pool.Open(ctx, code, "hyla")
	if err != nil {
		t.Fatalf("client open after reconnect: %v", err)
	}
	s2 := <-doneAccept
	_, _ = c2.Write([]byte("after-reconnect"))
	_ = c2.Close()
	got, _ := io.ReadAll(s2)
	_ = s2.Close()
	if !strings.EqualFold(string(got), "after-reconnect") {
		t.Fatalf("got %q", got)
	}
}

func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certObj, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(certObj)
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, pool
}

func TestStripSchemePocketURLAndWSURL(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	code, err := EncodePairCode(PairCode{
		Relay:      "wss://relay.example.com/ws",
		HostPub:    kp.Pub,
		PSK:        kp.PSK,
		RelayToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	ra, err := ParseRelayAddr("wss://relay.example.com/ws")
	if err != nil {
		t.Fatal(err)
	}
	if ra.WSURL() != "wss://relay.example.com/ws" {
		t.Fatalf("WSURL() = %q; want wss://relay.example.com/ws", ra.WSURL())
	}
	for _, wrapped := range []string{
		code,
		Scheme + code,
		"hyla://beacon?code=" + code + "&name=studio",
		"hyla://pair?code=" + code + "&name=studio",
		"hyla://pair?code=" + Scheme + code + "&name=studio",
	} {
		if !ValidPairCode(wrapped) {
			t.Errorf("ValidPairCode(%q) = false", wrapped)
		}
		if StripScheme(wrapped) != code {
			t.Errorf("StripScheme(%q) = %q; want %q", wrapped, StripScheme(wrapped), code)
		}
	}
	if ValidPairCode("hyla://pair?code=tcQ4Lq8sG8GrCx9Tu9Px_Gq2IsWbZjWmMx1Qs3Vk1_2&name=studio") {
		t.Fatal("tailcat hyla://pair URL accepted as BeaconTower")
	}
}

func TestMuxOpenErrFramingIntegrity(t *testing.T) {
	cRaw, sRaw := net.Pipe()
	defer cRaw.Close()
	defer sRaw.Close()

	// Host side uses Mux directly; client side writes raw mux frames to trigger
	// bad stream id and duplicate stream id errors before opening a valid stream.
	hostMux := NewMux(sRaw, false)
	defer hostMux.Close()

	go func() {
		for {
			st, _, err := hostMux.AcceptStream()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(st)
		}
	}()

	writeRawFrame := func(typ byte, id, val uint32, payload []byte) error {
		buf := make([]byte, muxHeaderLen+len(payload))
		buf[0] = typ
		binary.BigEndian.PutUint32(buf[1:5], id)
		binary.BigEndian.PutUint32(buf[5:9], val)
		copy(buf[9:], payload)
		_, err := cRaw.Write(buf)
		return err
	}
	readRawFrame := func() (byte, uint32, []byte, error) {
		var hdr [muxHeaderLen]byte
		if _, err := io.ReadFull(cRaw, hdr[:]); err != nil {
			return 0, 0, nil, err
		}
		typ := hdr[0]
		id := binary.BigEndian.Uint32(hdr[1:5])
		val := binary.BigEndian.Uint32(hdr[5:9])
		var payload []byte
		if (typ == muxOpen || typ == muxOpenErr || typ == muxData) && val > 0 {
			payload = make([]byte, val)
			if _, err := io.ReadFull(cRaw, payload); err != nil {
				return 0, 0, nil, err
			}
		}
		return typ, id, payload, nil
	}

	// 1. Trigger "bad stream id" (even ID from client).
	if err := writeRawFrame(muxOpen, 2, 4, []byte("hyla")); err != nil {
		t.Fatal(err)
	}
	typ, id, payload, err := readRawFrame()
	if err != nil || typ != muxOpenErr || id != 2 || string(payload) != "bad stream id" {
		t.Fatalf("bad stream id reply = (typ=%d id=%d %q err=%v)", typ, id, payload, err)
	}

	// 2. Open valid stream 1, then send duplicate OPEN for stream 1.
	if err := writeRawFrame(muxOpen, 1, 4, []byte("hyla")); err != nil {
		t.Fatal(err)
	}
	typ, id, _, err = readRawFrame()
	if err != nil || typ != muxOpenOK || id != 1 {
		t.Fatalf("open stream 1 reply = (typ=%d id=%d err=%v)", typ, id, err)
	}
	if err := writeRawFrame(muxOpen, 1, 4, []byte("hyla")); err != nil {
		t.Fatal(err)
	}
	typ, id, payload, err = readRawFrame()
	if err != nil || typ != muxOpenErr || id != 1 || string(payload) != "duplicate id" {
		t.Fatalf("duplicate id reply = (typ=%d id=%d %q err=%v)", typ, id, payload, err)
	}

	// 3. Framing must remain intact on stream 1.
	if err := writeRawFrame(muxData, 1, 5, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	typ, id, payload, err = readRawFrame()
	if err != nil || typ != muxData || id != 1 || string(payload) != "hello" {
		t.Fatalf("echo after errors = (typ=%d id=%d %q err=%v)", typ, id, payload, err)
	}
}

func TestMuxFastCloseAndGracefulFin(t *testing.T) {
	cRaw, sRaw := net.Pipe()
	cMux := NewMux(cRaw, true)
	sMux := NewMux(sRaw, false)
	defer cMux.Close()
	defer sMux.Close()

	go func() {
		for {
			st, _, err := sMux.AcceptStream()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				_, _ = conn.Write([]byte("short-reply!"))
				_ = conn.Close()
			}(st)
		}
	}()

	for i := range 50 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		st, err := cMux.OpenStream(ctx, "pocket")
		cancel()
		if err != nil {
			t.Fatalf("iter %d OpenStream: %v", i, err)
		}
		got, err := io.ReadAll(st)
		_ = st.Close()
		if err != nil {
			t.Fatalf("iter %d ReadAll: %v", i, err)
		}
		if string(got) != "short-reply!" {
			t.Fatalf("iter %d got %q; want short-reply!", i, got)
		}
	}
}

func TestMuxStreamDeadlines(t *testing.T) {
	cRaw, sRaw := net.Pipe()
	cMux := NewMux(cRaw, true)
	sMux := NewMux(sRaw, false)
	defer cMux.Close()
	defer sMux.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		st, _, err := sMux.AcceptStream()
		if err == nil {
			accepted <- st
		}
	}()

	cSt, err := cMux.OpenStream(context.Background(), "hyla")
	if err != nil {
		t.Fatal(err)
	}
	defer cSt.Close()
	sSt := <-accepted
	defer sSt.Close()

	// 1. Waking an already-blocked Read via SetReadDeadline(time.Now()).
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 16)
		_, err := cSt.Read(buf)
		readDone <- err
	}()
	time.Sleep(30 * time.Millisecond)
	_ = cSt.SetReadDeadline(time.Now())
	select {
	case err := <-readDone:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("blocked Read after SetReadDeadline(now) = %v; want os.ErrDeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SetReadDeadline(now) did not wake blocked Read")
	}

	// Clearing read deadline allows subsequent reads to succeed.
	_ = cSt.SetReadDeadline(time.Time{})
	if _, err := sSt.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(cSt, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("Read after clearing deadline = %q, %v", buf, err)
	}

	// 2. Expired write deadline must fail even when send window > 0.
	_ = cSt.SetWriteDeadline(time.Now().Add(-time.Second))
	if n, err := cSt.Write([]byte("x")); !errors.Is(err, os.ErrDeadlineExceeded) || n != 0 {
		t.Fatalf("Write with expired deadline and available window = (%d, %v); want (0, ErrDeadlineExceeded)", n, err)
	}
	_ = cSt.SetWriteDeadline(time.Time{})

	// 3. Waking a window-blocked Write via SetWriteDeadline(time.Now()).
	writeDone := make(chan error, 1)
	go func() {
		big := make([]byte, muxInitWindow+64*1024)
		_, err := cSt.Write(big)
		writeDone <- err
	}()
	time.Sleep(40 * time.Millisecond)
	_ = cSt.SetWriteDeadline(time.Now())
	select {
	case err := <-writeDone:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("window-blocked Write after SetWriteDeadline(now) = %v; want os.ErrDeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SetWriteDeadline(now) did not wake blocked Write")
	}
}

type failOnceConn struct {
	net.Conn
	failNextWrite atomic.Bool
}

func (f *failOnceConn) Write(p []byte) (int, error) {
	if f.failNextWrite.Swap(false) {
		return 0, errors.New("injected write error")
	}
	return f.Conn.Write(p)
}

func TestAEADConnErrorLatchAndWSFullRecord(t *testing.T) {
	// 1. Test full maxRecordPlain (32 KiB) over ws:// without hitting 32,768B WebSocket read limit.
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(ServerConfig{ListenAddr: "127.0.0.1:0"})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	wsURL := "ws://" + ln.Addr().String() + "/ws"
	replay := NewReplayCache()
	hostRaw, err := DialRelay(context.Background(), wsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer hostRaw.Close()
	relayPub, nonce, _, err := readServerHello(hostRaw)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := RegisterProof(kp.Priv, relayPub, nonce, TokenDigest(""))
	if err != nil {
		t.Fatal(err)
	}
	var reg [64]byte
	copy(reg[:32], kp.Pub[:])
	copy(reg[32:64], proof[:])
	if err := writeCtrlMsg(hostRaw, msgRegister, reg[:]); err != nil {
		t.Fatal(err)
	}
	if err := readOKOrErr(hostRaw); err != nil {
		t.Fatal(err)
	}

	hostJoined := make(chan net.Conn, 1)
	go func() {
		for {
			typ, payload, err := readCtrlMsg(hostRaw)
			if err != nil {
				return
			}
			if typ == msgInvite && len(payload) == 16 {
				var sid [16]byte
				copy(sid[:], payload)
				jRaw, err := DialRelay(context.Background(), wsURL)
				if err != nil {
					return
				}
				_, jNonce, _, err := readServerHello(jRaw)
				if err != nil {
					jRaw.Close()
					return
				}
				jp := JoinTokenProof(TokenDigest(""), jNonce, sid)
				var jm [48]byte
				copy(jm[:16], sid[:])
				copy(jm[16:48], jp[:])
				if err := writeCtrlMsg(jRaw, msgJoin, jm[:]); err != nil {
					jRaw.Close()
					return
				}
				if err := readOKOrErr(jRaw); err != nil {
					jRaw.Close()
					return
				}
				e2e, err := ServerE2EHandshake(context.Background(), jRaw, kp, replay)
				if err != nil {
					jRaw.Close()
					return
				}
				hostJoined <- e2e
				return
			}
		}
	}()

	cliRaw, err := DialRelay(context.Background(), wsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer cliRaw.Close()
	_, cNonce, _, err := readServerHello(cliRaw)
	if err != nil {
		t.Fatal(err)
	}
	hid := HostIDFromPub(kp.Pub)
	cp := ClientTokenProof(TokenDigest(""), cNonce, hid)
	var cm [48]byte
	copy(cm[:16], hid[:])
	copy(cm[16:48], cp[:])
	if err := writeCtrlMsg(cliRaw, msgConnect, cm[:]); err != nil {
		t.Fatal(err)
	}
	if err := readOKOrErr(cliRaw); err != nil {
		t.Fatal(err)
	}
	cE2E, err := ClientE2EHandshake(context.Background(), cliRaw, kp.Pub, kp.PSK)
	if err != nil {
		t.Fatal(err)
	}
	defer cE2E.Close()
	sE2E := <-hostJoined
	defer sE2E.Close()

	// Exactly 32 KiB payload produces a 32,788-byte WebSocket binary message.
	fullRec := bytes.Repeat([]byte{0x5a}, maxRecordPlain)
	errCh := make(chan error, 1)
	go func() {
		_, werr := cE2E.Write(fullRec)
		errCh <- werr
	}()
	gotRec := make([]byte, len(fullRec))
	if _, err := io.ReadFull(sE2E, gotRec); err != nil {
		t.Fatalf("ws 32KiB AEAD record ReadFull: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ws 32KiB AEAD record Write: %v", err)
	}
	if !bytes.Equal(gotRec, fullRec) {
		t.Fatal("ws 32KiB payload mismatch")
	}

	// 2. Test write error latching to ensure nonce is never reused.
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()
	go func() { _, _ = io.Copy(io.Discard, p2) }()
	fc := &failOnceConn{Conn: p1}
	key := make([]byte, chacha20poly1305.KeySize)
	ac, err := newAEADConn(fc, key, key, dirClientToHost, dirHostToClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ac.Write([]byte("ok-1")); err != nil {
		t.Fatal(err)
	}
	fc.failNextWrite.Store(true)
	if _, err := ac.Write([]byte("fail-2")); err == nil {
		t.Fatal("expected injected write error")
	}
	if _, err := ac.Write([]byte("must-stay-latched")); err == nil {
		t.Fatal("aeadConn allowed Write after write error (risk of nonce reuse)")
	}
}

func TestReplayCacheOldestEvictionAndTimestampOverflow(t *testing.T) {
	rc := NewReplayCache()
	base := time.Now()
	var keys [][32]byte
	for i := range maxReplaySize + 5 {
		var k [32]byte
		binary.BigEndian.PutUint32(k[:], uint32(i+1))
		keys = append(keys, k)
		if !rc.CheckAndAdd(k, base.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatalf("unexpected duplicate for fresh key %d", i)
		}
	}
	// Newest maxReplaySize keys must still be tracked and rejected on replay.
	newestIdx := len(keys) - 1
	if rc.CheckAndAdd(keys[newestIdx], base.Add(time.Duration(newestIdx+1)*time.Millisecond)) {
		t.Fatal("newest key was evicted from full replay cache")
	}
	// Oldest key (index 0) was deterministically evicted.
	if !rc.CheckAndAdd(keys[0], base.Add(time.Duration(len(keys))*time.Millisecond)) {
		t.Fatal("oldest key should have been evicted")
	}

	// Huge timestamps that overflow int64 nanoseconds multiplication must be rejected.
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	for _, bogusTS := range []uint64{math.MaxUint64, math.MaxInt64, 1 << 62} {
		cRaw, sRaw := net.Pipe()
		ceKey, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		var cePriv, cePub [32]byte
		copy(cePriv[:], ceKey.Bytes())
		copy(cePub[:], ceKey.PublicKey().Bytes())
		dhES, err := x25519Shared(cePriv, kp.Pub)
		if err != nil {
			t.Fatal(err)
		}
		hsKey, err := deriveHandshakeKey(kp.PSK, dhES, kp.Pub, cePub)
		if err != nil {
			t.Fatal(err)
		}
		hsAEAD, err := chacha20poly1305.New(hsKey)
		if err != nil {
			t.Fatal(err)
		}
		var initPlain [24]byte
		binary.BigEndian.PutUint64(initPlain[:8], bogusTS)
		_, _ = rand.Read(initPlain[8:])
		var zeroNonce [chacha20poly1305.NonceSize]byte
		ct := hsAEAD.Seal(nil, zeroNonce[:], initPlain[:], buildInitAAD(kp.Pub, cePub))
		pkt := make([]byte, 1+32+2+len(ct))
		pkt[0] = e2eVersion
		copy(pkt[1:33], cePub[:])
		binary.BigEndian.PutUint16(pkt[33:35], uint16(len(ct)))
		copy(pkt[35:], ct)
		go func() {
			_, _ = cRaw.Write(pkt)
			_ = cRaw.Close()
		}()
		if conn, err := ServerE2EHandshake(context.Background(), sRaw, kp, NewReplayCache()); err == nil {
			_ = conn.Close()
			t.Fatalf("bogus timestamp %d bypassed skew check", bogusTS)
		}
		_ = sRaw.Close()
	}
}

func TestRelayOptionalTokenAndCloseCleanup(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Open relay (no token) must accept host even if host has a leftover token.
	openSrv := NewServer(ServerConfig{ListenAddr: "127.0.0.1:0"})
	openLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = openSrv.Serve(openLn) }()
	openHost, err := ListenHost(context.Background(), HostConfig{
		RelayAddr:  openLn.Addr().String(),
		RelayToken: "stale-unused-token",
		Keys:       kp,
	})
	if err != nil {
		t.Fatalf("ListenHost against open relay with stale token failed: %v", err)
	}
	_ = openHost.Close()
	_ = openSrv.Close()

	// 2. Token-required relay rejects tokenless host with clear error.
	authSrv := NewServer(ServerConfig{ListenAddr: "127.0.0.1:0", Token: "req-tok"})
	authLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = authSrv.Serve(authLn) }()
	if _, err := ListenHost(context.Background(), HostConfig{
		RelayAddr: authLn.Addr().String(),
		Keys:      kp,
	}); err == nil || !strings.Contains(err.Error(), "access token") {
		t.Fatalf("expected missing access token error, got %v", err)
	}

	// 3. Server.Close closes active bridged sessions cleanly.
	hostLn, err := ListenHost(context.Background(), HostConfig{
		RelayAddr:  authLn.Addr().String(),
		RelayToken: "req-tok",
		Keys:       kp,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hostLn.Close()
	go func() {
		for {
			c, err := hostLn.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				_, _ = io.Copy(conn, conn)
				_ = conn.Close()
			}(c)
		}
	}()

	code, err := hostLn.PairCode()
	if err != nil {
		t.Fatal(err)
	}
	pool := NewClientPool()
	defer pool.CloseAll()
	st, err := pool.Open(context.Background(), code, "hyla")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	var pong [2]byte
	if _, err := io.ReadFull(st, pong[:]); err != nil {
		t.Fatal(err)
	}
	if hosts, sessions := authSrv.Stats(); hosts != 1 || sessions != 1 {
		t.Fatalf("Stats before Close = (%d, %d); want (1, 1)", hosts, sessions)
	}
	_ = authSrv.Close()
	readEnded := make(chan error, 1)
	go func() {
		var b [8]byte
		_, rerr := st.Read(b[:])
		readEnded <- rerr
	}()
	select {
	case rerr := <-readEnded:
		if rerr == nil {
			t.Fatal("expected stream Read error after Server.Close()")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active bridged stream remained open after Server.Close()")
	}

	// 4. ClientPool.CloseCode cancels inflight dials cleanly.
	stallLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer stallLn.Close()
	go func() {
		for {
			c, err := stallLn.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	stallCode, err := EncodePairCode(PairCode{
		Relay:   stallLn.Addr().String(),
		HostPub: kp.Pub,
		PSK:     kp.PSK,
	})
	if err != nil {
		t.Fatal(err)
	}
	dialDone := make(chan error, 1)
	go func() {
		_, oerr := pool.Open(context.Background(), stallCode, "hyla")
		dialDone <- oerr
	}()
	time.Sleep(40 * time.Millisecond)
	pool.CloseCode(stallCode)
	select {
	case oerr := <-dialDone:
		if oerr == nil {
			t.Fatal("expected inflight dial error on CloseCode")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CloseCode did not cancel inflight dial")
	}
}
