package checksum

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifier_ParseChecksums(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "valid checksums",
			input: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  empty.tar.gz
a948904f2f0f479b8f8564cbf12dac6b0c7e0e5f5e8e8e8e8e8e8e8e8e8e8e8e  umono_v1.0.0_Linux_x86_64.tar.gz`,
			expected: map[string]string{
				"empty.tar.gz":                     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"umono_v1.0.0_Linux_x86_64.tar.gz": "a948904f2f0f479b8f8564cbf12dac6b0c7e0e5f5e8e8e8e8e8e8e8e8e8e8e8e",
			},
			wantErr: false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
			wantErr:  true,
		},
		{
			name: "with comments and empty lines",
			input: `# This is a comment
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  file.tar.gz

# Another comment`,
			expected: map[string]string{
				"file.tar.gz": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			wantErr: false,
		},
		{
			name:     "invalid hash length",
			input:    "abc123  file.tar.gz",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVerifier()
			err := v.parseChecksums(bytes.NewBufferString(tt.input))

			if (err != nil) != tt.wantErr {
				t.Errorf("parseChecksums() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for filename, expectedHash := range tt.expected {
					actualHash, ok := v.checksums[filename]
					if !ok {
						t.Errorf("expected checksum for %s not found", filename)
						continue
					}
					if actualHash != expectedHash {
						t.Errorf("checksum for %s = %s, want %s", filename, actualHash, expectedHash)
					}
				}
			}
		})
	}
}

func TestVerifier_VerifyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")

	err := os.WriteFile(testFile, testContent, 0o644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	v := NewVerifier()
	v.checksums["test.txt"] = expectedHash

	err = v.VerifyFile(testFile, "test.txt")
	if err != nil {
		t.Errorf("VerifyFile() unexpected error: %v", err)
	}

	v.checksums["test.txt"] = "0000000000000000000000000000000000000000000000000000000000000000"
	err = v.VerifyFile(testFile, "test.txt")
	if err == nil {
		t.Error("VerifyFile() expected error for mismatched checksum")
	}

	_, ok := err.(*ChecksumMismatchError)
	if !ok {
		t.Errorf("VerifyFile() expected ChecksumMismatchError, got %T", err)
	}
}

func TestVerifier_VerifyReader(t *testing.T) {
	testContent := []byte("test data for verification")

	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	v := NewVerifier()
	v.checksums["data.bin"] = expectedHash

	data, err := v.VerifyReader(bytes.NewReader(testContent), "data.bin")
	if err != nil {
		t.Errorf("VerifyReader() unexpected error: %v", err)
	}
	if !bytes.Equal(data, testContent) {
		t.Error("VerifyReader() returned data doesn't match input")
	}

	v.checksums["data.bin"] = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = v.VerifyReader(bytes.NewReader(testContent), "data.bin")
	if err == nil {
		t.Error("VerifyReader() expected error for mismatched checksum")
	}
}

func TestVerifier_HasChecksum(t *testing.T) {
	v := NewVerifier()
	v.checksums["exists.tar.gz"] = "abc123"

	if !v.HasChecksum("exists.tar.gz") {
		t.Error("HasChecksum() returned false for existing checksum")
	}

	if v.HasChecksum("notexists.tar.gz") {
		t.Error("HasChecksum() returned true for non-existing checksum")
	}
}

func TestCalculateFileSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hash_test.txt")
	testContent := []byte("calculate my hash")

	err := os.WriteFile(testFile, testContent, 0o644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	actualHash, err := CalculateFileSHA256(testFile)
	if err != nil {
		t.Errorf("CalculateFileSHA256() unexpected error: %v", err)
	}

	if actualHash != expectedHash {
		t.Errorf("CalculateFileSHA256() = %s, want %s", actualHash, expectedHash)
	}
}

func TestChecksumMismatchError(t *testing.T) {
	err := &ChecksumMismatchError{
		Filename: "test.tar.gz",
		Expected: "expected_hash",
		Actual:   "actual_hash",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("ChecksumMismatchError.Error() returned empty string")
	}

	if !contains(msg, "test.tar.gz") || !contains(msg, "expected_hash") || !contains(msg, "actual_hash") {
		t.Errorf("ChecksumMismatchError.Error() missing expected content: %s", msg)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
