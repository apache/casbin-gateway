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

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// encPrefix marks a value produced by EncryptWithKey. It lets DecryptWithKey
// tell ciphertext apart from a legacy plaintext value written before encryption
// was enabled, so existing rows keep working.
const encPrefix = "enc:v1:"

// IsEncrypted reports whether value carries the ciphertext marker.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encPrefix)
}

// deriveKey turns an arbitrary-length secret into a 32-byte AES-256 key.
func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptWithKey returns an AES-256-GCM ciphertext (base64, prefixed) for value.
// Encryption is opt-in: an empty secret or empty value returns value unchanged,
// and an already-encrypted value is returned as-is so re-saving is idempotent.
func EncryptWithKey(secret, value string) (string, error) {
	if secret == "" || value == "" || IsEncrypted(value) {
		return value, nil
	}

	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Prepend the nonce to the ciphertext so DecryptWithKey can recover it.
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptWithKey reverses EncryptWithKey. A value without the marker is treated
// as legacy plaintext and returned unchanged, which keeps pre-encryption rows
// usable after the key is switched on.
func DecryptWithKey(secret, value string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	if secret == "" {
		return "", errors.New("value is encrypted but no encryption key is configured")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encPrefix))
	if err != nil {
		return "", err
	}

	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid ciphertext: too short")
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func newGCM(secret string) (cipher.AEAD, error) {
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
