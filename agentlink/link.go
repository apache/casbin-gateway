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

// Package agentlink sends one link in an agent's own URL scheme to one copy of
// that agent.
//
// A sign-in finished in a browser comes back as a link the agent registered
// itself for - claude://... - and the command behind that registration names no
// state directory. The copy it starts therefore joins the one running on the
// default state directory, which is where the sign-in lands, so a second copy
// could never be signed in at all. While a copy is waiting to be signed in,
// Gateway holds the scheme, hands the one link that follows to that copy, and
// puts the agent's own command back.
package agentlink

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agenthome"
)

// Subcommand is how this executable is invoked as the handler of one link, by
// both the server and the desktop launcher.
const Subcommand = "open-agent-link"

// holdInterval is how often a live capture checks that the scheme is still
// Gateway's: an agent registers itself again as it starts, which is exactly
// when a capture is taken.
const holdInterval = time.Second

// maxUrlLength caps what is accepted as a link, which arrives from a browser.
const maxUrlLength = 4096

// linksDir holds one file per captured scheme, beside the instance state.
var linksDir = filepath.Join(".casbin-gateway", "agent-links")

// ErrUnsupported is returned where Gateway cannot take a URL scheme over.
var ErrUnsupported = errors.New("gateway can only route an agent's own links on Windows")

// Target is the copy of an agent a captured link is handed to: the command that
// instance is started with.
type Target struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

// Claim is one scheme Gateway holds while it waits for a sign-in link.
type Claim struct {
	Scheme string `json:"scheme"`
	// Instance is the stored name of the copy the link is meant for, which is
	// what the pages match a capture to.
	Instance string    `json:"instance"`
	Target   Target    `json:"target"`
	Expires  time.Time `json:"expires"`
	// Restore is the command the agent had registered, put back as soon as the
	// link has been routed or the wait has run out.
	Restore string `json:"restore"`
}

var (
	mutex sync.Mutex
	// held counts the captures taken per scheme, so the goroutine holding one
	// can tell that a later capture has replaced it.
	held = map[string]int{}
)

// Supported reports whether Gateway can route an agent's own links on this
// host.
func Supported() bool {
	return schemesSupported()
}

// Capture points a scheme at Gateway and records the copy the next link belongs
// to. The command the agent had registered is kept, and put back once the link
// has arrived.
func Capture(scheme string, instance string, target Target, ttl time.Duration) (Claim, error) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if err := checkScheme(scheme); err != nil {
		return Claim{}, err
	}
	if target.Executable == "" {
		return Claim{}, errors.New("no launcher was found for this instance")
	}
	if !Supported() {
		return Claim{}, ErrUnsupported
	}

	mutex.Lock()
	defer mutex.Unlock()

	registered, err := readHandler(scheme)
	if err != nil {
		return Claim{}, err
	}
	claim := Claim{
		Scheme:   scheme,
		Instance: instance,
		Target:   target,
		Expires:  time.Now().Add(ttl),
		Restore:  registered,
	}
	switch {
	case isOurs(registered):
		// Gateway already holds this scheme, so what it replaced is recorded
		// only in the capture it was taken for.
		previous, ok := readClaim(scheme)
		if !ok || previous.Restore == "" {
			return Claim{}, fmt.Errorf("gateway holds %s:// with nothing recorded to put back", scheme)
		}
		claim.Restore = previous.Restore
	case registered == "":
		return Claim{}, fmt.Errorf("nothing on this account opens %s:// links", scheme)
	}

	if err := writeClaim(claim); err != nil {
		return Claim{}, err
	}
	command, err := handlerCommand()
	if err != nil {
		_ = removeClaim(scheme)
		return Claim{}, err
	}
	if err := writeHandler(scheme, command); err != nil {
		_ = removeClaim(scheme)
		return Claim{}, err
	}
	hold(scheme)
	return claim, nil
}

// Release ends a capture early and gives the scheme back to the agent.
func Release(scheme string) error {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if err := checkScheme(scheme); err != nil {
		return err
	}

	mutex.Lock()
	defer mutex.Unlock()

	claim, ok := readClaim(scheme)
	if !ok {
		delete(held, scheme)
		return nil
	}
	// A capture that is over must not be taken again by the goroutine holding
	// it, which the generation bump is what stops.
	held[scheme]++
	release(scheme, claim)
	return nil
}

// Pending is the capture waiting on one scheme, if it has not run out.
func Pending(scheme string) (Claim, bool) {
	claim, ok := readClaim(strings.ToLower(strings.TrimSpace(scheme)))
	if !ok || time.Now().After(claim.Expires) {
		return Claim{}, false
	}
	return claim, true
}

// Restore picks a still-pending capture back up after Gateway restarts, and
// gives back every other scheme a Gateway that did not get to finish one left
// registered to itself. It runs at startup, where this process is the only
// Gateway there is: the registry keeps whatever the last process wrote, but the
// goroutine that kept reasserting it died with that process.
func Restore() {
	dir, err := linksPath()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()
	for _, entry := range entries {
		scheme := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || scheme == entry.Name() {
			continue
		}
		claim, ok := readClaim(scheme)
		if !ok {
			continue
		}
		if time.Now().After(claim.Expires) {
			release(scheme, claim)
			continue
		}
		if err := resume(scheme, claim); err != nil {
			release(scheme, claim)
		}
	}
}

// resume puts the capture recorded in claim back the way Capture left it -
// registered to Gateway and watched by a hold goroutine - so a restart while a
// sign-in is still pending does not lose it. It is called with the mutex held.
func resume(scheme string, claim Claim) error {
	if !Supported() {
		return ErrUnsupported
	}
	registered, err := readHandler(scheme)
	if err != nil {
		return err
	}
	if !isOurs(registered) {
		if registered != "" {
			claim.Restore = registered
			if err := writeClaim(claim); err != nil {
				return err
			}
		}
		command, err := handlerCommand()
		if err != nil {
			return err
		}
		if err := writeHandler(scheme, command); err != nil {
			return err
		}
	}
	hold(scheme)
	return nil
}

// HandleIfInvoked routes one link and exits, when this process was started as
// the handler of a captured scheme rather than as Gateway itself.
func HandleIfInvoked() {
	if len(os.Args) < 2 || os.Args[1] != Subcommand {
		return
	}
	detachConsole()

	link := ""
	if len(os.Args) > 2 {
		link = os.Args[2]
	}
	if err := Open(link); err != nil {
		// Nobody is watching a process a browser started, so a link that could
		// not be routed leaves its reason beside the captures instead.
		reportOpenError(link, err)
	}
	os.Exit(0)
}

// Open hands one link to the copy waiting for it, or, where none is, to the
// command the agent had registered - which is what a link opened before Gateway
// captured anything.
func Open(link string) error {
	scheme, err := schemeOf(link)
	if err != nil {
		return err
	}
	claim, ok := readClaim(scheme)
	if !ok {
		return fmt.Errorf("no %s:// link was expected here", scheme)
	}

	// One link per capture: the command the agent registered goes back before
	// this one is opened, whether or not it is the link that was waited for.
	mutex.Lock()
	release(scheme, claim)
	mutex.Unlock()

	if time.Now().After(claim.Expires) {
		return openWith(claim.Restore, link)
	}
	return start(claim.Target.Executable, append(append([]string{}, claim.Target.Args...), link))
}

// hold keeps a captured scheme registered to Gateway for as long as the capture
// lasts, since the agent takes the registration back whenever it starts. It is
// called with the mutex held.
func hold(scheme string) {
	held[scheme]++
	generation := held[scheme]

	go func() {
		for {
			time.Sleep(holdInterval)

			mutex.Lock()
			if held[scheme] != generation {
				mutex.Unlock()
				return
			}
			done := reassert(scheme)
			mutex.Unlock()
			if done {
				return
			}
		}
	}()
}

// reassert puts Gateway's command back where the agent has overwritten it, and
// reports whether the capture is over. It is called with the mutex held.
func reassert(scheme string) bool {
	claim, ok := readClaim(scheme)
	if !ok {
		// The handler routed the link and gave the scheme back already.
		delete(held, scheme)
		return true
	}
	if time.Now().After(claim.Expires) {
		release(scheme, claim)
		return true
	}

	registered, err := readHandler(scheme)
	if err != nil || isOurs(registered) {
		return false
	}
	// The agent registered itself again, and an updated one does that from a
	// new path, so that is what a restore has to put back from here on.
	if registered != "" && registered != claim.Restore {
		claim.Restore = registered
		_ = writeClaim(claim)
	}
	if command, err := handlerCommand(); err == nil {
		_ = writeHandler(scheme, command)
	}
	return false
}

// release gives one scheme back to the agent and forgets the capture. It is
// called with the mutex held.
func release(scheme string, claim Claim) {
	delete(held, scheme)
	if registered, err := readHandler(scheme); err == nil && isOurs(registered) && claim.Restore != "" {
		// The capture is the only record of the command being put back, so it
		// is kept until that has actually happened.
		if err := writeHandler(scheme, claim.Restore); err != nil {
			return
		}
	}
	_ = removeClaim(scheme)
}

// isOurs reports whether a registered command runs Gateway's own handler.
func isOurs(command string) bool {
	return strings.Contains(strings.ToLower(command), Subcommand)
}

// schemeOf is the scheme of a link, checked as what it is: an argument a
// browser handed to a command line.
func schemeOf(link string) (string, error) {
	if link == "" {
		return "", errors.New("no link was passed")
	}
	if len(link) > maxUrlLength {
		return "", errors.New("this link is longer than a link can be")
	}
	for _, r := range link {
		if r <= ' ' || r == '"' || r == 0x7f {
			return "", errors.New("this link holds characters a link cannot hold")
		}
	}

	index := strings.Index(link, "://")
	if index <= 0 {
		return "", fmt.Errorf("%q is not a link in an agent's own scheme", link)
	}
	scheme := strings.ToLower(link[:index])
	if err := checkScheme(scheme); err != nil {
		return "", err
	}
	return scheme, nil
}

// checkScheme keeps a scheme to what is safe as one file name and as one
// registry key.
func checkScheme(scheme string) error {
	if scheme == "" {
		return errors.New("the URL scheme is empty")
	}
	if len(scheme) > 32 {
		return errors.New("the URL scheme is longer than 32 characters")
	}
	for i, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return fmt.Errorf("%q is not a URL scheme", scheme)
		}
	}
	return nil
}

func linksPath() (string, error) {
	home, err := agenthome.Resolve("")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, linksDir), nil
}

func claimPath(scheme string) (string, error) {
	if err := checkScheme(scheme); err != nil {
		return "", err
	}
	dir, err := linksPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, scheme+".json"), nil
}

func readClaim(scheme string) (Claim, bool) {
	path, err := claimPath(scheme)
	if err != nil {
		return Claim{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Claim{}, false
	}

	claim := Claim{}
	if json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &claim) != nil || claim.Scheme != scheme {
		return Claim{}, false
	}
	return claim, true
}

func writeClaim(claim Claim) error {
	path, err := claimPath(claim.Scheme)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(claim, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func removeClaim(scheme string) error {
	path, err := claimPath(scheme)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// reportOpenError leaves the last link this host could not route where the
// operator can find it: the handler has no console and no log of its own.
func reportOpenError(link string, cause error) {
	dir, err := linksPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	note := fmt.Sprintf("%s\n%s\n%v\n", time.Now().Format(time.RFC3339), link, cause)
	_ = os.WriteFile(filepath.Join(dir, "last-error.txt"), []byte(note), 0o600)
}
