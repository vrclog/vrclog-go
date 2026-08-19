package logfile

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
)

func SourceID(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	normalized := filepath.Clean(abs)

	if runtime.GOOS == "windows" {
		normalized = normalizeWindows(normalized)
	}

	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:]), nil
}

func normalizeWindows(path string) string {
	return strings.ToLower(path)
}
