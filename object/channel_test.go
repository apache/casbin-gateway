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

package object

import "testing"

func TestValidateBaseUrl(t *testing.T) {
	tests := []struct {
		name    string
		baseUrl string
		wantErr bool
	}{
		// Valid public upstreams.
		{"public https", "https://api.openai.com/v1", false},
		{"public http", "http://api.deepseek.com/v1", false},
		{"public hostname with port", "https://oneapi.example.com:8080", false},
		{"public ip literal", "https://8.8.8.8/v1", false},

		// Scheme and host validation.
		{"empty", "", true},
		{"missing scheme", "api.openai.com/v1", true},
		{"unsupported scheme", "ftp://api.openai.com", true},
		{"file scheme", "file:///etc/passwd", true},

		// SSRF: loopback / localhost.
		{"localhost", "http://localhost/v1", true},
		{"localhost uppercase", "http://LOCALHOST/v1", true},
		{"localhost subdomain", "http://foo.localhost/v1", true},
		{"loopback ipv4", "http://127.0.0.1/v1", true},
		{"loopback ipv4 alt", "http://127.9.9.9:8080", true},
		{"loopback ipv6", "http://[::1]/v1", true},

		// SSRF: cloud metadata / link-local.
		{"metadata endpoint", "http://169.254.169.254/latest/meta-data", true},
		{"link-local ipv6", "http://[fe80::1]/v1", true},

		// SSRF: private ranges.
		{"rfc1918 10", "http://10.0.0.1/v1", true},
		{"rfc1918 172", "http://172.16.5.4/v1", true},
		{"rfc1918 192", "https://192.168.1.1/v1", true},
		{"ipv6 unique local", "http://[fc00::1]/v1", true},

		// SSRF: unspecified / broadcast.
		{"unspecified", "http://0.0.0.0/v1", true},
		{"broadcast", "http://255.255.255.255/v1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBaseUrl(tt.baseUrl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBaseUrl(%q) error = %v, wantErr = %v", tt.baseUrl, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBaseUrl_Allowlist(t *testing.T) {
	tests := []struct {
		name      string
		allowlist string
		baseUrl   string
		wantErr   bool
	}{
		{"exact ip allowed", "10.0.0.5", "http://10.0.0.5/v1", false},
		{"exact ip does not allow others", "10.0.0.5", "http://10.0.0.6/v1", true},
		{"cidr allows range", "192.168.0.0/16", "http://192.168.1.1/v1", false},
		{"cidr does not allow outside", "192.168.0.0/16", "http://10.0.0.1/v1", true},
		{"localhost by name", "localhost", "http://localhost/v1", false},
		{"comma separated list", "127.0.0.1, 10.0.0.0/8", "http://10.1.2.3/v1", false},
		{"space separated list", "127.0.0.1 10.0.0.0/8", "http://127.0.0.1/v1", false},
		{"allowlist does not weaken scheme check", "10.0.0.5", "ftp://10.0.0.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// conf.GetConfigString reads env vars first, so this overrides the setting.
			t.Setenv("allowedPrivateUpstreamHosts", tt.allowlist)
			err := validateBaseUrl(tt.baseUrl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBaseUrl(%q) with allowlist %q error = %v, wantErr = %v", tt.baseUrl, tt.allowlist, err, tt.wantErr)
			}
		})
	}
}
