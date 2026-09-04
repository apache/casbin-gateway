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

package cloudsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Signature Version 4, which every S3-compatible service speaks and which is
// some eighty lines of hashing. It is written out here rather than pulled in:
// the AWS SDK is a large dependency for four requests.
const (
	sigAlgorithm = "AWS4-HMAC-SHA256"
	sigService   = "s3"
	// The headers signed on every request. Anything else a proxy adds on the
	// way out stays out of the signature.
	sigHeaders = "host;x-amz-content-sha256;x-amz-date"
)

// sign fills in the Authorization header. The path and query it is given are
// the ones the request will really carry, because the signature covers them
// exactly as they go over the wire.
func (target *s3Target) sign(req *http.Request, path string, query string, payload []byte) {
	now := time.Now().UTC()
	stamp := now.Format("20060102T150405Z")
	day := now.Format("20060102")

	payloadHash := hashHex(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", stamp)

	canonicalRequest := strings.Join([]string{
		req.Method,
		uriEncode(path, false),
		query,
		"host:" + req.URL.Host + "\n" +
			"x-amz-content-sha256:" + payloadHash + "\n" +
			"x-amz-date:" + stamp + "\n",
		sigHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{day, target.region, sigService, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigAlgorithm,
		stamp,
		scope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	key := []byte("AWS4" + target.secretKey)
	for _, part := range []string{day, target.region, sigService, "aws4_request"} {
		key = sign(key, part)
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigAlgorithm, target.accessKey, scope, sigHeaders, hex.EncodeToString(sign(key, stringToSign))))
}

func sign(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func hashHex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
