package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/go-rod/rod/lib/proto"
)

// Encryption key for cookies (in production, should be loaded from secure env)
// For demo, we use a fixed key if not provided or generate one
var encryptionKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

func init() {
	if key := os.Getenv("COOKIE_ENCRYPTION_KEY"); key != "" {
		if len(key) == 32 {
			encryptionKey = []byte(key)
		}
	}
}

func EncryptCookies(cookies []*proto.NetworkCookie) (string, error) {
	data, err := json.Marshal(cookies)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return hex.EncodeToString(ciphertext), nil
}

func DecryptCookies(encrypted string) ([]*proto.NetworkCookie, error) {
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(plaintext, &cookies); err != nil {
		return nil, err
	}

	return cookies, nil
}
