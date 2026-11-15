package config

import (
	"errors"
	"testing"
	"time"
)

const (
	testName        = "vigie"
	envNamespace    = "env-namespace"
	envOwner        = "env-owner"
	envTokenNS      = "env-token-namespace"
	envTokenSecret  = "env-token-secret"
	envTokenKey     = "env-token-key"
	envOperator     = "env-operator"
	envPROpenLabel  = "env-pr-open"
	envConsoleURL   = "https://console.example"
	flagOwner       = "flag-owner"
	envStaleDays    = 7
	envLabel        = 4
	envInterval     = 5 * time.Second
	flagInterval    = 2 * time.Minute
	negativeSeconds = "-5s"
	envHiddenFile   = "/env/hidden.json"
	flagHiddenFile  = "/flag/hidden.json"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func renamedEnv(prefix string) map[string]string {
	return map[string]string{
		prefix + "NAMESPACE":         envNamespace,
		prefix + "GITHUB_OWNER":      envOwner,
		prefix + "TOKEN_NAMESPACE":   envTokenNS,
		prefix + "TOKEN_SECRET":      envTokenSecret,
		prefix + "TOKEN_KEY":         envTokenKey,
		prefix + "OPERATOR":          envOperator,
		prefix + "IN_PROGRESS_LABEL": "4",
		prefix + "PR_OPEN_LABEL":     envPROpenLabel,
		prefix + "CONSOLE_URL":       envConsoleURL,
		prefix + "STALE_DAYS":        "7",
		prefix + "INTERVAL":          "5s",
		prefix + "HIDDEN_FILE":       envHiddenFile,
	}
}

func wantEnvConfig() Config {
	return Config{
		Namespace: envNamespace, GitHubOwner: envOwner, VikunjaURL: DefaultVikunjaURL,
		TokenNamespace: envTokenNS, TokenSecret: envTokenSecret, TokenKey: envTokenKey,
		Operator: envOperator, InProgressLabel: envLabel, PROpenLabel: envPROpenLabel,
		ConsoleURL: envConsoleURL, StaleDays: envStaleDays, Interval: envInterval,
		HiddenFile: envHiddenFile,
	}
}

func TestParseDefaults(t *testing.T) {
	t.Parallel()
	got, err := Parse(testName, nil, envFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Namespace: DefaultNamespace, GitHubOwner: DefaultGitHubOwner, VikunjaURL: DefaultVikunjaURL,
		TokenNamespace: DefaultTokenNamespace, TokenSecret: DefaultTokenSecret, TokenKey: DefaultTokenKey,
		Operator: DefaultOperator, InProgressLabel: DefaultInProgressLabel, PROpenLabel: DefaultPROpenLabel,
		ConsoleURL: DefaultConsoleURL,
		StaleDays:  DefaultStaleDays, Interval: DefaultInterval,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseEnvAndFlags(t *testing.T) {
	t.Parallel()
	env := renamedEnv(currentEnvPrefix)
	tests := []struct {
		name  string
		args  []string
		check func(Config) bool
	}{
		{"env overrides default", nil, func(c Config) bool {
			return c == wantEnvConfig()
		}},
		{"flag beats env", []string{"-github-owner", flagOwner, "-interval", "2m"}, func(c Config) bool {
			return c.GitHubOwner == flagOwner && c.Interval == flagInterval && c.StaleDays == envStaleDays
		}},
		{"hidden file flag beats env", []string{"-hidden-file", flagHiddenFile}, func(c Config) bool {
			return c.HiddenFile == flagHiddenFile
		}},
		{"once flag", []string{"-once"}, func(c Config) bool { return c.Once }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(testName, tt.args, envFrom(env))
			if err != nil {
				t.Fatal(err)
			}
			if !tt.check(got) {
				t.Errorf("unexpected config %+v", got)
			}
		})
	}
}

func TestParseLegacyEnvFallback(t *testing.T) {
	t.Parallel()
	got, err := Parse(testName, nil, envFrom(renamedEnv(legacyEnvPrefix)))
	if err != nil {
		t.Fatal(err)
	}
	if want := wantEnvConfig(); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseCurrentEnvPrecedesLegacy(t *testing.T) {
	t.Parallel()
	env := renamedEnv(legacyEnvPrefix)
	env[currentEnvPrefix+"GITHUB_OWNER"] = flagOwner
	got, err := Parse(testName, nil, envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if got.GitHubOwner != flagOwner {
		t.Errorf("GitHubOwner = %q, want %q", got.GitHubOwner, flagOwner)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		invalid bool // reported by vigie rather than by the flag package
	}{
		{"bad env int", nil, map[string]string{"VIGIE_STALE_DAYS": "many"}, true},
		{"bad env label", nil, map[string]string{"VIGIE_IN_PROGRESS_LABEL": "x"}, true},
		{"bad env duration", nil, map[string]string{"VIGIE_INTERVAL": "soon"}, true},
		{"bad legacy env int", nil, map[string]string{"ATOP_STALE_DAYS": "many"}, true},
		{"bad legacy env duration", nil, map[string]string{"ATOP_INTERVAL": "soon"}, true},
		{"zero interval flag", []string{"-interval", "0s"}, nil, true},
		{"negative interval env", nil, map[string]string{"VIGIE_INTERVAL": negativeSeconds}, true},
		{"unknown flag", []string{"-nope"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(testName, tt.args, envFrom(tt.env))
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := errors.Is(err, ErrInvalid); got != tt.invalid {
				t.Errorf("errors.Is(err, ErrInvalid) = %v, want %v (err: %v)", got, tt.invalid, err)
			}
		})
	}
}
