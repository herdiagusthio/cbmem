package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func ContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func FileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return ContentHash(data), nil
}

func StringHash(s string) string { return ContentHash([]byte(s)) }
