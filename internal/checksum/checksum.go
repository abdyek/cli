package checksum

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Verifier struct {
	checksums map[string]string
}

func NewVerifier() *Verifier {
	return &Verifier{
		checksums: make(map[string]string),
	}
}

func (v *Verifier) LoadFromURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums: HTTP %d", resp.StatusCode)
	}

	return v.parseChecksums(resp.Body)
}

func (v *Verifier) LoadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open checksums file: %w", err)
	}
	defer file.Close()

	return v.parseChecksums(file)
}

func (v *Verifier) parseChecksums(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		filename := parts[1]

		if len(hash) != 64 {
			continue
		}

		v.checksums[filename] = strings.ToLower(hash)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to parse checksums: %w", err)
	}

	if len(v.checksums) == 0 {
		return fmt.Errorf("no valid checksums found")
	}

	return nil
}

func (v *Verifier) VerifyFile(filepath, filename string) error {
	expectedHash, ok := v.checksums[filename]
	if !ok {
		return fmt.Errorf("no checksum found for %s", filename)
	}

	actualHash, err := calculateFileSHA256(filepath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualHash != expectedHash {
		return &ChecksumMismatchError{
			Filename: filename,
			Expected: expectedHash,
			Actual:   actualHash,
		}
	}

	return nil
}

func (v *Verifier) VerifyReader(r io.Reader, filename string) ([]byte, error) {
	expectedHash, ok := v.checksums[filename]
	if !ok {
		return nil, fmt.Errorf("no checksum found for %s", filename)
	}

	hasher := sha256.New()
	data, err := io.ReadAll(io.TeeReader(r, hasher))
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return nil, &ChecksumMismatchError{
			Filename: filename,
			Expected: expectedHash,
			Actual:   actualHash,
		}
	}

	return data, nil
}

func (v *Verifier) HasChecksum(filename string) bool {
	_, ok := v.checksums[filename]
	return ok
}

func (v *Verifier) GetChecksum(filename string) (string, bool) {
	hash, ok := v.checksums[filename]
	return hash, ok
}

type ChecksumMismatchError struct {
	Filename string
	Expected string
	Actual   string
}

func (e *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("checksum mismatch for %s: expected %s, got %s", e.Filename, e.Expected, e.Actual)
}

func calculateFileSHA256(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func CalculateSHA256(r io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func CalculateFileSHA256(filepath string) (string, error) {
	return calculateFileSHA256(filepath)
}
