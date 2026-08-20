package controllers

import (
	"net/http"
	"testing"

	"github.com/apache/casbin-gateway/object"
)

func TestModelEndpointCandidates(t *testing.T) {
	actual, err := object.BuildModelEndpointCandidates("https://api.deepseek.com/anthropic", "custom")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"https://api.deepseek.com/anthropic/v1/models",
		"https://api.deepseek.com/anthropic/models",
		"https://api.deepseek.com/v1/models",
		"https://api.deepseek.com/models",
	}
	if len(actual) != len(expected) {
		t.Fatalf("got %v, want %v", actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Errorf("candidate %d = %q, want %q", index, actual[index], expected[index])
		}
	}
}

func TestCopyRequestHeaders(t *testing.T) {
	source := http.Header{
		"Authorization":     {"client-token"},
		"X-Api-Key":         {"client-key"},
		"Connection":        {"keep-alive, X-Remove-Me"},
		"X-Remove-Me":       {"secret"},
		"Anthropic-Beta":    {"tools-2026"},
		"Anthropic-Version": {"2023-06-01"},
		"User-Agent":        {"claude-code"},
	}
	destination := http.Header{}
	copyRequestHeaders(destination, source)
	for _, name := range []string{"Authorization", "X-Api-Key", "Connection", "X-Remove-Me"} {
		if destination.Get(name) != "" {
			t.Errorf("%s was forwarded", name)
		}
	}
	for _, name := range []string{"Anthropic-Beta", "Anthropic-Version", "User-Agent"} {
		if destination.Get(name) != source.Get(name) {
			t.Errorf("%s was not preserved", name)
		}
	}
}
