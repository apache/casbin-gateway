// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-secret"
	plaintext := "sk-1234567890abcdef"

	ciphertext, err := EncryptWithKey(secret, plaintext)
	if err != nil {
		t.Fatalf("EncryptWithKey failed: %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext equals plaintext; value was not encrypted")
	}
	if !IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext is missing the encryption marker: %q", ciphertext)
	}

	got, err := DecryptWithKey(secret, ciphertext)
	if err != nil {
		t.Fatalf("DecryptWithKey failed: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestEncryptIsNonDeterministic(t *testing.T) {
	secret, plaintext := "s", "same-value"
	a, _ := EncryptWithKey(secret, plaintext)
	b, _ := EncryptWithKey(secret, plaintext)
	if a == b {
		t.Fatal("two encryptions produced identical ciphertext; nonce is not random")
	}
}

func TestEncryptOptOut(t *testing.T) {
	// No secret means encryption is disabled: the value is returned unchanged.
	if got, _ := EncryptWithKey("", "plain"); got != "plain" {
		t.Fatalf("empty secret should pass through, got %q", got)
	}
	// An empty value is never wrapped.
	if got, _ := EncryptWithKey("secret", ""); got != "" {
		t.Fatalf("empty value should pass through, got %q", got)
	}
}

func TestEncryptIdempotent(t *testing.T) {
	secret := "s"
	once, _ := EncryptWithKey(secret, "value")
	twice, err := EncryptWithKey(secret, once)
	if err != nil {
		t.Fatalf("re-encrypt failed: %v", err)
	}
	if twice != once {
		t.Fatal("re-encrypting an already-encrypted value changed it")
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	// A value written before encryption was enabled has no marker and must be
	// returned untouched, even without a key.
	if got, err := DecryptWithKey("", "legacy-plain-key"); err != nil || got != "legacy-plain-key" {
		t.Fatalf("legacy plaintext passthrough failed: got %q, err %v", got, err)
	}
	if got, err := DecryptWithKey("secret", "legacy-plain-key"); err != nil || got != "legacy-plain-key" {
		t.Fatalf("legacy plaintext passthrough with key failed: got %q, err %v", got, err)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	ciphertext, _ := EncryptWithKey("right-key", "secret-value")
	if _, err := DecryptWithKey("wrong-key", ciphertext); err == nil {
		t.Fatal("decrypting with the wrong key should fail")
	}
}

func TestDecryptEncryptedWithoutKeyFails(t *testing.T) {
	ciphertext, _ := EncryptWithKey("key", "secret-value")
	if _, err := DecryptWithKey("", ciphertext); err == nil {
		t.Fatal("decrypting ciphertext without a configured key should fail")
	}
}
