package source

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

const (
	// GitHubAPIURL is GitHub's REST API, where installation tokens are
	// minted.
	GitHubAPIURL = "https://api.github.com"
	// githubMediaType and githubAPIVersion are the REST API's recommended
	// Accept and X-GitHub-Api-Version headers.
	githubMediaType  = "application/vnd.github+json"
	githubAPIVersion = "2022-11-28"

	// jwtBackdate is how far before now a JWT's iat is set, against clock
	// drift between this host and GitHub.
	jwtBackdate = 60 * time.Second
	// jwtLifetime is how long after now a JWT expires; GitHub's maximum is
	// ten minutes.
	jwtLifetime = 9 * time.Minute
	// jwtHeader is the fixed header of the JWTs: RS256.
	jwtHeader = `{"alg":"RS256","typ":"JWT"}`
	// tokenRefreshMargin is how long before an installation token expires
	// a new one is minted.
	tokenRefreshMargin = 5 * time.Minute
	// maxMintResponse bounds the body read from a mint's response.
	maxMintResponse = 64 << 10
	// permissionRead is the only access level vigie asks for or accepts.
	permissionRead = "read"
)

// readOnlyPermissions are the permissions asked of every installation
// token: the App can write, so a leaked token must not.
var readOnlyPermissions = map[string]string{
	"checks":        permissionRead,
	"contents":      permissionRead,
	"metadata":      permissionRead,
	"pull_requests": permissionRead,
}

// GitHubTokenSource picks the GitHub credential once, at start-up: the App
// installation's tokens when the VIGIE_GITHUB_APP_* variables are all set,
// else GitHubToken's lookup. Some of them set, or a malformed one, is an
// error.
func GitHubTokenSource(getenv func(string) string, run Runner, client *http.Client,
	now func() time.Time) (TokenFunc, error) {
	app, err := GitHubAppFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return GitHubToken(getenv, run), nil
	}
	tokens, err := newInstallationTokens(GitHubAPIURL, *app, client, now)
	if err != nil {
		return nil, err
	}
	return tokens.current, nil
}

// installationTokens mints read-only installation tokens for one App
// installation and caches the current one. It is safe for concurrent use;
// one mint runs at a time.
type installationTokens struct {
	url    string
	path   string
	appID  string
	key    *rsa.PrivateKey
	client *http.Client
	now    func() time.Time

	mu       sync.Mutex
	token    string
	expireAt time.Time
}

// newInstallationTokens returns the minter of app's tokens from the REST
// API at apiURL (no trailing slash).
func newInstallationTokens(apiURL string, app GitHubApp, client *http.Client,
	now func() time.Time) (*installationTokens, error) {
	key, err := parseRSAKey(app.PrivateKey)
	if err != nil {
		return nil, err
	}
	path := "/app/installations/" + strconv.FormatInt(app.InstallationID, 10) + "/access_tokens"
	return &installationTokens{
		url: apiURL + path, path: path, appID: strconv.FormatInt(app.ID, 10),
		key: key, client: client, now: now,
	}, nil
}

// current returns the cached installation token, minted again first when
// it is missing or expires within tokenRefreshMargin.
func (m *installationTokens) current(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && m.now().Add(tokenRefreshMargin).Before(m.expireAt) {
		return m.token, nil
	}
	token, expireAt, err := m.mint(ctx)
	if err != nil {
		return "", err
	}
	m.token, m.expireAt = token, expireAt
	return token, nil
}

type mintRequest struct {
	Permissions map[string]string `json:"permissions"`
}

type mintResponse struct {
	Token       string            `json:"token"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Permissions map[string]string `json:"permissions"`
}

// mint exchanges a fresh JWT for a read-only installation token. Errors
// name the path and status, never a credential.
func (m *installationTokens) mint(ctx context.Context) (string, time.Time, error) {
	fail := func(format string, args ...any) (string, time.Time, error) {
		return "", time.Time{}, fmt.Errorf("github app: POST %s: "+format, append([]any{m.path}, args...)...)
	}
	jwt, err := m.signJWT()
	if err != nil {
		return fail("signing: %w", err)
	}
	body, err := json.Marshal(mintRequest{Permissions: readOnlyPermissions})
	if err != nil {
		return fail("%w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(body))
	if err != nil {
		return fail("%w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", githubMediaType)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", userAgent)
	resp, err := m.client.Do(req)
	if err != nil {
		return fail("%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fail("%s%s", resp.Status, errorMessage(resp.Body))
	}
	var out mintResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMintResponse)).Decode(&out); err != nil {
		return fail("decoding the response")
	}
	if out.Token == "" || out.ExpiresAt.IsZero() {
		return fail("the response has no token or expiry")
	}
	if name, ok := writablePermission(out.Permissions); ok {
		return fail("the token grants %s %q, not %q", name, out.Permissions[name], permissionRead)
	}
	return out.Token, out.ExpiresAt, nil
}

// writablePermission names a granted permission above read, in name
// order, if any: GitHub ignoring the asked permissions must not hand vigie
// a token that can write.
func writablePermission(granted map[string]string) (string, bool) {
	names := make([]string, 0, len(granted))
	for name, level := range granted {
		if level != permissionRead {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	return slices.Min(names), true
}

// jwtClaims are the claims GitHub requires of an App's JWT.
type jwtClaims struct {
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
}

// signJWT returns an RS256 JWT identifying the App, valid for jwtLifetime.
func (m *installationTokens) signJWT() (string, error) {
	now := m.now()
	claims, err := json.Marshal(jwtClaims{
		IssuedAt:  now.Add(-jwtBackdate).Unix(),
		ExpiresAt: now.Add(jwtLifetime).Unix(),
		Issuer:    m.appID,
	})
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString([]byte(jwtHeader)) + "." + enc.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}
