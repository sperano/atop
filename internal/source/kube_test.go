package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kelosAPIVersion = kelosGroup + "/" + kelosVersion
	testServer      = "https://kube.example:6443"
)

var errBoom = errors.New("boom")

// fakeKube is a fake dynamic client holding objs, which knows the list
// kinds of every resource the sources read.
func fakeKube(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	listKinds := map[schema.GroupVersionResource]string{
		sessionsResource: "SessionList",
		tasksResource:    "TaskList",
		podsResource:     "PodList",
		secretsResource:  "SecretList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
}

func clientOf(c dynamic.Interface) DynamicClient {
	return func() (dynamic.Interface, error) { return c, nil }
}

func failingClient(err error) DynamicClient {
	return func() (dynamic.Interface, error) { return nil, err }
}

// kubeObj turns a test object into an unstructured one of the given kind in
// namespace ns. The JSON round trip gives it the value types unstructured
// objects hold.
func kubeObj(t *testing.T, apiVersion, kind, ns string, o obj) *unstructured.Unstructured {
	t.Helper()
	var content map[string]any
	if err := json.Unmarshal(mustJSON(t, o), &content); err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: content}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(ns)
	return u
}

// failList makes every list of resource fail with err.
func failList(c *dynamicfake.FakeDynamicClient, resource string, err error) {
	c.PrependReactor("list", resource, func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

func TestRestConfigKubeconfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	kubeconfig := `apiVersion: v1
kind: Config
clusters: [{name: c, cluster: {server: "` + testServer + `"}}]
users: [{name: u, user: {token: t}}]
contexts: [{name: x, context: {cluster: c, user: u}}]
current-context: x
`
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	cfg, err := restConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != testServer {
		t.Errorf("host = %q, want %q", cfg.Host, testServer)
	}
}

func TestRestConfigInCluster(t *testing.T) {
	// rest.InClusterConfig reads the real environment; with it empty, the
	// in-cluster branch reports that it is not in a cluster.
	t.Setenv(inClusterEnv, "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	getenv := func(key string) string {
		if key == inClusterEnv {
			return "10.0.0.1"
		}
		return ""
	}
	if _, err := restConfig(getenv); !errors.Is(err, rest.ErrNotInCluster) {
		t.Errorf("err = %v, want rest.ErrNotInCluster", err)
	}
}

func TestNewDynamicClientWithoutConfig(t *testing.T) {
	t.Setenv(inClusterEnv, "")
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	client := NewDynamicClient(func(string) string { return "" })
	_, err := client()
	if !clientcmd.IsEmptyConfig(errors.Unwrap(err)) || !strings.HasPrefix(err.Error(), "kubernetes config: ") {
		t.Fatalf("err = %v, want an empty-config error", err)
	}
	if _, again := client(); again != err {
		t.Errorf("second call err = %v, want the first one, %v", again, err)
	}
}

func TestListJSONErrors(t *testing.T) {
	t.Parallel()
	var list itemList[pod]
	err := listJSON(context.Background(), failingClient(errBoom), podsResource, testNS, "", &list)
	if !errors.Is(err, errBoom) {
		t.Errorf("client error = %v, want %v", err, errBoom)
	}
	kube := fakeKube()
	failList(kube, "pods", errBoom)
	err = listJSON(context.Background(), clientOf(kube), podsResource, testNS, "", &list)
	if !errors.Is(err, errBoom) || err.Error() != "listing pods: boom" {
		t.Errorf("list error = %v", err)
	}
}

func secretObj(t *testing.T, data obj) *unstructured.Unstructured {
	return kubeObj(t, coreVersion, "Secret", "vikunja", obj{"metadata": obj{"name": "agent-claude"}, "data": data})
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestSecretValue(t *testing.T) {
	t.Parallel()
	secret := TokenSecret{Namespace: "vikunja", Name: "agent-claude", Key: "token"}
	tests := []struct {
		name    string
		data    obj
		want    string
		wantErr string
	}{
		{"decoded and trimmed", obj{"token": b64(testToken + "\n")}, testToken, ""},
		{"missing key", obj{"other": b64(testToken)}, "", `secret vikunja/agent-claude has no key "token"`},
		{"bad base64", obj{"token": "!!!"}, "", `decoding secret vikunja/agent-claude key "token": illegal base64 data at input byte 0`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := secretValue(context.Background(), clientOf(fakeKube(secretObj(t, tt.data))), secret)
			if got != tt.want || errText(err) != tt.wantErr {
				t.Errorf("got %q err=%v, want %q err=%q", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestSecretValueReadErrors(t *testing.T) {
	t.Parallel()
	secret := TokenSecret{Namespace: "vikunja", Name: "agent-claude", Key: "token"}
	if _, err := secretValue(context.Background(), failingClient(errBoom), secret); !errors.Is(err, errBoom) {
		t.Errorf("client error = %v, want %v", err, errBoom)
	}
	_, err := secretValue(context.Background(), clientOf(fakeKube()), secret)
	if !apierrors.IsNotFound(err) || !strings.HasPrefix(errText(err), "reading secret vikunja/agent-claude: ") {
		t.Errorf("missing secret error = %v", err)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
