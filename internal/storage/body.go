package storage

import (
	"fmt"
	"os"
	"strings"
)

// ReadBody loads a memory body from a file. Every failure names the path, and
// a file that holds no text is refused, so an interrupted edit cannot replace
// a stored memory with nothing.
func ReadBody(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read body from %s: %w", path, err)
	}
	body := string(data)
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("body in %s is empty", path)
	}
	return body, nil
}
