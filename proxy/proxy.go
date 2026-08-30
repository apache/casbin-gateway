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
	transport     *http.Transport
)

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
func Transport() *http.Transport {
	transportOnce.Do(func() {
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = Proxy
	})
	return transport
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

// parseProxyUrl reads a proxy address. A bare "host:port" means SOCKS5, which
// is what the setting has always been taken to mean; "socks5://", "socks5h://",
// "http://" and "https://" addresses are honoured as written, and may carry
// credentials.
func parseProxyUrl(httpProxy string) *url.URL {
	if httpProxy == "" {
		return nil
	}

	if !strings.Contains(httpProxy, "://") {
		httpProxy = "socks5://" + httpProxy
	}

	parsedUrl, err := url.Parse(httpProxy)
	if err != nil || parsedUrl.Host == "" {
		fmt.Printf("httpProxy is not a valid proxy address, outbound traffic is left unproxied: %s\n", httpProxy)
		return nil
	}

	fmt.Printf("Proxy enabled for outbound traffic: %s\n", parsedUrl.Redacted())
	return parsedUrl
}
