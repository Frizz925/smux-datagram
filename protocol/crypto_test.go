package protocol

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"
)

func TestCrypto(t *testing.T) {
	require := require.New(t)
	randGen := rand.New(rand.NewSource(0))
	content := []byte("Hello, world!")
	expected := NewRaw(FrameRaw, content)

	key, err := ReadPrivateKey(randGen)
	require.Nil(err)
	aead, err := chacha20poly1305.New(key[:])
	require.Nil(err)
	facade := NewCryptoFacade(aead)
	nonce := Uint32ToNonce(1)

	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	cipherLen := buf.Len() + aead.NonceSize() + aead.Overhead()
	buf.Reset()

	cout, err := facade.Encrypt(expected, nonce)
	require.Nil(err)
	require.Nil(cout.Serialize(buf))
	require.Greater(buf.Len(), cipherLen, "Crypto frame should have padding")

	cin, err := ReadCrypto(buf)
	require.Nil(err)
	actual, err := facade.Decrypt(cin, nonce)
	require.Nil(err)
	require.Equal(expected, actual)
}
