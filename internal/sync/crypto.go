package sync

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"

	"hyla/internal/plugins/manifest"
)

const (
	// snapshotVersion is what a snapshot holds, which a later format tells
	// from this one.
	snapshotVersion = 1
	// magic opens a sealed snapshot, so that a file says what it is even
	// when its name does not.
	magic    = "HYLASYNC1"
	saltLen  = 16
	nonceLen = 12
	keyLen   = 32
	// argon2id's work, as OWASP suggests: 64 MiB and three passes make a
	// passphrase expensive to guess, and half a second to open.
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
)

// Snapshot is what a server holds, before it is sealed: one machine's
// configuration, and what tells it from another's.
type Snapshot struct {
	Version       int                        `json:"version"`
	UpdatedAt     int64                      `json:"updatedAt"` // milliseconds, as Hyla counts them
	Host          string                     `json:"host"`      // the machine that wrote it
	Hash          string                     `json:"hash"`      // of Content (and Plugins/PluginConfig when present)
	Content       string                     `json:"content"`   // the configuration file itself
	PluginsSynced bool                       `json:"pluginsSynced,omitempty"`
	Plugins       *manifest.Registry         `json:"plugins,omitempty"`
	PluginConfig  map[string]json.RawMessage `json:"pluginConfig,omitempty"`
}

// ErrPassphrase is a snapshot that this passphrase does not open: another
// one sealed it, or its bytes were changed. A snapshot is never read
// partially, so a wrong passphrase changes nothing.
var ErrPassphrase = errors.New("this passphrase does not open the snapshot")

// sealedLen is the least a sealed snapshot can be: what says what it is, the
// salt, the nonce, and a tag.
const sealedLen = len(magic) + saltLen + nonceLen + 16

// seal encrypts a snapshot: AES-256-GCM under a key Argon2id derives from the
// passphrase, with a fresh salt and nonce, so that sealing the same
// configuration twice gives two different files.
func seal(passphrase string, s Snapshot) ([]byte, error) {
	plain, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	head := make([]byte, len(magic)+saltLen)
	copy(head, magic)
	if _, err := rand.Read(head[len(magic):]); err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	gcm, err := aead(passphrase, head[len(magic):])
	if err != nil {
		return nil, err
	}
	// The head is authenticated too: a salt changed on the server fails to
	// open rather than opening something else.
	out := append(head, nonce...)
	return gcm.Seal(out, nonce, plain, head), nil
}

// open decrypts a snapshot, or says it cannot: ErrPassphrase for another
// passphrase's, or one whose bytes were changed.
func open(passphrase string, b []byte) (*Snapshot, error) {
	if len(b) < sealedLen || !bytes.HasPrefix(b, []byte(magic)) {
		return nil, errors.New("sync: the snapshot is not Hyla's")
	}
	head := b[:len(magic)+saltLen]
	nonce := b[len(head) : len(head)+nonceLen]
	gcm, err := aead(passphrase, head[len(magic):])
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, b[len(head)+nonceLen:], head)
	if err != nil {
		return nil, ErrPassphrase
	}
	var s Snapshot
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, fmt.Errorf("sync: the snapshot does not read: %w", err)
	}
	if s.Version > snapshotVersion {
		return nil, fmt.Errorf("sync: the snapshot is version %d, and this Hyla syncs %d", s.Version, snapshotVersion)
	}
	return &s, nil
}

// aead is the cipher a passphrase and a salt make.
func aead(passphrase string, salt []byte) (cipher.AEAD, error) {
	key := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
