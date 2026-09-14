package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The tool has to work before it has ever been configured.
func TestMissingFileYieldsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a missing file must not be an error: %v", err)
	}
	if cfg.Interval != DefaultInterval || cfg.Timeout != DefaultTimeout {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if cfg.Language != DefaultLanguage {
		t.Errorf("Language = %q, want %q", cfg.Language, DefaultLanguage)
	}
	if cfg.WebhookURL != "" {
		t.Error("a webhook must never have a default value")
	}
}

// A typo in the user's own settings must surface. Falling back to defaults
// would measure something they did not ask for.
func TestMalformedFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("a malformed configuration file must be reported")
	}
}

func TestBadDurationIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"interval":"soon"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("an unparsable duration must be reported")
	}
}

func TestFileValuesAreApplied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
		"endpoints": ["https://example.invalid/file"],
		"layer2": "203.0.113.53",
		"interval": "45s",
		"retries": 0,
		"no_history": true,
		"language": "zh-TW"
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0] != "https://example.invalid/file" {
		t.Errorf("Endpoints = %v", cfg.Endpoints)
	}
	if cfg.Layer2 != "203.0.113.53" {
		t.Errorf("Layer2 = %q", cfg.Layer2)
	}
	if cfg.Interval != 45*time.Second {
		t.Errorf("Interval = %v", cfg.Interval)
	}
	// A deliberate zero must survive. This is why the optional scalars are
	// pointers: without them zero is indistinguishable from omitted.
	if cfg.Retries != 0 {
		t.Errorf("Retries = %d, want the configured 0", cfg.Retries)
	}
	if !cfg.NoHistory {
		t.Error("NoHistory = false, want the configured true")
	}
	if cfg.Language != "zh-TW" {
		t.Errorf("Language = %q", cfg.Language)
	}
}

// The webhook is a credential, and an environment variable is how a credential
// reaches a process without being written down.
func TestEnvironmentOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"webhook_url":"https://from-file.invalid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(WebhookEnvVar, "https://from-env.invalid")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebhookURL != "https://from-env.invalid" {
		t.Errorf("WebhookURL = %q, want the environment value", cfg.WebhookURL)
	}
}

func TestNoColorEnvironmentVariable(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The convention is that its presence is what counts, whatever the value.
	if !cfg.NoColor {
		t.Error("NO_COLOR present but colour not disabled")
	}
}
