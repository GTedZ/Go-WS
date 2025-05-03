package gows

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

func generateSecureRandomInt64() int64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(fmt.Errorf("failed to generate secure random int64: %w", err))
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}
