package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const testGitHubToken = "ghs_s3cret"

// graphQLServer answers every request with status and body, and records
// what it was sent.
type graphQLServer struct {
	status int
	body   string

	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	method, path, auth, contentType string
	body                            graphQLRequest
}

func (s *graphQLServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body graphQLRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.mu.Lock()
	s.requests = append(s.requests, recordedRequest{
		method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"),
		contentType: r.Header.Get("Content-Type"), body: body,
	})
	s.mu.Unlock()
	w.WriteHeader(s.status)
	_, _ = w.Write([]byte(s.body))
}

func staticToken(token string) TokenFunc {
	return func(context.Context) (string, error) { return token, nil }
}

func newGitHub(t *testing.T, s *graphQLServer) GitHub {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return GitHub{
		Owner: "sperano", StaleDays: testStaleDays, APIURL: srv.URL + "/graphql",
		Client: srv.Client(), Token: staticToken(testGitHubToken), Now: fixedNow,
	}
}

func TestGitHubFetch(t *testing.T) {
	t.Parallel()
	resp := obj{"data": obj{"search": obj{"nodes": []any{
		prObj(prOpts{}),
		obj{}, // a non-PR search hit decodes as an empty object
		nil,
	}}}}
	s := &graphQLServer{status: http.StatusOK, body: string(mustJSON(t, resp))}
	rows, err := newGitHub(t, s).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "cocovm#7" {
		t.Fatalf("rows = %+v", rows)
	}
	if len(s.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(s.requests))
	}
	req := s.requests[0]
	if req.method != http.MethodPost || req.path != "/graphql" || req.contentType != "application/json" {
		t.Errorf("request = %s %s (%s)", req.method, req.path, req.contentType)
	}
	if req.auth != "bearer "+testGitHubToken {
		t.Errorf("authorization = %q", req.auth)
	}
	wantVars := map[string]any{"q": "is:pr is:open owner:sperano archived:false", "n": float64(PRSearchLimit)}
	if req.body.Query != prQuery || len(req.body.Variables) != len(wantVars) ||
		req.body.Variables["q"] != wantVars["q"] || req.body.Variables["n"] != wantVars["n"] {
		t.Errorf("body = %+v", req.body)
	}
}

func TestGitHubFetchErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"http status with GitHub's message", http.StatusUnauthorized, `{"message":"Bad credentials"}`,
			"github graphql: 401 Unauthorized: Bad credentials"},
		{"http status without a JSON body", http.StatusBadGateway, "<html>bad gateway</html>",
			"github graphql: 502 Bad Gateway"},
		{"graphql errors", http.StatusOK, `{"data":null,"errors":[{"message":"rate limited"},{"message":"timeout"}]}`,
			"github graphql: rate limited; timeout"},
		{"graphql errors beside partial data", http.StatusOK, `{"data":{"search":{"nodes":[]}},"errors":[{"message":"partial"}]}`,
			"github graphql: partial"},
		{"malformed response", http.StatusOK, "not json",
			"github graphql: decoding response: invalid character 'o' in literal null (expecting 'u')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := newGitHub(t, &graphQLServer{status: tt.status, body: tt.body}).Fetch(context.Background())
			if errText(err) != tt.wantErr {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitHubFetchTokenError(t *testing.T) {
	t.Parallel()
	s := &graphQLServer{status: http.StatusOK, body: `{}`}
	g := newGitHub(t, s)
	g.Token = func(context.Context) (string, error) { return "", errNoGitHubToken }
	if _, err := g.Fetch(context.Background()); !errors.Is(err, errNoGitHubToken) {
		t.Errorf("err = %v, want %v", err, errNoGitHubToken)
	}
	if len(s.requests) != 0 {
		t.Errorf("requests = %d, want none without a token", len(s.requests))
	}
}

func TestGitHubFetchTransportErrorHidesToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	g := GitHub{APIURL: srv.URL, Client: &http.Client{}, Token: staticToken(testGitHubToken), Now: fixedNow}
	_, err := g.Fetch(context.Background())
	if !strings.HasPrefix(errText(err), "github graphql: ") || strings.Contains(err.Error(), testGitHubToken) {
		t.Errorf("err = %v", err)
	}
}

func TestGitHubEndpointDefault(t *testing.T) {
	t.Parallel()
	if got := (GitHub{}).endpoint(); got != GitHubGraphQLURL {
		t.Errorf("endpoint = %q, want %q", got, GitHubGraphQLURL)
	}
}

func TestGitHubToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		env       map[string]string
		runner    *fakeRunner
		want      string
		wantErr   error
		wantCalls int
	}{
		{"GH_TOKEN first", map[string]string{ghTokenEnv: "gh", githubTokenEnv: "github"}, &fakeRunner{}, "gh", nil, 0},
		{"GITHUB_TOKEN when GH_TOKEN is blank", map[string]string{ghTokenEnv: " ", githubTokenEnv: "github\n"}, &fakeRunner{}, "github", nil, 0},
		{"gh auth token last, trimmed", nil, &fakeRunner{replies: map[string]string{"gh auth token": "gho_x\n"}}, "gho_x", nil, 1},
		{"gh failure", nil, &fakeRunner{errs: map[string]error{"gh auth token": errBoom}}, "", errBoom, 1},
		{"gh prints nothing", nil, &fakeRunner{replies: map[string]string{"gh auth token": "\n"}}, "", errNoGitHubToken, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(key string) string { return tt.env[key] }
			got, err := GitHubToken(getenv, tt.runner.run)(context.Background())
			if got != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("got %q err=%v, want %q err=%v", got, err, tt.want, tt.wantErr)
			}
			if err != nil && !errors.Is(err, errNoGitHubToken) {
				t.Errorf("err = %v, want it to wrap %v", err, errNoGitHubToken)
			}
			if len(tt.runner.calls) != tt.wantCalls || (tt.wantCalls == 1 && tt.runner.calls[0] != "gh auth token") {
				t.Errorf("calls = %q", tt.runner.calls)
			}
		})
	}
}
