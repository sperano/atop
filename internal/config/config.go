// Package config parses atop's flags, each defaulting to an environment
// variable and then to a built-in value.
package config

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"time"
)

// ErrInvalid marks a bad environment value or setting; flag syntax errors
// are reported by the flag package itself.
var ErrInvalid = errors.New("invalid configuration")

// Built-in defaults, used when neither a flag nor its environment variable is set.
const (
	DefaultNamespace       = "kelos-agents"
	DefaultGitHubOwner     = "sperano"
	DefaultVikunjaURL      = "https://vikunja.spe.quebec/api/v1"
	DefaultTokenNamespace  = "vikunja"
	DefaultTokenSecret     = "agent-claude"
	DefaultTokenKey        = "token"
	DefaultOperator        = "eric"
	DefaultInProgressLabel = 1
	DefaultConsoleURL      = "https://kelos.spe.quebec"
	DefaultStaleDays       = 30
	DefaultInterval        = 30 * time.Second
)

// Config holds every setting atop reads at start-up.
type Config struct {
	Namespace       string
	GitHubOwner     string
	VikunjaURL      string
	TokenNamespace  string
	TokenSecret     string
	TokenKey        string
	Operator        string
	InProgressLabel int
	ConsoleURL      string
	StaleDays       int
	Interval        time.Duration
	Once            bool
}

// Parse reads flags from args, taking each default from getenv when set.
func Parse(name string, args []string, getenv func(string) string) (Config, error) {
	var c Config
	env := envDefaults{getenv: getenv}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&c.Namespace, "namespace", env.str("ATOP_NAMESPACE", DefaultNamespace), "Kelos namespace (ATOP_NAMESPACE)")
	fs.StringVar(&c.GitHubOwner, "github-owner", env.str("ATOP_GITHUB_OWNER", DefaultGitHubOwner), "GitHub owner whose open PRs are listed (ATOP_GITHUB_OWNER)")
	fs.StringVar(&c.VikunjaURL, "vikunja-url", env.str("VIKUNJA_URL", DefaultVikunjaURL), "Vikunja API URL (VIKUNJA_URL)")
	fs.StringVar(&c.TokenNamespace, "token-namespace", env.str("ATOP_TOKEN_NAMESPACE", DefaultTokenNamespace), "namespace of the Vikunja token Secret (ATOP_TOKEN_NAMESPACE)")
	fs.StringVar(&c.TokenSecret, "token-secret", env.str("ATOP_TOKEN_SECRET", DefaultTokenSecret), "name of the Vikunja token Secret (ATOP_TOKEN_SECRET)")
	fs.StringVar(&c.TokenKey, "token-key", env.str("ATOP_TOKEN_KEY", DefaultTokenKey), "key of the Vikunja token in the Secret (ATOP_TOKEN_KEY)")
	fs.StringVar(&c.Operator, "operator", env.str("ATOP_OPERATOR", DefaultOperator), "your Vikunja username (ATOP_OPERATOR)")
	fs.IntVar(&c.InProgressLabel, "in-progress-label", env.integer("ATOP_IN_PROGRESS_LABEL", DefaultInProgressLabel), "id of the Vikunja \"in progress\" label (ATOP_IN_PROGRESS_LABEL)")
	fs.StringVar(&c.ConsoleURL, "console-url", env.str("ATOP_CONSOLE_URL", DefaultConsoleURL), "Kelos console URL (ATOP_CONSOLE_URL)")
	fs.IntVar(&c.StaleDays, "stale-days", env.integer("ATOP_STALE_DAYS", DefaultStaleDays), "days after which an untouched PR or reply is stale (ATOP_STALE_DAYS)")
	fs.DurationVar(&c.Interval, "interval", env.duration("ATOP_INTERVAL", DefaultInterval), "refresh interval (ATOP_INTERVAL)")
	fs.BoolVar(&c.Once, "once", false, "print one plain-text snapshot and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if env.err != nil {
		return Config{}, env.err
	}
	if c.Interval <= 0 {
		return Config{}, fmt.Errorf("%w: interval must be positive, got %s", ErrInvalid, c.Interval)
	}
	return c, nil
}

// envDefaults reads defaults from the environment, keeping the first parse error.
type envDefaults struct {
	getenv func(string) string
	err    error
}

func (e *envDefaults) str(key, fallback string) string {
	if v := e.getenv(key); v != "" {
		return v
	}
	return fallback
}

func (e *envDefaults) integer(key string, fallback int) int {
	v := e.getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail(key, err)
		return fallback
	}
	return n
}

func (e *envDefaults) duration(key string, fallback time.Duration) time.Duration {
	v := e.getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		e.fail(key, err)
		return fallback
	}
	return d
}

func (e *envDefaults) fail(key string, err error) {
	if e.err == nil {
		e.err = fmt.Errorf("%w: %s: %w", ErrInvalid, key, err)
	}
}
