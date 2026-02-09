package crypto

import (
	"testing"
)

func TestCipherEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	cipher, err := NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"json", `{"domain": "google.com", "type": "A"}`},
		{"unicode", "سلام دنیا 🌍"},
		{"long", "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := cipher.Encrypt([]byte(tc.plaintext))
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			decrypted, err := cipher.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if string(decrypted) != tc.plaintext {
				t.Errorf("Mismatch: got %q, want %q", string(decrypted), tc.plaintext)
			}
		})
	}
}

func TestCipherInvalidKey(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{"too_short", "abcd1234"},
		{"invalid_hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCipher(tc.key)
			if err == nil {
				t.Error("Expected error for invalid key")
			}
		})
	}
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if key1 == key2 {
		t.Error("Generated keys should be unique")
	}

	if len(key1) != 64 {
		t.Errorf("Key should be 64 hex chars, got %d", len(key1))
	}
}
