package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const (
	KeySize   = chacha20poly1305.KeySize
	NonceSize = chacha20poly1305.NonceSize
)

type (
	PrivateKey [KeySize]byte
	PublicKey  [KeySize]byte
	Nonce      [NonceSize]byte
)

// Reader can be nil to generate new private key from cryptographically secure random generator.
func ReadPrivateKey(r io.Reader) (priv PrivateKey, err error) {
	if r == nil {
		r = rand.Reader
	}
	err = ReadFull(r, priv[:])
	if err != nil {
		return
	}
	priv.clamp()
	return priv, nil
}

func (priv PrivateKey) PublicKey() (pub PublicKey, err error) {
	return CreatePublicKey(priv[:])
}

func (priv PrivateKey) SharedSecret(pub PublicKey) ([]byte, error) {
	return SharedSecret(priv[:], pub[:])
}

func (priv PrivateKey) Serialize(w io.Writer) error {
	return WriteFull(w, priv[:])
}

func (priv PrivateKey) String() string {
	return "--PRIVATE KEY--"
}

func (priv PrivateKey) clamp() {
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64
}

func CreatePublicKey(key []byte) (pub PublicKey, err error) {
	b, err := curve25519.X25519(key, curve25519.Basepoint)
	if err != nil {
		return pub, err
	}
	copy(pub[:], b)
	return pub, nil
}

func ReadPublicKey(r io.Reader) (pub PublicKey, err error) {
	return pub, ReadFull(r, pub[:])
}

func (pub PublicKey) SharedSecret(priv PrivateKey) ([]byte, error) {
	return SharedSecret(priv[:], pub[:])
}

func (pub PublicKey) Serialize(w io.Writer) error {
	return WriteFull(w, pub[:])
}

func (pub PublicKey) String() string {
	return hex.EncodeToString(pub[:])
}

func SharedSecret(k0, k1 []byte) ([]byte, error) {
	return curve25519.X25519(k0, k1)
}

func Uint32ToNonce(v uint32) (n Nonce) {
	binary.LittleEndian.PutUint32(n[8:], v)
	return n
}

func Uint64ToNonce(v uint64) (n Nonce) {
	binary.LittleEndian.PutUint64(n[4:], v)
	return n
}
