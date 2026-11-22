package beacontower

import (
	"context"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	e2eVersion     byte = 1
	maxRecordPlain      = 32 << 10
	maxClockSkew        = 5 * time.Minute
	replayWindow        = 10 * time.Minute
	maxReplaySize       = 16384

	dirClientToHost byte = 1
	dirHostToClient byte = 2
)

// KeyPair holds a host's long-term X25519 identity keypair and pre-shared key.
type KeyPair struct {
	Priv [32]byte
	Pub  [32]byte
	PSK  [32]byte
}

// GenerateKeyPair creates a fresh X25519 keypair and 256-bit pre-shared key.
func GenerateKeyPair() (KeyPair, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	var kp KeyPair
	copy(kp.Priv[:], priv.Bytes())
	copy(kp.Pub[:], priv.PublicKey().Bytes())
	if _, err := rand.Read(kp.PSK[:]); err != nil {
		return KeyPair{}, err
	}
	return kp, nil
}

// PublicFromPrivate derives the X25519 public key from a 32-byte private key.
func PublicFromPrivate(priv [32]byte) ([32]byte, error) {
	k, err := ecdh.X25519().NewPrivateKey(priv[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("beacontower: invalid private key: %w", err)
	}
	var pub [32]byte
	copy(pub[:], k.PublicKey().Bytes())
	return pub, nil
}

// PublicFromPrivateOrCheckPub validates that pub is a valid non-zero X25519 key.
func PublicFromPrivateOrCheckPub(pub [32]byte) ([32]byte, error) {
	var zero [32]byte
	if pub == zero {
		return zero, errors.New("beacontower: empty public key")
	}
	if _, err := ecdh.X25519().NewPublicKey(pub[:]); err != nil {
		return zero, fmt.Errorf("beacontower: invalid public key: %w", err)
	}
	return pub, nil
}

func x25519Shared(priv, pub [32]byte) ([]byte, error) {
	k, err := ecdh.X25519().NewPrivateKey(priv[:])
	if err != nil {
		return nil, err
	}
	p, err := ecdh.X25519().NewPublicKey(pub[:])
	if err != nil {
		return nil, err
	}
	return k.ECDH(p)
}

// TokenDigest hashes an optional relay access token so plaintext tokens never cross the wire.
func TokenDigest(token string) [32]byte {
	if token == "" {
		return [32]byte{}
	}
	buf := make([]byte, 9+len(token))
	copy(buf[:9], "bt-tok-v1")
	copy(buf[9:], token)
	return sha256.Sum256(buf)
}

// RegisterProof proves ownership of hostPriv (preventing HostID hijacking) and
// possession of the relay token against the relay's ephemeral challenge.
func RegisterProof(hostPriv, relayEphPub, nonce, tokenDigest [32]byte) ([32]byte, error) {
	ss, err := x25519Shared(hostPriv, relayEphPub)
	if err != nil {
		return [32]byte{}, err
	}
	return computeRegisterHMAC(ss, nonce, tokenDigest), nil
}

// VerifyRegisterProof verifies a host's REGISTER proof on the relay side.
func VerifyRegisterProof(relayEphPriv, hostPub, nonce, tokenDigest, proof [32]byte) bool {
	ss, err := x25519Shared(relayEphPriv, hostPub)
	if err != nil {
		return false
	}
	want := computeRegisterHMAC(ss, nonce, tokenDigest)
	return hmac.Equal(want[:], proof[:])
}

func computeRegisterHMAC(ss []byte, nonce, tokenDigest [32]byte) [32]byte {
	mac := hmac.New(sha256.New, ss)
	mac.Write([]byte("bt-reg-v1"))
	mac.Write(nonce[:])
	mac.Write(tokenDigest[:])
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

// ClientTokenProof binds a CONNECT request to the relay challenge nonce and target HostID.
func ClientTokenProof(tokenDigest, nonce [32]byte, hostID [16]byte) [32]byte {
	mac := hmac.New(sha256.New, tokenDigest[:])
	mac.Write([]byte("bt-cli-v1"))
	mac.Write(nonce[:])
	mac.Write(hostID[:])
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

// JoinTokenProof binds a JOIN request to the relay challenge nonce and sessionID.
func JoinTokenProof(tokenDigest, nonce [32]byte, sessionID [16]byte) [32]byte {
	mac := hmac.New(sha256.New, tokenDigest[:])
	mac.Write([]byte("bt-join-v1"))
	mac.Write(nonce[:])
	mac.Write(sessionID[:])
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

// ReplayCache tracks recently seen ephemeral client handshake keys to thwart
// E2EE handshake replay attacks within the allowed clock-skew window.
type ReplayCache struct {
	mu   sync.Mutex
	seen map[[32]byte]time.Time
}

// NewReplayCache creates a bounded replay cache for host E2EE handshakes.
func NewReplayCache() *ReplayCache {
	return &ReplayCache{seen: make(map[[32]byte]time.Time)}
}

// CheckAndAdd returns true iff key has not been seen inside the replay window.
func (rc *ReplayCache) CheckAndAdd(key [32]byte, now time.Time) bool {
	if rc == nil {
		return true
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	cutoff := now.Add(-replayWindow)
	if len(rc.seen) >= maxReplaySize {
		for k, t := range rc.seen {
			if t.Before(cutoff) {
				delete(rc.seen, k)
			}
		}
		if len(rc.seen) >= maxReplaySize {
			// Evict oldest arbitrary entry to stay strictly bounded under flood.
			for k := range rc.seen {
				delete(rc.seen, k)
				if len(rc.seen) < maxReplaySize {
					break
				}
			}
		}
	} else if len(rc.seen) > 128 && len(rc.seen)%64 == 0 {
		for k, t := range rc.seen {
			if t.Before(cutoff) {
				delete(rc.seen, k)
			}
		}
	}
	if prev, ok := rc.seen[key]; ok && !prev.Before(cutoff) {
		return false
	}
	rc.seen[key] = now
	return true
}

// ClientE2EHandshake performs the 1-RTT Noise_IKpsk2-style E2EE handshake on a
// relay-bridged connection and returns an encrypted net.Conn.
func ClientE2EHandshake(ctx context.Context, raw net.Conn, hostPub, psk [32]byte) (net.Conn, error) {
	if _, err := PublicFromPrivateOrCheckPub(hostPub); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer func() { _ = stop() }()

	ceKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	var cePriv, cePub [32]byte
	copy(cePriv[:], ceKey.Bytes())
	copy(cePub[:], ceKey.PublicKey().Bytes())

	dhES, err := x25519Shared(cePriv, hostPub)
	if err != nil {
		return nil, err
	}
	hsKey, err := deriveHandshakeKey(psk, dhES, hostPub, cePub)
	if err != nil {
		return nil, err
	}
	hsAEAD, err := chacha20poly1305.New(hsKey)
	if err != nil {
		return nil, err
	}

	var initPlain [24]byte
	binary.BigEndian.PutUint64(initPlain[0:8], uint64(time.Now().Unix()))
	if _, err := rand.Read(initPlain[8:24]); err != nil {
		return nil, err
	}
	var clientNonce [16]byte
	copy(clientNonce[:], initPlain[8:24])

	var zeroNonce [chacha20poly1305.NonceSize]byte
	initAAD := buildInitAAD(hostPub, cePub)
	ct := hsAEAD.Seal(nil, zeroNonce[:], initPlain[:], initAAD)

	pkt := make([]byte, 1+32+2+len(ct))
	pkt[0] = e2eVersion
	copy(pkt[1:33], cePub[:])
	binary.BigEndian.PutUint16(pkt[33:35], uint16(len(ct)))
	copy(pkt[35:], ct)
	if _, err := raw.Write(pkt); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e write init: %w", err)
	}

	// Read server E2E_RESP: [1B ver][32B hePub][2B ctLen][ct]
	var hdr [35]byte
	if _, err := io.ReadFull(raw, hdr[:]); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e read resp: %w", err)
	}
	if hdr[0] != e2eVersion {
		return nil, fmt.Errorf("beacontower e2e: unsupported version %d", hdr[0])
	}
	var hePub [32]byte
	copy(hePub[:], hdr[1:33])
	respCTLen := int(binary.BigEndian.Uint16(hdr[33:35]))
	if respCTLen < chacha20poly1305.Overhead || respCTLen > 256 {
		return nil, fmt.Errorf("beacontower e2e: invalid response length %d", respCTLen)
	}
	respCT := make([]byte, respCTLen)
	if _, err := io.ReadFull(raw, respCT); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e read resp body: %w", err)
	}

	dhEE, err := x25519Shared(cePriv, hePub)
	if err != nil {
		return nil, err
	}
	keyC2H, keyH2C, tr, err := deriveSessionKeys(psk, dhES, dhEE, hostPub, cePub, hePub)
	if err != nil {
		return nil, err
	}
	h2cAEAD, err := chacha20poly1305.New(keyH2C)
	if err != nil {
		return nil, err
	}
	respPlain, err := h2cAEAD.Open(nil, zeroNonce[:], respCT, buildRespAAD(tr))
	if err != nil {
		return nil, errors.New("beacontower e2e: server authentication failed")
	}
	if len(respPlain) != len(clientNonce) || subtle.ConstantTimeCompare(respPlain, clientNonce[:]) != 1 {
		return nil, errors.New("beacontower e2e: server challenge mismatch")
	}
	if !stop() && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return newAEADConn(raw, keyC2H, keyH2C, dirClientToHost, dirHostToClient)
}

// ServerE2EHandshake verifies a client's E2E_INIT handshake, completes mutual
// key derivation with a fresh ephemeral key for PFS, and returns an encrypted net.Conn.
func ServerE2EHandshake(ctx context.Context, raw net.Conn, kp KeyPair, replay *ReplayCache) (net.Conn, error) {
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer func() { _ = stop() }()

	var hdr [35]byte
	if _, err := io.ReadFull(raw, hdr[:]); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e read init: %w", err)
	}
	if hdr[0] != e2eVersion {
		return nil, fmt.Errorf("beacontower e2e: unsupported version %d", hdr[0])
	}
	var cePub [32]byte
	copy(cePub[:], hdr[1:33])
	ctLen := int(binary.BigEndian.Uint16(hdr[33:35]))
	if ctLen != 24+chacha20poly1305.Overhead {
		return nil, fmt.Errorf("beacontower e2e: unexpected init size %d", ctLen)
	}
	ct := make([]byte, ctLen)
	if _, err := io.ReadFull(raw, ct); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e read init body: %w", err)
	}

	dhES, err := x25519Shared(kp.Priv, cePub)
	if err != nil {
		return nil, fmt.Errorf("beacontower e2e: %w", err)
	}
	hsKey, err := deriveHandshakeKey(kp.PSK, dhES, kp.Pub, cePub)
	if err != nil {
		return nil, err
	}
	hsAEAD, err := chacha20poly1305.New(hsKey)
	if err != nil {
		return nil, err
	}
	var zeroNonce [chacha20poly1305.NonceSize]byte
	initPlain, err := hsAEAD.Open(nil, zeroNonce[:], ct, buildInitAAD(kp.Pub, cePub))
	if err != nil || len(initPlain) != 24 {
		return nil, errors.New("beacontower e2e: client authentication failed")
	}

	now := time.Now()
	ts := int64(binary.BigEndian.Uint64(initPlain[0:8]))
	skew := now.Unix() - ts
	if skew < 0 {
		skew = -skew
	}
	if time.Duration(skew)*time.Second > maxClockSkew {
		return nil, errors.New("beacontower e2e: handshake timestamp outside allowed skew")
	}
	if !replay.CheckAndAdd(cePub, now) {
		return nil, errors.New("beacontower e2e: replayed handshake rejected")
	}
	clientNonce := initPlain[8:24]

	heKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	var hePriv, hePub [32]byte
	copy(hePriv[:], heKey.Bytes())
	copy(hePub[:], heKey.PublicKey().Bytes())

	dhEE, err := x25519Shared(hePriv, cePub)
	if err != nil {
		return nil, err
	}
	keyC2H, keyH2C, tr, err := deriveSessionKeys(kp.PSK, dhES, dhEE, kp.Pub, cePub, hePub)
	if err != nil {
		return nil, err
	}
	h2cAEAD, err := chacha20poly1305.New(keyH2C)
	if err != nil {
		return nil, err
	}
	respCT := h2cAEAD.Seal(nil, zeroNonce[:], clientNonce, buildRespAAD(tr))
	pkt := make([]byte, 1+32+2+len(respCT))
	pkt[0] = e2eVersion
	copy(pkt[1:33], hePub[:])
	binary.BigEndian.PutUint16(pkt[33:35], uint16(len(respCT)))
	copy(pkt[35:], respCT)
	if _, err := raw.Write(pkt); err != nil {
		if !stop() && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("beacontower e2e write resp: %w", err)
	}
	if !stop() && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return newAEADConn(raw, keyH2C, keyC2H, dirHostToClient, dirClientToHost)
}

func deriveHandshakeKey(psk [32]byte, dhES []byte, hostPub, cePub [32]byte) ([]byte, error) {
	info := make([]byte, 12+32+32)
	copy(info[:12], "bt-e2e-hs-v1")
	copy(info[12:44], hostPub[:])
	copy(info[44:76], cePub[:])
	return hkdf.Key(sha256.New, dhES, psk[:], string(info), chacha20poly1305.KeySize)
}

func buildInitAAD(hostPub, cePub [32]byte) []byte {
	aad := make([]byte, 14+32+32)
	copy(aad[:14], "bt-e2e-init-v1")
	copy(aad[14:46], hostPub[:])
	copy(aad[46:78], cePub[:])
	return aad
}

func buildRespAAD(tr [32]byte) []byte {
	aad := make([]byte, 14+32)
	copy(aad[:14], "bt-e2e-resp-v1")
	copy(aad[14:], tr[:])
	return aad
}

func deriveSessionKeys(psk [32]byte, dhES, dhEE []byte, hostPub, cePub, hePub [32]byte) (c2h, h2c []byte, tr [32]byte, err error) {
	ikm := make([]byte, 0, len(dhES)+len(dhEE))
	ikm = append(ikm, dhES...)
	ikm = append(ikm, dhEE...)
	prk, err := hkdf.Extract(sha256.New, ikm, psk[:])
	if err != nil {
		return nil, nil, tr, err
	}
	trIn := make([]byte, 12+32+32+32)
	copy(trIn[:12], "bt-e2e-tr-v1")
	copy(trIn[12:44], hostPub[:])
	copy(trIn[44:76], cePub[:])
	copy(trIn[76:108], hePub[:])
	tr = sha256.Sum256(trIn)

	infoC2H := append([]byte("bt-c2h"), tr[:]...)
	infoH2C := append([]byte("bt-h2c"), tr[:]...)
	c2h, err = hkdf.Expand(sha256.New, prk, string(infoC2H), chacha20poly1305.KeySize)
	if err != nil {
		return nil, nil, tr, err
	}
	h2c, err = hkdf.Expand(sha256.New, prk, string(infoH2C), chacha20poly1305.KeySize)
	if err != nil {
		return nil, nil, tr, err
	}
	return c2h, h2c, tr, nil
}

type aeadConn struct {
	net.Conn

	rmu    sync.Mutex
	raead  cipher.AEAD
	rseq   uint64
	rdir   byte
	rbuf   []byte
	rawBuf []byte

	wmu   sync.Mutex
	waead cipher.AEAD
	wseq  uint64
	wdir  byte
	wbuf  []byte
}

func newAEADConn(raw net.Conn, writeKey, readKey []byte, writeDir, readDir byte) (*aeadConn, error) {
	wa, err := chacha20poly1305.New(writeKey)
	if err != nil {
		return nil, err
	}
	ra, err := chacha20poly1305.New(readKey)
	if err != nil {
		return nil, err
	}
	return &aeadConn{
		Conn:   raw,
		raead:  ra,
		rseq:   1, // seq 0 was reserved for E2E_RESP
		rdir:   readDir,
		waead:  wa,
		wseq:   1,
		wdir:   writeDir,
		rawBuf: make([]byte, maxRecordPlain+chacha20poly1305.Overhead),
		wbuf:   make([]byte, 4+maxRecordPlain+chacha20poly1305.Overhead),
	}, nil
}

func makeRecordNonceAndAAD(dir byte, seq uint64, nonce *[12]byte, aad *[9]byte) {
	*nonce = [12]byte{}
	binary.BigEndian.PutUint64(nonce[4:12], seq)
	aad[0] = dir
	binary.BigEndian.PutUint64(aad[1:9], seq)
}

func (c *aeadConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	c.rmu.Lock()
	defer c.rmu.Unlock()
	for len(c.rbuf) == 0 {
		var lenBuf [4]byte
		if _, err := io.ReadFull(c.Conn, lenBuf[:]); err != nil {
			return 0, err
		}
		ctLen := int(binary.BigEndian.Uint32(lenBuf[:]))
		if ctLen < chacha20poly1305.Overhead || ctLen > maxRecordPlain+chacha20poly1305.Overhead {
			return 0, fmt.Errorf("beacontower: invalid encrypted record size %d", ctLen)
		}
		if _, err := io.ReadFull(c.Conn, c.rawBuf[:ctLen]); err != nil {
			return 0, err
		}
		if c.rseq == 0 {
			return 0, errors.New("beacontower: read sequence exhausted")
		}
		var nonce [12]byte
		var aad [9]byte
		makeRecordNonceAndAAD(c.rdir, c.rseq, &nonce, &aad)
		plain, err := c.raead.Open(c.rbuf[:0], nonce[:], c.rawBuf[:ctLen], aad[:])
		if err != nil {
			return 0, errors.New("beacontower: ciphertext authentication failed")
		}
		c.rseq++
		c.rbuf = plain
	}
	n := copy(p, c.rbuf)
	if n == len(c.rbuf) {
		c.rbuf = c.rbuf[:0]
	} else {
		copy(c.rbuf, c.rbuf[n:])
		c.rbuf = c.rbuf[:len(c.rbuf)-n]
	}
	return n, nil
}

func (c *aeadConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > maxRecordPlain {
			chunk = chunk[:maxRecordPlain]
		}
		if c.wseq == 0 {
			return written, errors.New("beacontower: write sequence exhausted")
		}
		var nonce [12]byte
		var aad [9]byte
		makeRecordNonceAndAAD(c.wdir, c.wseq, &nonce, &aad)
		ct := c.waead.Seal(c.wbuf[4:4], nonce[:], chunk, aad[:])
		binary.BigEndian.PutUint32(c.wbuf[:4], uint32(len(ct)))
		if _, err := c.Conn.Write(c.wbuf[:4+len(ct)]); err != nil {
			return written, err
		}
		c.wseq++
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}
