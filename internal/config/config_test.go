package config

import (
	"errors"
	"strings"
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
	envLucarneURL   = "https://lucarne.example"
	envLucarneClust = "env-cluster"
	flagLucarneURL  = "https://flag-lucarne.example"
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
		ConsoleURL: envConsoleURL, LucarneURL: DefaultLucarneURL, LucarneCluster: DefaultLucarneCluster,
		StaleDays: envStaleDays, Interval: envInterval, HiddenFile: envHiddenFile,
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
		ConsoleURL: DefaultConsoleURL, LucarneURL: DefaultLucarneURL, LucarneCluster: DefaultLucarneCluster,
		StaleDays: DefaultStaleDays, Interval: DefaultInterval,
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

func TestParseLucarne(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		currentEnvPrefix + "LUCARNE_URL":     envLucarneURL,
		currentEnvPrefix + "LUCARNE_CLUSTER": envLucarneClust,
	}
	legacy := map[string]string{
		legacyEnvPrefix + "LUCARNE_URL":     envLucarneURL,
		legacyEnvPrefix + "LUCARNE_CLUSTER": envLucarneClust,
	}
	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		wantURL     string
		wantCluster string
	}{
		{"env overrides default", nil, env, envLucarneURL, envLucarneClust},
		{"flag beats env", []string{"-lucarne-url", flagLucarneURL}, env, flagLucarneURL, envLucarneClust},
		{"empty flag turns run links off", []string{"-lucarne-url="}, env, "", envLucarneClust},
		{"no legacy fallback", nil, legacy, DefaultLucarneURL, DefaultLucarneCluster},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(testName, tt.args, envFrom(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if got.LucarneURL != tt.wantURL || got.LucarneCluster != tt.wantCluster {
				t.Errorf("lucarne = %q, %q; want %q, %q", got.LucarneURL, got.LucarneCluster, tt.wantURL, tt.wantCluster)
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
		name     string
		args     []string
		env      map[string]string
		invalid  bool // reported by vigie rather than by the flag package
		wantText string
	}{
		{"bad env int", nil, map[string]string{"VIGIE_STALE_DAYS": "many"}, true, ""},
		{"bad env label", nil, map[string]string{"VIGIE_IN_PROGRESS_LABEL": "x"}, true, ""},
		{"bad env duration", nil, map[string]string{"VIGIE_INTERVAL": "soon"}, true, ""},
		{"bad legacy env int", nil, map[string]string{"ATOP_STALE_DAYS": "many"}, true, "ATOP_STALE_DAYS"},
		{"bad legacy env duration", nil, map[string]string{"ATOP_INTERVAL": "soon"}, true, "ATOP_INTERVAL"},
		{"zero interval flag", []string{"-interval", "0s"}, nil, true, ""},
		{"negative interval env", nil, map[string]string{"VIGIE_INTERVAL": negativeSeconds}, true, ""},
		{"unknown flag", []string{"-nope"}, nil, false, ""},
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
			if tt.wantText != "" && !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantText)
			}
		})
	}
}

const (
	envListen  = "127.0.0.1:9000"
	flagListen = ":9090"
)

func TestParseServe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantListen string
	}{
		{"default listen", []string{ServeCommand}, nil, DefaultListen},
		{"env listen", []string{ServeCommand}, map[string]string{"VIGIE_LISTEN": envListen}, envListen},
		{"flag beats env", []string{ServeCommand, "-listen", flagListen}, map[string]string{"VIGIE_LISTEN": envListen}, flagListen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(testName, tt.args, envFrom(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if !got.Serve || got.Listen != tt.wantListen || got.Once {
				t.Errorf("Serve, Listen, Once = %v, %q, %v; want true, %q, false", got.Serve, got.Listen, got.Once, tt.wantListen)
			}
		})
	}
}

func TestParseServeSharesTheOtherFlags(t *testing.T) {
	t.Parallel()
	got, err := Parse(testName, []string{ServeCommand, "-hidden-file", flagHiddenFile}, envFrom(renamedEnv(currentEnvPrefix)))
	if err != nil {
		t.Fatal(err)
	}
	want := wantEnvConfig()
	want.HiddenFile, want.Serve, want.Listen = flagHiddenFile, true, DefaultListen
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseWithoutServeHasNoListen(t *testing.T) {
	t.Parallel()
	got, err := Parse(testName, nil, envFrom(map[string]string{"VIGIE_LISTEN": envListen}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Serve || got.Listen != "" {
		t.Errorf("Serve, Listen = %v, %q; want false, empty", got.Serve, got.Listen)
	}
}

func TestParseArgumentErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		invalid  bool
		wantText string
	}{
		{"once is not a serve flag", []string{ServeCommand, "-once"}, false, ""},
		{"listen is a serve flag only", []string{"-listen", flagListen}, false, ""},
		{"unknown subcommand", []string{"serv"}, true, `unexpected argument "serv"`},
		{"argument after serve", []string{ServeCommand, "now"}, true, `unexpected argument "now"`},
		{"empty listen", []string{ServeCommand, "-listen", ""}, true, "listen address must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(testName, tt.args, envFrom(nil))
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := errors.Is(err, ErrInvalid); got != tt.invalid {
				t.Errorf("errors.Is(err, ErrInvalid) = %v, want %v (err: %v)", got, tt.invalid, err)
			}
			if tt.wantText != "" && !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantText)
			}
		})
	}
}
