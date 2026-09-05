// Copyright 2021 The casbin Authors. All Rights Reserved.
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

// Package proxy routes every outbound request through the proxy configured as
// httpProxy in conf/app.conf, falling back to the HTTP_PROXY / HTTPS_PROXY
// environment variables when that setting is empty.
package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/conf"
)

// ProxyHttpClient sends its requests through the configured proxy.
var ProxyHttpClient *http.Client

var (
	proxyLock sync.Mutex
	proxyRead bool
	proxyRaw  string
	proxyUrl  *url.URL

	transportOnce sync.Once
	transport     http.RoundTripper
)

// UserAgent identifies this gateway to the upstreams it calls. Go leaves the
// header at "Go-http-client/1.1", which the bot filters in front of several
// relays refuse outright with a 403 and an HTML error page.
const UserAgent = "casbin-gateway"

func InitHttpClient() {
	// Reading the setting here rather than on first use keeps the "proxy
	// enabled" line in the startup log.
	currentProxyUrl()
	ProxyHttpClient = &http.Client{Transport: Transport()}
}

// Proxy picks the proxy to reach req through. It is meant to be assigned to
// http.Transport.Proxy, which calls it per request, so the httpProxy setting is
// read on use instead of at package initialization: in -tags embed builds the
// embedded conf/app.conf is only loaded once main's init() runs, after every
// imported package has already initialized its own variables. Reading it per
// request also means a proxy set on the Settings page applies straight away.
func Proxy(req *http.Request) (*url.URL, error) {
	if parsedUrl := currentProxyUrl(); parsedUrl != nil {
		return parsedUrl, nil
	}

	return http.ProxyFromEnvironment(req)
}

// Transport returns the shared transport for outbound requests. Sharing one
// instance keeps the connection pool shared as well.
func Transport() http.RoundTripper {
	transportOnce.Do(func() {
		base := http.DefaultTransport.(*http.Transport).Clone()
		base.Proxy = Proxy
		transport = WithUserAgent(base)
	})
	return transport
}

// WithUserAgent names this gateway on every request that does not already
// carry a User-Agent of its own, so a request forwarding the caller's keeps it.
func WithUserAgent(base http.RoundTripper) http.RoundTripper {
	return userAgentTransport{base: base}
}

type userAgentTransport struct {
	base http.RoundTripper
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := req.Header["User-Agent"]; !ok {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", UserAgent)
	}
	return t.base.RoundTrip(req)
}

// currentProxyUrl parses the httpProxy setting, re-parsing it only when the
// setting itself has changed.
func currentProxyUrl() *url.URL {
	httpProxy := strings.TrimSpace(conf.GetConfigStringUnquoted("httpProxy"))

	proxyLock.Lock()
	defer proxyLock.Unlock()

	if proxyRead && httpProxy == proxyRaw {
		return proxyUrl
	}

	proxyRead = true
	proxyRaw = httpProxy
	proxyUrl = parseProxyUrl(httpProxy)
	return proxyUrl
}

// parseProxyUrl reads the httpProxy setting, reporting an address it cannot use
// rather than failing the requests that would have gone through it.
func parseProxyUrl(httpProxy string) *url.URL {
	if httpProxy == "" {
		return nil
	}

	parsedUrl, err := parseProxyAddress(httpProxy)
	if err != nil {
		fmt.Printf("httpProxy is not a valid proxy address, outbound traffic is left unproxied: %s\n", httpProxy)
		return nil
	}

	fmt.Printf("Proxy enabled for outbound traffic: %s\n", parsedUrl.Redacted())
	return parsedUrl
}

// parseProxyAddress reads a proxy address. A bare "host:port" means SOCKS5,
// which is what the setting has always been taken to mean; "socks5://",
// "socks5h://", "http://" and "https://" addresses are honoured as written, and
// may carry credentials.
func parseProxyAddress(address string) (*url.URL, error) {
	if !strings.Contains(address, "://") {
		address = "socks5://" + address
	}

	parsedUrl, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid proxy address: %w", address, err)
	}
	if parsedUrl.Host == "" {
		return nil, fmt.Errorf("%s is not a valid proxy address", address)
	}

	switch parsedUrl.Scheme {
	case "socks5", "socks5h", "http", "https":
	default:
		return nil, fmt.Errorf("%s is not a proxy scheme Gateway can use", parsedUrl.Scheme)
	}
	return parsedUrl, nil
}

// Env is the proxy configuration a child process reads, for the package
// managers and vendor install scripts Gateway runs: none of them share this
// process's transport, so a proxy set here has to be handed to them. Empty when
// no proxy is configured, which leaves a child whatever the environment already
// gives it.
//
// Both spellings are set because tools disagree on which they read. winget is
// the exception on Windows: it goes through the system proxy and ignores these.
func Env() []string {
	parsedUrl := currentProxyUrl()
	if parsedUrl == nil {
		return nil
	}

	address := parsedUrl.String()
	return []string{
		"HTTP_PROXY=" + address, "http_proxy=" + address,
		"HTTPS_PROXY=" + address, "https_proxy=" + address,
		"ALL_PROXY=" + address, "all_proxy=" + address,
	}
}
