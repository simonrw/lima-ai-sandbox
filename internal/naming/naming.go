package naming

import (
	"crypto/rand"
	"fmt"
)

const prefix = "sandbox-"

// Generate returns a name like "sandbox-a1b2c3d4".
func Generate() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random name: %w", err)
	}
	return fmt.Sprintf("%s%x", prefix, b), nil
}

// IsSandbox reports whether name has the sandbox prefix.
func IsSandbox(name string) bool {
	return len(name) > len(prefix) && name[:len(prefix)] == prefix
}
