package storage

import (
	"crypto/sha1"
	"encoding/hex"
)

func pathHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
