package config

import (
	"errors"
	"testing"
	"time"
)

const (
	testName        = "atop"
	envOwner        = "env-owner"
	flagOwner       = "flag-owner"
	envStaleDays    = 7
	envLabel        = 4
	envInterval     = 5 * time.Second
	flagInterval    = 2 * time.Minute
	negativeSeconds = "-5s"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
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
		Operator: DefaultOperator, InProgressLabel: DefaultInProgressLabel, ConsoleURL: DefaultConsoleURL,
		StaleDays: DefaultStaleDays, Interval: DefaultInterval,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseEnvAndFlags(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"ATOP_GITHUB_OWNER":      envOwner,
		"ATOP_STALE_DAYS":        "7",
		"ATOP_IN_PROGRESS_LABEL": "4",
		"ATOP_INTERVAL":          "5s",
	}
	tests := []struct {
		name  string
		args  []string
		check func(Config) bool
	}{
		{"env overrides default", nil, func(c Config) bool {
			return c.GitHubOwner == envOwner && c.StaleDays == envStaleDays &&
				c.InProgressLabel == envLabel && c.Interval == envInterval
		}},
		{"flag beats env", []string{"-github-owner", flagOwner, "-interval", "2m"}, func(c Config) bool {
			return c.GitHubOwner == flagOwner && c.Interval == flagInterval && c.StaleDays == envStaleDays
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

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		invalid bool // reported by atop rather than by the flag package
	}{
		{"bad env int", nil, map[string]string{"ATOP_STALE_DAYS": "many"}, true},
		{"bad env label", nil, map[string]string{"ATOP_IN_PROGRESS_LABEL": "x"}, true},
		{"bad env duration", nil, map[string]string{"ATOP_INTERVAL": "soon"}, true},
		{"zero interval flag", []string{"-interval", "0s"}, nil, true},
		{"negative interval env", nil, map[string]string{"ATOP_INTERVAL": negativeSeconds}, true},
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
