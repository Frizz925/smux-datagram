package protocol

import (
	"bytes"
	"crypto/cipher"
	"fmt"
	"io"
)

const frameSizeMult = 16

type Crypto []byte

func ReadCrypto(r io.Reader) (Crypto, error) {
	raw, err := ReadRaw(r)
	if err != nil {
		return nil, err
	}
	return Crypto(raw), nil
}

func (Crypto) Type() FrameType {
	return FrameCrypto
}

func (c Crypto) Serialize(w io.Writer) error {
	return WriteRaw(w, c[:])
}

func (c Crypto) String() string {
	return fmt.Sprintf("Crypto(Length: %d)", len(c))
}

type CryptoFacade struct {
	cipher.AEAD
}

func NewCryptoFacade(aead cipher.AEAD) *CryptoFacade {
	return &CryptoFacade{aead}
}

func (cf *CryptoFacade) Decrypt(crypto Crypto, nonce Nonce) (Frame, error) {
	plaintext, err := cf.Open(nil, nonce[:], crypto[:], nil)
	if err != nil {
		return nil, err
	}
	raw, err := ReadRaw(bytes.NewReader(plaintext))
	if err != nil {
		return nil, err
	}
	return ReadFrame(bytes.NewReader(raw))
}

func (cf *CryptoFacade) Encrypt(frame Frame, nonce Nonce) (Crypto, error) {
	frameBuf := &bytes.Buffer{}
	if err := WriteFrame(frameBuf, frame); err != nil {
		return nil, err
	}
	rawBuf := &bytes.Buffer{}
	if err := WriteRaw(rawBuf, frameBuf.Bytes()); err != nil {
		return nil, err
	}

	// Add padding to the next closest multiplication of 16
	length := rawBuf.Len()
	paddingSize := frameSizeMult - (length % frameSizeMult)
	padding := make([]byte, paddingSize)
	rawBuf.Write(padding)

	ciphertext := cf.Seal(nil, nonce[:], rawBuf.Bytes(), nil)
	return Crypto(ciphertext), nil
}
