package beacontower

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
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
