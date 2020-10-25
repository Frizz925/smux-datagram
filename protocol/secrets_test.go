package protocol

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretKeys(t *testing.T) {
	require := require.New(t)
	randGen := rand.New(rand.NewSource(0))

	privKey1, err := ReadPrivateKey(randGen)
	require.Nil(err)
	privKey2, err := ReadPrivateKey(randGen)
	require.Nil(err)
	require.NotEqual(privKey1, privKey2)

	pubKey1, err := privKey1.PublicKey()
	require.Nil(err)
	pubKey2, err := privKey2.PublicKey()
	require.Nil(err)

	ss1, err := privKey1.SharedSecret(pubKey2)
	require.Nil(err)
	ss2, err := privKey2.SharedSecret(pubKey1)
	require.Nil(err)
	require.Equal(ss1, ss2)
}
