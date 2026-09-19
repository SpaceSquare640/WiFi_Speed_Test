// Package config loads settings from the platform configuration directory.
//
// Precedence runs command line flags, then environment, then file, then
// built-in defaults.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Directory and file names under the platform configuration directory.
const (
	DirName         = "wifitest"
	FileName        = "config.json"
	HistoryName     = "history.jsonl"
	WebhookEnvVar   = "WIFITEST_WEBHOOK_URL"
	DefaultLanguage = "en"
)

// Defaults for a run that has never been configured.
const (
	DefaultInterval = 30 * time.Second
	DefaultTimeout  = 10 * time.Second
	DefaultRetries  = 3
	DefaultSamples  = 3
	DefaultStreams  = 1
)

// Config is the resolved configuration for a run.
type Config struct {
	Endpoints []string
	Layer1    string
	Layer2    string
	Layer3    string

	Interval time.Duration
	Timeout  time.Duration
	Retries  int
	Samples  int
	Streams  int

	// WebhookURL is a credential. It is read from the configuration file or the
	// environment and is never written to source or to version control. The
	// notifier stays disabled while it is empty; there is no default recipient.
	WebhookURL string

	HistoryPath string
	NoHistory   bool

	Language string
	NoColor  bool
}

// file is the on-disk shape. It is separate from Config so that the file format
// is an explicit contract rather than whatever the internal struct happens to
// look like, and so durations can be written the way people write them.
type file struct {
	Endpoints []string `json:"endpoints,omitempty"`
	Layer1    string   `json:"layer1,omitempty"`
	Layer2    string   `json:"layer2,omitempty"`
	Layer3    string   `json:"layer3,omitempty"`

	Interval string `json:"interval,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
	Retries  *int   `json:"retries,omitempty"`
	Samples  *int   `json:"samples,omitempty"`
	Streams  *int   `json:"streams,omitempty"`

	WebhookURL string `json:"webhook_url,omitempty"`

	HistoryPath string `json:"history_path,omitempty"`
	NoHistory   *bool  `json:"no_history,omitempty"`

	Language string `json:"language,omitempty"`
	NoColor  *bool  `json:"no_color,omitempty"`
}

// Defaults returns the configuration used before anything is set.
func Defaults() Config {
	return Config{
		Interval: DefaultInterval,
		Timeout:  DefaultTimeout,
		Retries:  DefaultRetries,
		Samples:  DefaultSamples,
		Streams:  DefaultStreams,
		Language: DefaultLanguage,
	}
}

// Dir returns the directory holding this tool's configuration and history.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, DirName), nil
}

// DefaultPath returns the platform's configuration file location.
func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// DefaultHistoryPath returns the platform's history file location.
func DefaultHistoryPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, HistoryName), nil
}

// Load reads the file at path. A missing file is not an error: it yields
// defaults, because the tool must work before it has ever been configured.
//
// A malformed file is an error. Silently falling back to defaults there would
// hide a typo in the user's own settings and measure something they did not ask
// for.
func Load(path string) (Config, error) {
	cfg := Defaults()

	if path == "" {
		var err error
		if path, err = DefaultPath(); err != nil {
			return applyEnv(cfg), nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return applyEnv(cfg), nil
		}
		return applyEnv(cfg), err
	}

	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return applyEnv(cfg), err
	}
	if err := f.merge(&cfg); err != nil {
		return applyEnv(cfg), err
	}
	return applyEnv(cfg), nil
}

// merge copies set fields over the configuration. Absent fields keep their
// default, which is why the optional scalars are pointers: without them a
// deliberate false or zero would be indistinguishable from an omission.
func (f file) merge(cfg *Config) error {
	if len(f.Endpoints) > 0 {
		cfg.Endpoints = f.Endpoints
	}
	if f.Layer1 != "" {
		cfg.Layer1 = f.Layer1
	}
	if f.Layer2 != "" {
		cfg.Layer2 = f.Layer2
	}
	if f.Layer3 != "" {
		cfg.Layer3 = f.Layer3
	}
	if f.Interval != "" {
		d, err := time.ParseDuration(f.Interval)
		if err != nil {
			return err
		}
		cfg.Interval = d
	}
	if f.Timeout != "" {
		d, err := time.ParseDuration(f.Timeout)
		if err != nil {
			return err
		}
		cfg.Timeout = d
	}
	if f.Retries != nil {
		cfg.Retries = *f.Retries
	}
	if f.Samples != nil {
		cfg.Samples = *f.Samples
	}
	if f.Streams != nil {
		cfg.Streams = *f.Streams
	}
	if f.WebhookURL != "" {
		cfg.WebhookURL = f.WebhookURL
	}
	if f.HistoryPath != "" {
		cfg.HistoryPath = f.HistoryPath
	}
	if f.NoHistory != nil {
		cfg.NoHistory = *f.NoHistory
	}
	if f.Language != "" {
		cfg.Language = f.Language
	}
	if f.NoColor != nil {
		cfg.NoColor = *f.NoColor
	}
	return nil
}

// applyEnv lets the environment override the file.
//
// Only the webhook is read this way, and deliberately: it is the one credential
// here, and an environment variable is how a credential reaches a process
// without being written down.
func applyEnv(cfg Config) Config {
	if url := os.Getenv(WebhookEnvVar); url != "" {
		cfg.WebhookURL = url
	}
	// NO_COLOR is honoured by convention across command line tools; its mere
	// presence, whatever the value, means no colour.
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		cfg.NoColor = true
	}
	return cfg
}
