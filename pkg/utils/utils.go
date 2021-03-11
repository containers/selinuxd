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
	ErrInvalidPath = errors.New("invalid path")
	ErrNoExtension = errors.New("file without extension")
)

func NewErrInvalidPath(path string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPath, path)
}

func GetFileWithoutExtension(filename string) string {
	extension := filepath.Ext(filename)
	return filename[0 : len(filename)-len(extension)]
}

func GetCleanBase(path string) (string, error) {
	// NOTE: don't trust the path even if it came from fsnotify
	cleanPath := filepath.Clean(path)
	if cleanPath == "" {
		return "", NewErrInvalidPath(path)
	}

	// NOTE: Still not trusting that path. Let's just use the base
	// and use our configured base path
	return filepath.Base(cleanPath), nil
}

func GetSafePath(modulePath, path string) (string, error) {
	policyFileBase, err := GetCleanBase(path)
	if err != nil {
		return "", err
	}
	policyPath := filepath.Join(modulePath, policyFileBase)
	return policyPath, nil
}

func PolicyNameFromPath(path string) (string, error) {
	if filepath.Ext(path) == "" {
		return "", fmt.Errorf("ignoring: %w", ErrNoExtension)
	}
	baseFile, err := GetCleanBase(path)
	if err != nil {
		return "", fmt.Errorf("failed getting clean base name for policy: %w", err)
	}
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
