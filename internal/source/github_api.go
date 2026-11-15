package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// GitHubGraphQLURL is GitHub's GraphQL endpoint.
	GitHubGraphQLURL = "https://api.github.com/graphql"
	// maxGitHubResponse caps the GraphQL response read into memory.
	maxGitHubResponse = 16 << 20
	// maxGitHubErrorBody caps how much of a failed response is quoted.
	maxGitHubErrorBody = 1 << 10
	userAgent          = "vigie"
)

// GitHub token environment variables, in lookup order.
const (
	ghTokenEnv     = "GH_TOKEN"
	githubTokenEnv = "GITHUB_TOKEN"
)

var errNoGitHubToken = errors.New("no GitHub token: set GH_TOKEN or GITHUB_TOKEN, or log in with gh")

// TokenFunc returns an API token.
type TokenFunc func(ctx context.Context) (string, error)

// GitHubToken looks the token up on every call: GH_TOKEN, else GITHUB_TOKEN,
// else the output of `gh auth token` (a laptop logged in with gh).
func GitHubToken(getenv func(string) string, run Runner) TokenFunc {
	return func(ctx context.Context) (string, error) {
		for _, key := range []string{ghTokenEnv, githubTokenEnv} {
			if token := strings.TrimSpace(getenv(key)); token != "" {
				return token, nil
			}
		}
		out, err := run(ctx, "gh", "auth", "token")
		if err != nil {
			return "", fmt.Errorf("%w: %w", errNoGitHubToken, err)
		}
		token := strings.TrimSpace(string(out))
		if token == "" {
			return "", errNoGitHubToken
		}
		return token, nil
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// graphQL POSTs a query and decodes its data into out. A non-200 status or
// a response carrying errors fails, quoting GitHub's message; the token is
// never part of an error.
func (g GitHub) graphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	token, err := g.Token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := g.Client.Do(req)
	if err != nil {
		return fmt.Errorf("github graphql: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github graphql: %s%s", resp.Status, errorMessage(resp.Body))
	}
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxGitHubResponse)).Decode(&res); err != nil {
		return fmt.Errorf("github graphql: decoding response: %w", err)
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("github graphql: %s", joinMessages(res.Errors))
	}
	if err := json.Unmarshal(res.Data, out); err != nil {
		return fmt.Errorf("github graphql: decoding data: %w", err)
	}
	return nil
}

func (g GitHub) endpoint() string {
	if g.APIURL != "" {
		return g.APIURL
	}
	return GitHubGraphQLURL
}

// errorMessage is ": <message>" from a failed response's JSON body, else
// empty.
func errorMessage(body io.Reader) string {
	var res graphQLError
	if err := json.NewDecoder(io.LimitReader(body, maxGitHubErrorBody)).Decode(&res); err != nil || res.Message == "" {
		return ""
	}
	return ": " + res.Message
}

func joinMessages(errs []graphQLError) string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}
