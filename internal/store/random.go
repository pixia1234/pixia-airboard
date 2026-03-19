package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func randomToken(size int) string {
	if size < 1 {
		size = 16
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("random token: %v", err))
	}
	return hex.EncodeToString(buf)
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("random uuid: %v", err))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
