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

package agentauth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeCodexEnv turns the test binary into a stand-in for the agent's own
// sign-in, so the lifecycle is exercised against a real child process rather
// than a mock of one. The value picks which sign-in it acts out.
const fakeCodexEnv = "CASBIN_GATEWAY_FAKE_CODEX"

const (
	// fakeWaits is the sign-in this fix is about: the agent prints the address,
	// starts listening, and waits for a browser that never comes back.
	fakeWaits = "waits"
	// fakeFails is an agent that cannot sign in at all.
	fakeFails = "fails"
	// fakeSucceeds is an agent that signs in and writes the credential.
	fakeSucceeds = "succeeds"
)

const fakeUrl = "https://auth.example.com/oauth/authorize?state=test"

func TestMain(m *testing.M) {
	if behaviour := os.Getenv(fakeCodexEnv); behaviour != "" {
		fakeCodex(behaviour)
		return
	}
	os.Exit(m.Run())
}

func fakeCodex(behaviour string) {
	switch behaviour {
	case fakeWaits:
		fmt.Println("Open this URL to sign in: " + fakeUrl)
		// Long enough that only being stopped ends it.
		time.Sleep(10 * time.Minute)
	case fakeFails:
		fmt.Println("gateway could not reach the sign-in service")
		os.Exit(1)
	case fakeSucceeds:
		fmt.Println("Open this URL to sign in: " + fakeUrl)
		auth := `{"tokens":{"access_token":"a","refresh_token":"r"},"last_refresh":null}`
		if err := os.WriteFile(filepath.Join(os.Getenv("CODEX_HOME"), credentialFile), []byte(auth), 0o600); err != nil {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

// startFake runs one sign-in against the stand-in agent.
func startFake(t *testing.T, behaviour string, save func(Credential) error) (Session, error) {
	t.Helper()
	t.Setenv(fakeCodexEnv, behaviour)
	if save == nil {
		save = func(Credential) error { return nil }
	}
	return StartLogin("codex", os.Args[0], save)
}

// resetSessions clears what earlier tests left, and stops anything still
// running so a stand-in agent does not outlive the test that started it.
func resetSessions(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		sessions.Lock()
		started := make([]*session, 0, len(sessions.byId))
		for _, kept := range sessions.byId {
			started = append(started, kept)
		}
		sessions.byId = map[string]*session{}
		sessions.active = nil
		sessions.Unlock()

		for _, kept := range started {
			kept.stop(errLoginCancelled)
		}
	})
}

// awaitFinished polls one session the way the page does.
func awaitFinished(t *testing.T, id string, within time.Duration) Session {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		session, ok := LoginSession(id)
		if !ok {
			t.Fatalf("session %s was forgotten while it was being polled", id)
		}
		if !session.Running {
			return session
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session %s was still running after %s", id, within)
	return Session{}
}

// A second sign-in started while one is genuinely waiting for the browser is
// still refused: the agent listens on a fixed port, so two would collide.
func TestSecondSignInIsRefusedWhileOneWaits(t *testing.T) {
	resetSessions(t)

	first, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}
	if !first.Running || first.Url != fakeUrl {
		t.Fatalf("StartLogin() = %+v, want it running with the address the agent printed", first)
	}

	if _, err := startFake(t, fakeWaits, nil); err == nil {
		t.Fatal("StartLogin() while one waits = nil, want it refused")
	} else if !strings.Contains(err.Error(), "already waiting") {
		t.Errorf("StartLogin() while one waits = %v, want it to say one is already waiting", err)
	}
}

// The reported bug: the browser page is closed without signing in, which the
// agent cannot see. Cancelling has to end it and let the next one start.
func TestCancelReleasesTheSignInSlot(t *testing.T) {
	resetSessions(t)

	started, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}

	cancelled, err := CancelLogin(started.Id)
	if err != nil {
		t.Fatalf("CancelLogin() = %v, want it cancelled", err)
	}
	if cancelled.Running || cancelled.Ok || !cancelled.Cancelled {
		t.Fatalf("CancelLogin() = %+v, want a finished, cancelled sign-in", cancelled)
	}
	if cancelled.Error != errLoginCancelled.Error() {
		t.Errorf("CancelLogin().Error = %q, want %q", cancelled.Error, errLoginCancelled)
	}
	if cancelled.EndTime == "" {
		t.Error("CancelLogin() left no end time")
	}

	// What the user does next: sign in again.
	retried, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() after a cancel = %v, want a started sign-in", err)
	}
	if !retried.Running {
		t.Errorf("StartLogin() after a cancel = %+v, want it running", retried)
	}
}

// Cancelling twice, or cancelling something already finished, leaves the
// outcome alone rather than rewriting it.
func TestCancelIsIdempotent(t *testing.T) {
	resetSessions(t)

	started, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}

	first, err := CancelLogin(started.Id)
	if err != nil {
		t.Fatalf("CancelLogin() = %v, want it cancelled", err)
	}
	second, err := CancelLogin(started.Id)
	if err != nil {
		t.Fatalf("CancelLogin() a second time = %v, want it accepted", err)
	}
	if second.EndTime != first.EndTime || second.Error != first.Error {
		t.Errorf("CancelLogin() a second time = %+v, want the first outcome %+v", second, first)
	}
}

func TestCancelUnknownSession(t *testing.T) {
	resetSessions(t)

	if _, err := CancelLogin("not-a-session"); err == nil {
		t.Fatal("CancelLogin() on an unknown id = nil, want an error")
	}
}

// An agent that cannot sign in must not hold the slot behind it either.
func TestFailedSignInReleasesTheSignInSlot(t *testing.T) {
	resetSessions(t)

	started, err := startFake(t, fakeFails, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}

	finished := awaitFinished(t, started.Id, 30*time.Second)
	if finished.Ok || finished.Error == "" {
		t.Fatalf("a failed sign-in = %+v, want it reported as failed", finished)
	}
	if finished.Cancelled {
		t.Errorf("a failed sign-in = %+v, want it not marked cancelled", finished)
	}

	if _, err := startFake(t, fakeWaits, nil); err != nil {
		t.Fatalf("StartLogin() after a failure = %v, want a started sign-in", err)
	}
}

// A sign-in that goes through releases the slot and stores what it brought back.
func TestSuccessfulSignInReleasesTheSignInSlot(t *testing.T) {
	resetSessions(t)

	saved := make(chan Credential, 1)
	started, err := startFake(t, fakeSucceeds, func(credential Credential) error {
		saved <- credential
		return nil
	})
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}

	finished := awaitFinished(t, started.Id, 30*time.Second)
	if !finished.Ok || finished.Error != "" {
		t.Fatalf("a finished sign-in = %+v, want it reported as ok", finished)
	}
	select {
	case credential := <-saved:
		if credential.Kind != KindSubscription {
			t.Errorf("saved credential kind = %q, want %q", credential.Kind, KindSubscription)
		}
	default:
		t.Error("a finished sign-in saved no credential")
	}

	if _, err := startFake(t, fakeWaits, nil); err != nil {
		t.Fatalf("StartLogin() after a success = %v, want a started sign-in", err)
	}
}

// Nobody has to cancel for an abandoned sign-in to be released: it times out.
func TestTimeoutReleasesTheSignInSlot(t *testing.T) {
	resetSessions(t)

	restore := loginTimeout
	loginTimeout = 500 * time.Millisecond
	t.Cleanup(func() { loginTimeout = restore })

	started, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}

	finished := awaitFinished(t, started.Id, 30*time.Second)
	if finished.Ok || finished.Error != errLoginTimedOut.Error() {
		t.Fatalf("a timed-out sign-in = %+v, want %q", finished, errLoginTimedOut)
	}

	loginTimeout = restore
	if _, err := startFake(t, fakeWaits, nil); err != nil {
		t.Fatalf("StartLogin() after a timeout = %v, want a started sign-in", err)
	}
}

// A finished sign-in stays readable while the page is still polling it, and is
// forgotten once nothing is.
func TestFinishedSessionsAreForgottenEventually(t *testing.T) {
	resetSessions(t)

	started, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}
	if _, err := CancelLogin(started.Id); err != nil {
		t.Fatalf("CancelLogin() = %v, want it cancelled", err)
	}
	if _, ok := LoginSession(started.Id); !ok {
		t.Fatal("a just-cancelled sign-in was forgotten before the page could read it")
	}

	restore := finishedTtl
	finishedTtl = -time.Second
	t.Cleanup(func() { finishedTtl = restore })

	next, err := startFake(t, fakeWaits, nil)
	if err != nil {
		t.Fatalf("StartLogin() = %v, want a started sign-in", err)
	}
	if _, ok := LoginSession(started.Id); ok {
		t.Error("a sign-in nobody is polling any more was kept")
	}
	if _, ok := LoginSession(next.Id); !ok {
		t.Error("the running sign-in was forgotten")
	}
}
