package cli

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"time"
)

func newLeaseID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "cbx_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000"), ".", "")
	}
	return "cbx_" + hex.EncodeToString(b[:])
}

func publicKeyFor(privatePath string) (string, error) {
	pub := privatePath + ".pub"
	data, err := os.ReadFile(pub)
	if err != nil {
		return "", exit(2, "read ssh public key %s: %v", pub, err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", exit(2, "ssh public key %s is empty", pub)
	}
	return key, nil
}
