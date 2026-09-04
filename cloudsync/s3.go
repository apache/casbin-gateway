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
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// KindS3 is object storage with the S3 API, which is most of it: S3 itself,
// Cloudflare R2, Backblaze B2, MinIO, Aliyun OSS and Tencent COS.
const KindS3 = "s3"

const defaultS3Region = "us-east-1"

func init() {
	Register(&Kind{
		Name:        KindS3,
		DisplayName: "S3",
		Description: "S3 description",
		Fields: []Field{
			{Name: "bucket", Label: "Bucket", Type: FieldText, Required: true},
			{Name: "region", Label: "Region", Type: FieldText, Placeholder: defaultS3Region},
			{Name: "endpoint", Label: "Endpoint", Type: FieldText, Placeholder: "https://s3.us-east-1.amazonaws.com", Hint: "Endpoint hint"},
			{Name: "prefix", Label: "Key prefix", Type: FieldText, Placeholder: "casbin-gateway/", Hint: "Key prefix hint"},
			{Name: "accessKeyId", Label: "Access key ID", Type: FieldText, Required: true},
			{Name: "secretAccessKey", Label: "Secret access key", Type: FieldSecret, Required: true},
			{Name: "pathStyle", Label: "Path-style addressing", Type: FieldSwitch, Hint: "Path-style addressing hint"},
		},
		New: newS3Target,
	})
}

type s3Target struct {
	bucket    string
	region    string
	prefix    string
	accessKey string
	secretKey string
	// endpoint carries the scheme and the host only, the bucket never.
	endpoint  *url.URL
	pathStyle bool
	client    *http.Client
}

func newS3Target(config Config) (Target, error) {
	region := config.OptionDefault("region", defaultS3Region)

	raw := config.OptionDefault("endpoint", fmt.Sprintf("https://s3.%s.amazonaws.com", region))
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	endpoint, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if endpoint.Host == "" {
		return nil, fmt.Errorf("this is not an S3 endpoint: %s", raw)
	}

	// Everything that is not AWS itself is addressed by path: MinIO and most
	// NAS gateways never got the wildcard certificate a bucket subdomain needs.
	pathStyle, set := config.OptionBool("pathStyle")
	if !set {
		pathStyle = !strings.HasSuffix(endpoint.Hostname(), "amazonaws.com")
	}

	prefix := strings.Trim(strings.ReplaceAll(config.Option("prefix"), "\\", "/"), "/")
	if prefix != "" {
		prefix += "/"
	}

	return &s3Target{
		bucket:    config.Option("bucket"),
		region:    region,
		prefix:    prefix,
		accessKey: config.Option("accessKeyId"),
		secretKey: config.Option("secretAccessKey"),
		endpoint:  &url.URL{Scheme: endpoint.Scheme, Host: endpoint.Host},
		pathStyle: pathStyle,
		client:    config.client(),
	}, nil
}

func (target *s3Target) Describe() string {
	return fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(target.endpoint.String(), "/"), target.bucket, target.prefix)
}

func (target *s3Target) List(ctx context.Context) ([]File, error) {
	files := []File{}
	token := ""

	for {
		query := map[string]string{"list-type": "2", "max-keys": "1000"}
		if target.prefix != "" {
			query["prefix"] = target.prefix
		}
		if token != "" {
			query["continuation-token"] = token
		}

		body, status, err := target.do(ctx, http.MethodGet, "", query, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, statusError(status, body)
		}

		result := &s3ListResult{}
		if err := xml.Unmarshal(body, result); err != nil {
			return nil, err
		}

		for _, item := range result.Contents {
			name := strings.TrimPrefix(item.Key, target.prefix)
			// A key with a slash left in it lives in a folder of its own, which
			// is somebody else's file rather than one of ours.
			if name == "" || strings.Contains(name, "/") {
				continue
			}

			file := File{Name: name, Size: item.Size}
			if at, err := time.Parse(time.RFC3339, item.LastModified); err == nil {
				file.ModifiedTime = at
			}
			files = append(files, file)
		}

		if !result.IsTruncated || result.NextContinuationToken == "" {
			return files, nil
		}
		token = result.NextContinuationToken
	}
}

func (target *s3Target) Read(ctx context.Context, name string) ([]byte, error) {
	body, status, err := target.do(ctx, http.MethodGet, name, nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, statusError(status, body)
	}
	return body, nil
}

func (target *s3Target) Write(ctx context.Context, name string, data []byte) error {
	body, status, err := target.do(ctx, http.MethodPut, name, nil, data)
	if err != nil {
		return err
	}
	if status/100 != 2 {
		return statusError(status, body)
	}
	return nil
}

func (target *s3Target) Remove(ctx context.Context, name string) error {
	body, status, err := target.do(ctx, http.MethodDelete, name, nil, nil)
	if err != nil {
		return err
	}
	if status/100 != 2 && status != http.StatusNotFound {
		return statusError(status, body)
	}
	return nil
}

func (target *s3Target) do(ctx context.Context, method string, name string, query map[string]string, payload []byte) ([]byte, int, error) {
	key, err := target.key(name)
	if err != nil {
		return nil, 0, err
	}

	requestUrl := *target.endpoint
	if target.pathStyle {
		requestUrl.Path = "/" + target.bucket + "/" + key
	} else {
		requestUrl.Host = target.bucket + "." + target.endpoint.Host
		requestUrl.Path = "/" + key
	}
	requestUrl.RawQuery = canonicalQuery(query)

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestUrl.String(), body)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.ContentLength = int64(len(payload))
		req.Header.Set("Content-Type", "application/json")
	}

	target.sign(req, requestUrl.Path, requestUrl.RawQuery, payload)

	res, err := target.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	answer, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return answer, res.StatusCode, nil
}

// key is where one file lives in the bucket. A listing arrives over the
// network, so a name that is not a name never becomes a key. The empty name is
// the bucket itself, which is what a listing asks for.
func (target *s3Target) key(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("this is not a file name: %s", name)
	}
	return target.prefix + name, nil
}

type s3ListResult struct {
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
	Contents              []struct {
		Key          string `xml:"Key"`
		Size         int64  `xml:"Size"`
		LastModified string `xml:"LastModified"`
	} `xml:"Contents"`
}

func canonicalQuery(query map[string]string) string {
	if len(query) == 0 {
		return ""
	}

	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, uriEncode(key, true)+"="+uriEncode(query[key], true))
	}
	return strings.Join(pairs, "&")
}

// uriEncode is the escaping S3 signs with: everything but the unreserved set,
// and the slash only when it is part of a value rather than of a path.
func uriEncode(value string, encodeSlash bool) string {
	var encoded strings.Builder
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch {
		case (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'),
			char == '-', char == '_', char == '.', char == '~':
			encoded.WriteByte(char)
		case char == '/' && !encodeSlash:
			encoded.WriteByte(char)
		default:
			fmt.Fprintf(&encoded, "%%%02X", char)
		}
	}
	return encoded.String()
}
