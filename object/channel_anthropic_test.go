package object

import (
	"net/url"
	"testing"
)

func TestValidateAnthropicChannel(t *testing.T) {
	channel := &Channel{
		Type: "anthropic", BaseUrl: "https://example.com", AuthType: "x-api-key",
		DefaultModel: " default ", HaikuModel: "haiku", SonnetModel: "sonnet", OpusModel: "opus",
	}
	if err := validateChannel(channel); err != nil {
		t.Fatal(err)
	}
	if channel.DefaultModel != "default" {
		t.Fatalf("model whitespace was not trimmed: %q", channel.DefaultModel)
	}
	channel.OpusModel = ""
	if err := validateChannel(channel); err == nil {
		t.Fatal("missing Anthropic model mapping was accepted")
	}
	channel.OpusModel = "opus"
	channel.AuthType = "query"
	if err := validateChannel(channel); err == nil {
		t.Fatal("invalid auth type was accepted")
	}
}

func TestBuildAnthropicMessagesUrl(t *testing.T) {
	cases := map[string]string{
		"https://api.anthropic.com":          "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/":         "https://api.anthropic.com/v1/messages",
		"https://example.com/v1":             "https://example.com/v1/messages",
		"https://example.com/v1/messages":    "https://example.com/v1/messages",
		"https://api.deepseek.com/anthropic": "https://api.deepseek.com/anthropic/v1/messages",
		"https://example.com/prefix/v1":      "https://example.com/prefix/v1/messages",
	}
	for input, expected := range cases {
		actual, err := BuildAnthropicMessagesUrl(input, "")
		if err != nil {
			t.Errorf("%s: %v", input, err)
		} else if actual != expected {
			t.Errorf("BuildAnthropicMessagesUrl(%q) = %q, want %q", input, actual, expected)
		}
	}

	actual, err := BuildAnthropicMessagesUrl("https://example.com/base?source=gateway#ignored", "beta=true&source=client")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(actual)
	if parsed.Fragment != "" || parsed.Query().Get("beta") != "true" || len(parsed.Query()["source"]) != 2 {
		t.Fatalf("query merge or fragment removal failed: %s", actual)
	}
}

func TestAnthropicModel(t *testing.T) {
	channel := &Channel{DefaultModel: "d", HaikuModel: "h", SonnetModel: "s", OpusModel: "o"}
	for alias, expected := range map[string]string{
		"casbin-default": "d", "casbin-haiku": "h", "casbin-sonnet": "s", "casbin-opus": "o", "other": "",
	} {
		if actual := channel.AnthropicModel(alias); actual != expected {
			t.Errorf("AnthropicModel(%q) = %q, want %q", alias, actual, expected)
		}
	}
}

func TestKnownProviderModelEndpoint(t *testing.T) {
	cases := []struct {
		provider string
		baseUrl  string
		expected string
		authType string
	}{
		{"anthropic", "https://api.anthropic.com", "https://api.anthropic.com/v1/models", "x-api-key"},
		{"deepseek", "https://api.deepseek.com/anthropic", "https://api.deepseek.com/models", "bearer"},
		{"kimi-for-coding", "https://api.kimi.com/coding/", "https://api.kimi.com/coding/v1/models", "bearer"},
		{"minimax", "https://api.minimaxi.com/anthropic", "https://api.minimaxi.com/anthropic/v1/models", "x-api-key"},
		{"openrouter", "https://openrouter.ai/api", "https://openrouter.ai/api/v1/models", "bearer"},
		{"longcat", "https://api.longcat.chat/anthropic", "https://api.longcat.chat/anthropic/v1/models", "bearer"},
	}
	for _, tc := range cases {
		actual, err := BuildModelEndpointCandidates(tc.baseUrl, tc.provider)
		if err != nil {
			t.Errorf("%s: %v", tc.provider, err)
			continue
		}
		if len(actual) != 1 || actual[0] != tc.expected {
			t.Errorf("%s candidates = %v", tc.provider, actual)
		}
		if actualAuth := ModelEndpointAuthType(tc.provider, "bearer"); actualAuth != tc.authType {
			t.Errorf("%s auth type = %s, want %s", tc.provider, actualAuth, tc.authType)
		}
	}
	if _, err := BuildModelEndpointCandidates("https://example.com/anthropic", "deepseek"); err == nil {
		t.Fatal("DeepSeek endpoint was allowed for a different origin")
	}
}
