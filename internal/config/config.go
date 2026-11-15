// Package config parses vigie's flags, each defaulting to an environment
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
	DefaultPROpenLabel     = "pr open"
	DefaultConsoleURL      = "https://kelos.spe.quebec"
	DefaultStaleDays       = 30
	DefaultInterval        = 2 * time.Minute
	currentEnvPrefix       = "VIGIE_"
	legacyEnvPrefix        = "ATOP_"
)

// Config holds every setting vigie reads at start-up.
type Config struct {
	Namespace       string
	GitHubOwner     string
	VikunjaURL      string
	TokenNamespace  string
	TokenSecret     string
	TokenKey        string
	Operator        string
	InProgressLabel int
	PROpenLabel     string
	ConsoleURL      string
	StaleDays       int
	Interval        time.Duration
	// HiddenFile is where hidden rows are kept; empty means
	// hidden.DefaultFile.
	HiddenFile string
	Once       bool
}

// Parse reads flags from args, taking each default from getenv when set.
func Parse(name string, args []string, getenv func(string) string) (Config, error) {
	var c Config
	env := envDefaults{getenv: getenv}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&c.Namespace, "namespace", env.renamedStr("NAMESPACE", DefaultNamespace), "Kelos namespace (VIGIE_NAMESPACE)")
	fs.StringVar(&c.GitHubOwner, "github-owner", env.renamedStr("GITHUB_OWNER", DefaultGitHubOwner), "GitHub owner whose open PRs are listed (VIGIE_GITHUB_OWNER)")
	fs.StringVar(&c.VikunjaURL, "vikunja-url", env.str("VIKUNJA_URL", "", DefaultVikunjaURL), "Vikunja API URL (VIKUNJA_URL)")
	fs.StringVar(&c.TokenNamespace, "token-namespace", env.renamedStr("TOKEN_NAMESPACE", DefaultTokenNamespace), "namespace of the Vikunja token Secret (VIGIE_TOKEN_NAMESPACE)")
	fs.StringVar(&c.TokenSecret, "token-secret", env.renamedStr("TOKEN_SECRET", DefaultTokenSecret), "name of the Vikunja token Secret (VIGIE_TOKEN_SECRET)")
	fs.StringVar(&c.TokenKey, "token-key", env.renamedStr("TOKEN_KEY", DefaultTokenKey), "key of the Vikunja token in the Secret (VIGIE_TOKEN_KEY)")
	fs.StringVar(&c.Operator, "operator", env.renamedStr("OPERATOR", DefaultOperator), "your Vikunja username (VIGIE_OPERATOR)")
	fs.IntVar(&c.InProgressLabel, "in-progress-label", env.renamedInteger("IN_PROGRESS_LABEL", DefaultInProgressLabel), "id of the Vikunja \"in progress\" label (VIGIE_IN_PROGRESS_LABEL)")
	fs.StringVar(&c.PROpenLabel, "pr-open-label", env.renamedStr("PR_OPEN_LABEL", DefaultPROpenLabel), "title of the Vikunja label on tasks whose PR is open (VIGIE_PR_OPEN_LABEL)")
	fs.StringVar(&c.ConsoleURL, "console-url", env.renamedStr("CONSOLE_URL", DefaultConsoleURL), "Kelos console URL (VIGIE_CONSOLE_URL)")
	fs.IntVar(&c.StaleDays, "stale-days", env.renamedInteger("STALE_DAYS", DefaultStaleDays), "days after which an untouched PR or reply is stale (VIGIE_STALE_DAYS)")
	fs.DurationVar(&c.Interval, "interval", env.renamedDuration("INTERVAL", DefaultInterval), "refresh interval (VIGIE_INTERVAL)")
	fs.StringVar(&c.HiddenFile, "hidden-file", env.renamedStr("HIDDEN_FILE", ""), "file of the rows hidden with h (VIGIE_HIDDEN_FILE; default <user config dir>/vigie/hidden.json)")
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

func (e *envDefaults) renamedStr(suffix, fallback string) string {
	return e.str(currentEnvPrefix+suffix, legacyEnvPrefix+suffix, fallback)
}

func (e *envDefaults) renamedInteger(suffix string, fallback int) int {
	key, v := e.value(currentEnvPrefix+suffix, legacyEnvPrefix+suffix)
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

func (e *envDefaults) renamedDuration(suffix string, fallback time.Duration) time.Duration {
	key, v := e.value(currentEnvPrefix+suffix, legacyEnvPrefix+suffix)
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

func (e *envDefaults) str(primary, legacy, fallback string) string {
	_, v := e.value(primary, legacy)
	if v == "" {
		return fallback
	}
	return v
}

func (e *envDefaults) value(primary, legacy string) (string, string) {
	if v := e.getenv(primary); v != "" {
		return primary, v
	}
	if legacy != "" {
		if v := e.getenv(legacy); v != "" {
			return legacy, v
		}
	}
	return "", ""
}

func (e *envDefaults) fail(key string, err error) {
	if e.err == nil {
		e.err = fmt.Errorf("%w: %s: %w", ErrInvalid, key, err)
	}
}
