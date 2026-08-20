package agentpatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeGatewayConfigureAndRestoreManagedKeys(t *testing.T) {
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("agentPatchStateDir", state)
	t.Setenv("httpport", "17123")
	target := Target{AgentId: "claude-code", Path: "/usr/bin/claude"}
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://original.example",
			"ANTHROPIC_API_KEY":  "original-key",
			"UNMANAGED":          "keep-me",
		},
		"permissions": map[string]any{"allow": []any{"Read"}},
	}
	if err := writeJSONConfig(path, initial, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := ConfigureClaudeGateway(target); err != nil {
		t.Fatal(err)
	}
	configured := readTestConfig(t, path)
	env, _ := objectAt(configured["env"])
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:17123" || env["ANTHROPIC_MODEL"] != "casbin-default" {
		t.Fatalf("unexpected Gateway values: %#v", env)
	}
	if _, exists := env["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("ANTHROPIC_API_KEY was not removed")
	}

	// A second configure refreshes managed values without replacing the first
	// backup or losing unrelated changes made in the meantime.
	configured["permissions"] = map[string]any{"allow": []any{"Read", "Bash"}}
	env["ANTHROPIC_BASE_URL"] = "https://manually-changed.example"
	if err := writeJSONConfig(path, configured, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureClaudeGateway(target); err != nil {
		t.Fatal(err)
	}
	if err := RestoreClaudeGateway(target); err != nil {
		t.Fatal(err)
	}

	restored := readTestConfig(t, path)
	restoredEnv, _ := objectAt(restored["env"])
	if restoredEnv["ANTHROPIC_BASE_URL"] != "https://original.example" || restoredEnv["ANTHROPIC_API_KEY"] != "original-key" {
		t.Fatalf("original managed values were not restored: %#v", restoredEnv)
	}
	if restoredEnv["UNMANAGED"] != "keep-me" {
		t.Fatal("unmanaged env value was changed")
	}
	permissions, _ := objectAt(restored["permissions"])
	allow := permissions["allow"].([]any)
	if len(allow) != 2 {
		t.Fatalf("unmanaged configuration change was lost: %#v", restored)
	}
	if _, err := os.Stat(gatewayStatePath(target)); !os.IsNotExist(err) {
		t.Fatal("Gateway backup state was not deleted after restore")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("configuration mode was not preserved: %v, %v", info, err)
	}
}

func readTestConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}
