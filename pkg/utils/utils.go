package utils

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrInvalidExtension = errors.New("file with invalid extension, valid extensions: .cil .pp")
)

func NewErrInvalidPath(path string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPath, path)
}

func GetFileWithoutExtension(filename string) string {
	extension := filepath.Ext(filename)
	return filename[0 : len(filename)-len(extension)]
}

func PolicyNameFromPath(path string) (string, error) {
	switch filepath.Ext(path) {
	case ".cil", ".pp":
	default:
		return "", fmt.Errorf("ignoring: %w", ErrInvalidExtension)
	}
	baseFile := filepath.Base(path)
	policy := GetFileWithoutExtension(baseFile)
	return policy, nil
}

// Checksum returns a checksum for a file on a given path
func Checksum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate checksum: %w", err)
	}
	defer f.Close()

	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("unable to calculate checksum: %w", err)
	}

	return h.Sum(nil), nil
}
