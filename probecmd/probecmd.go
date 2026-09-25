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

// Package probecmd is "probe": the authenticity suite run once against an
// endpoint and a key, with no database, no server and nothing installed.
package probecmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/object"
)

// commandName is how the usage line names this program.
var commandName = "gateway-probe"

// RunCommand handles "probe" and reports whether it did.
func RunCommand(args []string) bool {
	if len(args) < 2 || args[1] != "probe" {
		return false
	}
	commandName = "casbin-gateway probe"
	os.Exit(Run(args[2:], os.Stdout, os.Stderr))
	return true
}

type options struct {
	baseUrl  string
	key      string
	model    string
	protocol string
	card     string
	noCard   bool
	hideHost bool
	json     bool
	lang     string
}

// Run is the whole command, so the standalone binary and the server's
// subcommand are the same program. It returns the exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseArgs(args, stderr)
	if err == flag.ErrHelp {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	text := textFor(opts.lang)

	if !opts.json {
		fmt.Fprintf(stdout, text.starting, displayHost(opts.baseUrl, opts.hideHost), opts.protocol)
	}
	probe := object.ProbeEndpoint(opts.baseUrl, opts.key, opts.protocol, opts.model)

	if opts.json {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(probe); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	} else {
		printReport(stdout, probe, text)
	}

	if probe.Grade == object.ProbeGradeUnknown {
		return 1
	}
	if !opts.noCard {
		path := opts.card
		if path == "" {
			path = defaultCardPath(opts.baseUrl, opts.hideHost)
		}
		if err := writeCard(path, probe, displayHost(opts.baseUrl, opts.hideHost), opts.protocol); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stderr, text.cardSaved, path)
	}
	return 0
}

func parseArgs(args []string, stderr io.Writer) (*options, error) {
	opts := &options{}
	flags := flag.NewFlagSet("probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.key, "key", "", "API key; defaults to $GATEWAY_PROBE_KEY, then $ANTHROPIC_API_KEY or $OPENAI_API_KEY")
	flags.StringVar(&opts.model, "model", "", "model to ask for; defaults to the first one the endpoint lists")
	flags.StringVar(&opts.protocol, "protocol", "", "openai or anthropic; guessed from the URL and model when omitted")
	flags.StringVar(&opts.card, "card", "", "where to save the shareable PNG card")
	flags.BoolVar(&opts.noCard, "no-card", false, "do not save a card")
	flags.BoolVar(&opts.hideHost, "hide-host", false, "mask the endpoint's host on the report and the card")
	flags.BoolVar(&opts.json, "json", false, "print the full report, requests and answers included, as JSON")
	flags.StringVar(&opts.lang, "lang", "", "en or zh; follows the system language when omitted")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s <base-url> [--key KEY] [--model MODEL] [--protocol openai|anthropic] [flags]\n", commandName)
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Asks an OpenAI- or Anthropic-compatible endpoint questions with checkable answers and grades")
		fmt.Fprintln(stderr, "whether what answers is the API it claims to be. It sends about twenty small requests on your key.")
		fmt.Fprintln(stderr, "")
		flags.PrintDefaults()
	}

	// Flags may come before or after the URL.
	positional := []string{}
	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		if flags.NArg() == 0 {
			break
		}
		positional = append(positional, flags.Arg(0))
		args = flags.Args()[1:]
	}
	if len(positional) != 1 {
		flags.Usage()
		return nil, fmt.Errorf("probe needs exactly one base URL")
	}

	opts.baseUrl = strings.TrimSpace(positional[0])
	if !strings.Contains(opts.baseUrl, "://") {
		opts.baseUrl = "https://" + opts.baseUrl
	}
	if parsed, err := url.Parse(opts.baseUrl); err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("%q is not a URL", positional[0])
	}

	switch strings.ToLower(opts.protocol) {
	case "":
		opts.protocol = guessProtocol(opts.baseUrl, opts.model)
	case "openai":
		opts.protocol = object.ProtocolOpenAi
	case "anthropic", "claude":
		opts.protocol = object.ProtocolAnthropic
	default:
		return nil, fmt.Errorf("unknown protocol %q, use openai or anthropic", opts.protocol)
	}

	if opts.key == "" {
		opts.key = keyFromEnv(opts.protocol)
	}
	if opts.key == "" {
		return nil, fmt.Errorf("no API key: pass --key, or set GATEWAY_PROBE_KEY so it stays out of your shell history")
	}
	return opts, nil
}

// guessProtocol picks Anthropic for an endpoint or a model that says so, which
// is what a relay selling Claude Code access usually serves.
func guessProtocol(baseUrl string, model string) string {
	lowered := strings.ToLower(baseUrl)
	if strings.Contains(lowered, "anthropic") || strings.Contains(lowered, "claude") ||
		strings.HasPrefix(strings.ToLower(model), "claude") {
		return object.ProtocolAnthropic
	}
	return object.ProtocolOpenAi
}

func keyFromEnv(protocol string) string {
	names := []string{"GATEWAY_PROBE_KEY"}
	if protocol == object.ProtocolAnthropic {
		names = append(names, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN")
	} else {
		names = append(names, "OPENAI_API_KEY")
	}
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func hostOf(baseUrl string) string {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		return baseUrl
	}
	return parsed.Host
}

// displayHost is the host as the report shows it. Masked, each label keeps its
// first letter and the last label stays whole, so a card can be posted without
// naming the seller.
func displayHost(baseUrl string, hide bool) string {
	host := hostOf(baseUrl)
	if !hide {
		return host
	}
	labels := strings.Split(host, ".")
	for index, label := range labels {
		if index == len(labels)-1 && len(labels) > 1 {
			break
		}
		if len(label) > 1 {
			labels[index] = label[:1] + strings.Repeat("*", len(label)-1)
		}
	}
	return strings.Join(labels, ".")
}

func defaultCardPath(baseUrl string, hide bool) string {
	name := "endpoint"
	if !hide {
		name = strings.NewReplacer(":", "-", "*", "").Replace(hostOf(baseUrl))
	}
	return fmt.Sprintf("probe-%s-%s.png", name, time.Now().Format("20060102"))
}
