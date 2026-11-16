package source

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// testKeyBits is the size of the test App key: the smallest Go accepts.
const testKeyBits = 2048

// testRSAKey is generated once: generating a key takes a while.
var testRSAKey = sync.OnceValue(func() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, testKeyBits)
	if err != nil {
		panic(err)
	}
	return k
})

func pemOf(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}

func pkcs1PEM() string { return pemOf(pemRSA, x509.MarshalPKCS1PrivateKey(testRSAKey())) }

func pkcs8PEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pemOf(pemPKCS8, der)
}

func appEnv(id, installation, key string) func(string) string {
	env := map[string]string{
		GitHubAppIDEnv: id, GitHubAppInstallationIDEnv: installation, GitHubAppPrivateKeyEnv: key,
	}
	return func(k string) string { return env[k] }
}

func TestGitHubAppFromEnv(t *testing.T) {
	t.Parallel()
	key := pkcs1PEM()
	oneLine := strings.ReplaceAll(strings.TrimSpace(key), "\n", `\n`)
	tests := []struct {
		name    string
		getenv  func(string) string
		want    *GitHubApp
		wantErr string
	}{
		{"none set", appEnv("", "", ""), nil, ""},
		{"blank counts as unset", appEnv(" ", "\t", "\n"), nil, ""},
		{"all set", appEnv(" 12 ", "34", key), &GitHubApp{ID: 12, InstallationID: 34, PrivateKey: PrivateKey(strings.TrimSpace(key))}, ""},
		{"one-line PEM unescaped", appEnv("12", "34", oneLine), &GitHubApp{ID: 12, InstallationID: 34, PrivateKey: PrivateKey(strings.TrimSpace(key))}, ""},
		{"only the id", appEnv("12", "", ""), nil,
			"github app: VIGIE_GITHUB_APP_INSTALLATION_ID, VIGIE_GITHUB_APP_PRIVATE_KEY not set, but the other GitHub App settings are"},
		{"only the key", appEnv("", "", key), nil,
			"github app: VIGIE_GITHUB_APP_ID, VIGIE_GITHUB_APP_INSTALLATION_ID not set, but the other GitHub App settings are"},
		{"no installation", appEnv("12", "", key), nil,
			"github app: VIGIE_GITHUB_APP_INSTALLATION_ID not set, but the other GitHub App settings are"},
		{"no key", appEnv("12", "34", ""), nil,
			"github app: VIGIE_GITHUB_APP_PRIVATE_KEY not set, but the other GitHub App settings are"},
		{"id not a number", appEnv("twelve", "34", key), nil, "github app: VIGIE_GITHUB_APP_ID must be a positive integer"},
		{"installation zero", appEnv("12", "0", key), nil, "github app: VIGIE_GITHUB_APP_INSTALLATION_ID must be a positive integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GitHubAppFromEnv(tt.getenv)
			if errText(err) != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("app = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRSAKey(t *testing.T) {
	t.Parallel()
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{"PKCS#1", pkcs1PEM(), ""},
		{"PKCS#8 RSA", pkcs8PEM(t, testRSAKey()), ""},
		{"not PEM", "s3cret-not-a-pem", "github app: VIGIE_GITHUB_APP_PRIVATE_KEY is not PEM"},
		{"corrupt PKCS#1", pemOf(pemRSA, []byte("s3cret")), "github app: VIGIE_GITHUB_APP_PRIVATE_KEY is not a valid PKCS#1 RSA key"},
		{"corrupt PKCS#8", pemOf(pemPKCS8, []byte("s3cret")), "github app: VIGIE_GITHUB_APP_PRIVATE_KEY is not a valid PKCS#8 key"},
		{"PKCS#8 ECDSA", pkcs8PEM(t, ecKey), "github app: VIGIE_GITHUB_APP_PRIVATE_KEY is not an RSA key"},
		{"other PEM type", pemOf("CERTIFICATE", []byte("s3cret")),
			`github app: VIGIE_GITHUB_APP_PRIVATE_KEY: PEM type "CERTIFICATE" is not "RSA PRIVATE KEY" or "PRIVATE KEY"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRSAKey(PrivateKey(tt.key))
			if errText(err) != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if err == nil && !got.Equal(testRSAKey()) {
				t.Error("parsed key differs from the encoded one")
			}
		})
	}
}

func TestPrivateKeyIsRedacted(t *testing.T) {
	t.Parallel()
	app := GitHubApp{ID: 1, InstallationID: 2, PrivateKey: PrivateKey(pkcs1PEM())}
	var logged bytes.Buffer
	slog.New(slog.NewTextHandler(&logged, nil)).Info("app", "key", app.PrivateKey)
	encoded, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"%v":   fmt.Sprintf("%v", app),
		"%+v":  fmt.Sprintf("%+v", app),
		"%#v":  fmt.Sprintf("%#v", app),
		"%q":   fmt.Sprintf("%q", app.PrivateKey),
		"slog": logged.String(),
		"json": string(encoded),
	}
	for name, out := range outputs {
		if strings.Contains(out, "PRIVATE KEY") || !strings.Contains(out, redacted) {
			t.Errorf("%s printed %q, want %s and no key", name, out, redacted)
		}
	}
}
