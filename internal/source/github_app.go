package source

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// GitHub App environment variables: with all three set, vigie mints its own
// installation tokens instead of reading GH_TOKEN and the like.
const (
	// GitHubAppIDEnv holds the App's numeric ID.
	GitHubAppIDEnv = "VIGIE_GITHUB_APP_ID"
	// GitHubAppInstallationIDEnv holds the numeric ID of the App's
	// installation on the owner.
	GitHubAppInstallationIDEnv = "VIGIE_GITHUB_APP_INSTALLATION_ID"
	// GitHubAppPrivateKeyEnv holds the App's private key, the PEM itself
	// (PKCS#1 "RSA PRIVATE KEY", which GitHub generates, or PKCS#8
	// "PRIVATE KEY"). Literal \n sequences stand for newlines, for a
	// variable that cannot hold several lines.
	GitHubAppPrivateKeyEnv = "VIGIE_GITHUB_APP_PRIVATE_KEY"
)

const (
	// pemRSA and pemPKCS8 are the PEM block types of the accepted keys.
	pemRSA   = "RSA PRIVATE KEY"
	pemPKCS8 = "PRIVATE KEY"
	// escapedNewline is how a one-line variable spells a PEM's newline.
	escapedNewline = `\n`
	// redacted is how a private key prints.
	redacted = "[redacted]"
)

// PrivateKey is a GitHub App's private key, as PEM. It prints as
// [redacted] through fmt, log/slog and text encodings.
type PrivateKey string

// String implements fmt.Stringer.
func (PrivateKey) String() string { return redacted }

// GoString implements fmt.GoStringer, for %#v.
func (PrivateKey) GoString() string { return redacted }

// LogValue implements slog.LogValuer.
func (PrivateKey) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalText implements encoding.TextMarshaler.
func (PrivateKey) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// GitHubApp identifies a GitHub App installation. Its installation tokens,
// unlike a fine-grained personal access token, can read check runs on
// private repositories.
type GitHubApp struct {
	ID, InstallationID int64
	PrivateKey         PrivateKey
}

// GitHubAppFromEnv reads the App settings through getenv. It returns nil
// when none is set, and an error naming each missing or malformed variable,
// never its value, when some are.
func GitHubAppFromEnv(getenv func(string) string) (*GitHubApp, error) {
	id := strings.TrimSpace(getenv(GitHubAppIDEnv))
	installation := strings.TrimSpace(getenv(GitHubAppInstallationIDEnv))
	key := strings.TrimSpace(getenv(GitHubAppPrivateKeyEnv))
	if id == "" && installation == "" && key == "" {
		return nil, nil
	}
	var missing []string
	for _, v := range []struct{ name, value string }{
		{GitHubAppIDEnv, id}, {GitHubAppInstallationIDEnv, installation}, {GitHubAppPrivateKeyEnv, key},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("github app: %s not set, but the other GitHub App settings are",
			strings.Join(missing, ", "))
	}
	app := &GitHubApp{PrivateKey: PrivateKey(unescapeNewlines(key))}
	var err error
	if app.ID, err = positiveID(GitHubAppIDEnv, id); err != nil {
		return nil, err
	}
	if app.InstallationID, err = positiveID(GitHubAppInstallationIDEnv, installation); err != nil {
		return nil, err
	}
	return app, nil
}

// unescapeNewlines turns the literal \n sequences of a one-line PEM into
// newlines; a PEM that already has lines is left alone.
func unescapeNewlines(pemText string) string {
	if strings.Contains(pemText, "\n") {
		return pemText
	}
	return strings.ReplaceAll(pemText, escapedNewline, "\n")
}

// positiveID parses the value of variable name as a positive integer.
func positiveID(name, value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("github app: %s must be a positive integer", name)
	}
	return n, nil
}

// parseRSAKey decodes a PKCS#1 or PKCS#8 PEM RSA private key. Its errors
// say what is wrong, never repeat the key.
func parseRSAKey(key PrivateKey) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return nil, fmt.Errorf("github app: %s is not PEM", GitHubAppPrivateKeyEnv)
	}
	switch block.Type {
	case pemRSA:
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("github app: %s is not a valid PKCS#1 RSA key", GitHubAppPrivateKeyEnv)
		}
		return k, nil
	case pemPKCS8:
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("github app: %s is not a valid PKCS#8 key", GitHubAppPrivateKeyEnv)
		}
		if k, ok := parsed.(*rsa.PrivateKey); ok {
			return k, nil
		}
		return nil, fmt.Errorf("github app: %s is not an RSA key", GitHubAppPrivateKeyEnv)
	}
	return nil, fmt.Errorf("github app: %s: PEM type %q is not %q or %q",
		GitHubAppPrivateKeyEnv, block.Type, pemRSA, pemPKCS8)
}
