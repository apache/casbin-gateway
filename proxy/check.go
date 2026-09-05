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

package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
)

const (
	dialTimeout    = 5 * time.Second
	requestTimeout = 12 * time.Second
)

// CheckResult is what one test of the outbound proxy found.
type CheckResult struct {
	// Address is the proxy as it was parsed, with any password hidden.
	Address string `json:"address"`
	// Dialed reports that the proxy port itself accepted a connection, which is
	// the difference between a proxy that is down and one that cannot get out.
	Dialed     bool   `json:"dialed"`
	DialMillis int64  `json:"dialMillis"`
	DialError  string `json:"dialError,omitempty"`
	// ExitIp and ExitCountry are where the traffic left from, which is how an
	// admin tells a local relay from one abroad.
	ExitIp      string        `json:"exitIp,omitempty"`
	ExitCountry string        `json:"exitCountry,omitempty"`
	Targets     []CheckTarget `json:"targets"`
}

// CheckTarget is one upstream the test fetched through the proxy.
type CheckTarget struct {
	Name string `json:"name"`
	Url  string `json:"url"`
	// Restricted marks an upstream that a blocked network cannot reach on its
	// own, so reaching it is what says the proxy leaves that network.
	Restricted bool   `json:"restricted,omitempty"`
	Ok         bool   `json:"ok"`
	Status     int    `json:"status,omitempty"`
	Millis     int64  `json:"millis"`
	Error      string `json:"error,omitempty"`
}

// The upstreams worth knowing about: the one a blocked network cannot reach,
// and the two the installs and the update check download from.
var checkTargets = []CheckTarget{
	{Name: "Google", Url: "https://www.google.com/generate_204", Restricted: true},
	{Name: "npm registry", Url: "https://registry.npmjs.org/-/ping"},
	{Name: "GitHub API", Url: "https://api.github.com/"},
}

// Check tries the proxy address given, or the stored setting when it is empty,
// and reports what answered. It stores nothing, so the Settings page can try a
// value before saving it.
func Check(address string) (*CheckResult, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = strings.TrimSpace(conf.GetConfigStringUnquoted("httpProxy"))
	}
	if address == "" {
		return nil, errors.New("no outbound proxy is configured")
	}

	parsedUrl, err := parseProxyAddress(address)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{Address: parsedUrl.Redacted()}

	started := time.Now()
	connection, err := net.DialTimeout("tcp", dialAddress(parsedUrl), dialTimeout)
	result.DialMillis = time.Since(started).Milliseconds()
	if err != nil {
		result.DialError = err.Error()
		return result, nil
	}
	result.Dialed = true
	_ = connection.Close()

	result.Targets = make([]CheckTarget, len(checkTargets))
	client := checkClient(parsedUrl)

	var group sync.WaitGroup
	for i := range checkTargets {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			result.Targets[i] = fetch(client, checkTargets[i])
		}(i)
	}
	group.Add(1)
	go func() {
		defer group.Done()
		result.ExitIp, result.ExitCountry = exitAddress(client)
	}()
	group.Wait()

	return result, nil
}

// checkClient sends its requests through the proxy under test rather than
// through the shared transport, which holds the stored proxy.
func checkClient(parsedUrl *url.URL) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(*http.Request) (*url.URL, error) { return parsedUrl, nil }
	return &http.Client{Timeout: requestTimeout, Transport: WithUserAgent(transport)}
}

func fetch(client *http.Client, target CheckTarget) CheckTarget {
	started := time.Now()
	response, err := client.Get(target.Url)
	target.Millis = time.Since(started).Milliseconds()
	if err != nil {
		target.Error = requestError(err)
		return target
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	target.Status = response.StatusCode
	// A 4xx from the upstream still proves the request got there, which is what
	// is being tested; only a 5xx is worth reporting as a failure.
	target.Ok = response.StatusCode < 500
	return target
}

// exitAddress reads where the traffic leaves from. Cloudflare answers this on
// every edge as a few "key=value" lines.
func exitAddress(client *http.Client) (string, string) {
	response, err := client.Get("https://www.cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return "", ""
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", ""
	}

	var ip, country string
	for _, line := range strings.Split(string(body), "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		switch name {
		case "ip":
			ip = value
		case "loc":
			country = value
		}
	}
	return ip, country
}

// dialAddress is the proxy's host and port, filling in the port each scheme is
// usually served on when the setting left it out.
func dialAddress(parsedUrl *url.URL) string {
	if parsedUrl.Port() != "" {
		return parsedUrl.Host
	}

	port := "1080"
	switch parsedUrl.Scheme {
	case "http":
		port = "80"
	case "https":
		port = "443"
	}
	return net.JoinHostPort(parsedUrl.Hostname(), port)
}

// requestError drops the URL the transport repeats back, which the page already
// shows beside the error.
func requestError(err error) string {
	message := err.Error()
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		message = urlErr.Err.Error()
	}
	return message
}
