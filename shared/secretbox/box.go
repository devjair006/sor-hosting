package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

func Seal(key, data []byte) (string, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	a, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return base64.RawStdEncoding.EncodeToString(a.Seal(nonce, nonce, data, []byte("sor-hosting:v1"))), nil
}
func Open(key []byte, value string) ([]byte, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(b)
	if e != nil {
		return nil, e
	}
	v, e := base64.RawStdEncoding.DecodeString(value)
	if e != nil || len(v) < a.NonceSize() {
		return nil, errors.New("invalid encrypted value")
	}
	return a.Open(nil, v[:a.NonceSize()], v[a.NonceSize():], []byte("sor-hosting:v1"))
}
