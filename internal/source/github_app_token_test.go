package source

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testAppID          = 12
	testInstallationID = 34
	testMintPath       = "/app/installations/34/access_tokens"
	// testTokenLifetime is how long GitHub's installation tokens last.
	testTokenLifetime = time.Hour
)

// mintedRequest is what the fake GitHub was sent by one mint.
type mintedRequest struct {
	method, path, auth, accept, version string
	body                                []byte
}

// mintServer stands for GitHub's access-token endpoint. Unless status is
// set it answers 201 with token "ghs_<n>" for the nth mint, expiring
// testTokenLifetime after now(), and granting perms (the asked ones when
// nil).
type mintServer struct {
	now    func() time.Time
	status int
	body   string
	perms  map[string]string

	mu       sync.Mutex
	requests []mintedRequest
}

func (s *mintServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, mintedRequest{
		method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"),
		accept: r.Header.Get("Accept"), version: r.Header.Get("X-GitHub-Api-Version"), body: body,
	})
	n, status, perms := len(s.requests), s.status, s.perms
	s.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(s.body))
		return
	}
	if perms == nil {
		perms = readOnlyPermissions
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token": fmt.Sprintf("ghs_%d", n), "expires_at": s.now().Add(testTokenLifetime), "permissions": perms,
	})
}

func (s *mintServer) sent() []mintedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mintedRequest(nil), s.requests...)
}

// clock is a settable test clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newMinter(t *testing.T, s *mintServer, now func() time.Time) *installationTokens {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	app := GitHubApp{ID: testAppID, InstallationID: testInstallationID, PrivateKey: PrivateKey(pkcs1PEM())}
	m, err := newInstallationTokens(srv.URL, app, srv.Client(), now)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// verifyJWT checks the RS256 signature of jwt with the test key and
// returns its decoded header and claims.
func verifyJWT(t *testing.T, jwt string) (header map[string]string, claims jwtClaims) {
	t.Helper()
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}
	enc := base64.RawURLEncoding
	sig, err := enc.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&testRSAKey().PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("JWT signature: %v", err)
	}
	for i, out := range []any{&header, &claims} {
		raw, err := enc.DecodeString(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
	}
	return header, claims
}

func TestMintRequest(t *testing.T) {
	t.Parallel()
	s := &mintServer{now: fixedNow}
	token, err := newMinter(t, s, fixedNow).current(context.Background())
	if err != nil || token != "ghs_1" {
		t.Fatalf("token = %q err=%v, want ghs_1", token, err)
	}
	reqs := s.sent()
	if len(reqs) != 1 {
		t.Fatalf("sent %d requests, want 1", len(reqs))
	}
	req := reqs[0]
	if req.method != http.MethodPost || req.path != testMintPath {
		t.Errorf("request = %s %s, want POST %s", req.method, req.path, testMintPath)
	}
	if req.accept != githubMediaType || req.version != githubAPIVersion {
		t.Errorf("Accept = %q, X-GitHub-Api-Version = %q", req.accept, req.version)
	}
	var body struct {
		Permissions map[string]string `json:"permissions"`
	}
	if err := json.Unmarshal(req.body, &body); err != nil {
		t.Fatalf("body %s: %v", req.body, err)
	}
	want := map[string]string{"checks": "read", "contents": "read", "metadata": "read", "pull_requests": "read"}
	if !maps.Equal(body.Permissions, want) {
		t.Errorf("permissions = %v, want %v", body.Permissions, want)
	}
	jwt, ok := strings.CutPrefix(req.auth, "Bearer ")
	if !ok {
		t.Fatalf("Authorization = %q, want a Bearer JWT", req.auth)
	}
	header, claims := verifyJWT(t, jwt)
	if header["alg"] != "RS256" || header["typ"] != "JWT" {
		t.Errorf("JWT header = %v", header)
	}
	wantClaims := jwtClaims{
		IssuedAt:  testNow.Add(-time.Minute).Unix(),
		ExpiresAt: testNow.Add(9 * time.Minute).Unix(),
		Issuer:    "12",
	}
	if claims != wantClaims {
		t.Errorf("claims = %+v, want %+v", claims, wantClaims)
	}
}

func TestInstallationTokenRenewal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		elapsed   time.Duration
		wantToken string
	}{
		{"fresh token reused", time.Minute, "ghs_1"},
		{"reused just outside the margin", testTokenLifetime - 5*time.Minute - time.Second, "ghs_1"},
		{"renewed at the margin", testTokenLifetime - 5*time.Minute, "ghs_2"},
		{"renewed once expired", testTokenLifetime + time.Minute, "ghs_2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &clock{t: testNow}
			s := &mintServer{now: c.now}
			m := newMinter(t, s, c.now)
			if got, err := m.current(context.Background()); err != nil || got != "ghs_1" {
				t.Fatalf("first token = %q err=%v", got, err)
			}
			c.advance(tt.elapsed)
			got, err := m.current(context.Background())
			if err != nil || got != tt.wantToken {
				t.Errorf("token = %q err=%v, want %q", got, err, tt.wantToken)
			}
		})
	}
}

func TestInstallationTokenMintsOnceForConcurrentCallers(t *testing.T) {
	t.Parallel()
	const callers = 8
	s := &mintServer{now: fixedNow}
	m := newMinter(t, s, fixedNow)
	var wg sync.WaitGroup
	tokens := make([]string, callers)
	for i := range callers {
		wg.Go(func() { tokens[i], _ = m.current(context.Background()) })
	}
	wg.Wait()
	for i, token := range tokens {
		if token != "ghs_1" {
			t.Errorf("caller %d got %q, want ghs_1", i, token)
		}
	}
	if n := len(s.sent()); n != 1 {
		t.Errorf("minted %d times, want 1", n)
	}
}

func TestMintErrors(t *testing.T) {
	t.Parallel()
	prefix := "github app: POST " + testMintPath + ": "
	tests := []struct {
		name    string
		status  int
		body    string
		perms   map[string]string
		wantErr string
	}{
		{"refused, quoting GitHub", http.StatusUnauthorized, `{"message":"A JSON web token could not be decoded"}`, nil,
			"401 Unauthorized: A JSON web token could not be decoded"},
		{"refused, no message", http.StatusUnprocessableEntity, "nope", nil, "422 Unprocessable Entity"},
		{"200 is not a mint", http.StatusOK, `{"token":"ghs_x"}`, nil, "200 OK"},
		{"not JSON", http.StatusCreated, "ghs_leaked", nil, "decoding the response"},
		{"no token", http.StatusCreated, `{"expires_at":"2026-10-05T13:00:00Z"}`, nil, "the response has no token or expiry"},
		{"no expiry", http.StatusCreated, `{"token":"ghs_leaked"}`, nil, "the response has no token or expiry"},
		{"write permission granted", 0, "", map[string]string{"checks": "read", "contents": "write", "issues": "admin"},
			`the token grants contents "write", not "read"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &mintServer{now: fixedNow, status: tt.status, body: tt.body, perms: tt.perms}
			token, err := newMinter(t, s, fixedNow).current(context.Background())
			if token != "" || errText(err) != prefix+tt.wantErr {
				t.Errorf("token = %q err=%v, want %q", token, err, prefix+tt.wantErr)
			}
		})
	}
}

func TestMintRetriesAfterAFailure(t *testing.T) {
	t.Parallel()
	s := &mintServer{now: fixedNow, status: http.StatusBadGateway}
	m := newMinter(t, s, fixedNow)
	if _, err := m.current(context.Background()); err == nil {
		t.Fatal("first mint succeeded, want 502")
	}
	s.mu.Lock()
	s.status = 0
	s.mu.Unlock()
	if got, err := m.current(context.Background()); err != nil || got != "ghs_2" {
		t.Errorf("token = %q err=%v, want ghs_2", got, err)
	}
}

// githubAPITransport sends every request to the handler instead of the
// network, recording the URLs asked for.
type githubAPITransport struct {
	handler http.Handler
	mu      sync.Mutex
	urls    []string
}

func (g *githubAPITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	g.mu.Lock()
	g.urls = append(g.urls, r.URL.String())
	g.mu.Unlock()
	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, r)
	return rec.Result(), nil
}

func TestGitHubTokenSource(t *testing.T) {
	t.Parallel()
	key := pkcs1PEM()
	tests := []struct {
		name      string
		env       map[string]string
		want      string
		wantErr   string
		wantMints int
	}{
		{"no App: GH_TOKEN", map[string]string{ghTokenEnv: "gh", githubTokenEnv: "github"}, "gh", "", 0},
		{"no App: GITHUB_TOKEN", map[string]string{githubTokenEnv: "github"}, "github", "", 0},
		{"no App: gh auth token", nil, "gho_cli", "", 0},
		{"App beats GH_TOKEN", map[string]string{
			GitHubAppIDEnv: "12", GitHubAppInstallationIDEnv: "34", GitHubAppPrivateKeyEnv: key, ghTokenEnv: "gh",
		}, "ghs_1", "", 1},
		{"half-set App", map[string]string{GitHubAppIDEnv: "12", ghTokenEnv: "gh"}, "",
			"github app: VIGIE_GITHUB_APP_INSTALLATION_ID, VIGIE_GITHUB_APP_PRIVATE_KEY not set, but the other GitHub App settings are", 0},
		{"App with a bad key", map[string]string{
			GitHubAppIDEnv: "12", GitHubAppInstallationIDEnv: "34", GitHubAppPrivateKeyEnv: "s3cret",
		}, "", "github app: VIGIE_GITHUB_APP_PRIVATE_KEY is not PEM", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			transport := &githubAPITransport{handler: &mintServer{now: fixedNow}}
			runner := &fakeRunner{replies: map[string]string{"gh auth token": "gho_cli\n"}}
			getenv := func(k string) string { return tt.env[k] }
			tokens, err := GitHubTokenSource(getenv, runner.run, &http.Client{Transport: transport}, fixedNow)
			if errText(err) != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			got, err := tokens(context.Background())
			if err != nil || got != tt.want {
				t.Errorf("token = %q err=%v, want %q", got, err, tt.want)
			}
			if len(transport.urls) != tt.wantMints || (tt.wantMints > 0 && transport.urls[0] != GitHubAPIURL+testMintPath) {
				t.Errorf("requests = %q, want %d to %s", transport.urls, tt.wantMints, GitHubAPIURL+testMintPath)
			}
		})
	}
}
