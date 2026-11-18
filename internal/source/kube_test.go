package source

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const testServer = "https://kube.example:6443"

var errBoom = errors.New("boom")

func failingClient(err error) KubeClient {
	return func() (rest.Interface, error) { return nil, err }
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

func TestNewKubeClientWithoutConfig(t *testing.T) {
	t.Setenv(inClusterEnv, "")
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	client := NewKubeClient(func(string) string { return "" })
	_, err := client()
	if !clientcmd.IsEmptyConfig(errors.Unwrap(err)) || !strings.HasPrefix(err.Error(), "kubernetes config: ") {
		t.Fatalf("err = %v, want an empty-config error", err)
	}
	if _, again := client(); again != err {
		t.Errorf("second call err = %v, want the first one, %v", again, err)
	}
}

// podObj is a task pod in phase.
func podObj(name, phase string) kubeObject {
	return kubeObj(podsResource, testNS, obj{"metadata": obj{"name": name, "labels": obj{"kelos.dev/component": "task"}},
		"status": obj{"phase": phase}})
}

func names(pods []pod) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Metadata.Name)
	}
	return out
}

func TestListItemsFollowsContinue(t *testing.T) {
	t.Parallel()
	kube := fakeKube(t, podObj("a", "Running"), podObj("b", "Succeeded"), podObj("c", "Failed"),
		podObj("d", "Pending"), podObj("e", "Running"), podObj("f", "Running"))
	kube.pageSize = 2
	opts := listOptions{LabelSelector: taskPodSelector, FieldSelector: taskPodFields}
	pods, err := listItems[pod](context.Background(), kube.client(t), podsResource, testNS, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(pods), []string{"a", "c", "d", "e", "f"}; !slices.Equal(got, want) {
		t.Errorf("pods = %v, want %v", got, want)
	}
	reqs := kube.requested("/api/v1/namespaces/" + testNS + "/pods")
	var continues []string
	for _, r := range reqs {
		q := r.URL.Query()
		continues = append(continues, q.Get(continueParam))
		if q.Get(limitParam) != strconv.Itoa(listPageSize) || q.Get(labelSelectorParam) != taskPodSelector ||
			q.Get(fieldSelectorParam) != "status.phase!=Succeeded" {
			t.Errorf("query = %v", q)
		}
		if accept := r.Header.Get("Accept"); accept != runtime.ContentTypeJSON {
			t.Errorf("Accept = %q, want %q", accept, runtime.ContentTypeJSON)
		}
	}
	if want := []string{"", "page-2", "page-4"}; !slices.Equal(continues, want) {
		t.Errorf("continue tokens = %q, want %q", continues, want)
	}
}

func TestListItemsPaths(t *testing.T) {
	t.Parallel()
	kube := fakeKube(t, kubeObj(tasksResource, testNS, obj{"metadata": obj{"name": "t"}}))
	tasks, err := listItems[kelosTask](context.Background(), kube.client(t), tasksResource, testNS, listOptions{})
	if err != nil || len(tasks) != 1 || tasks[0].Metadata.Name != "t" {
		t.Fatalf("tasks = %+v err=%v", tasks, err)
	}
	reqs := kube.requested("/apis/kelos.dev/v1alpha2/namespaces/" + testNS + "/tasks")
	if len(reqs) != 1 {
		t.Fatalf("%d requests, want 1", len(reqs))
	}
	q := reqs[0].URL.Query()
	if q.Get(limitParam) != strconv.Itoa(listPageSize) ||
		q.Has(labelSelectorParam) || q.Has(fieldSelectorParam) || q.Has(continueParam) {
		t.Errorf("query = %v, want a limit and no selector or continue token", q)
	}
}

func TestListItemsRepeatedContinue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		tokens   []string // the continue token of each page, the last one served again and again
		want     string
		requests int32
	}{
		{"consecutive", []string{"same"}, `listing pods: the API server repeated continue token "same"`, 2},
		{"cycle", []string{"a", "b", "a"}, `listing pods: the API server repeated continue token "a"`, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client := serverClient(t, func(w http.ResponseWriter, _ *http.Request) {
				n := int(calls.Add(1))
				token := tt.tokens[min(n, len(tt.tokens))-1]
				writeJSON(w, obj{"metadata": obj{"continue": token}, "items": []obj{}})
			})
			_, err := listItems[pod](context.Background(), client, podsResource, testNS, listOptions{})
			if errText(err) != tt.want {
				t.Errorf("err = %v, want %s", err, tt.want)
			}
			if n := calls.Load(); n != tt.requests {
				t.Errorf("requests = %d, want %d", n, tt.requests)
			}
		})
	}
}

func TestListItemsNamespaceSelector(t *testing.T) {
	t.Parallel()
	kube := fakeKube(t, kubeObj(podsResource, testNS, obj{"metadata": obj{"name": "a"}}))
	tests := []struct {
		selector string
		want     []string
	}{
		{"metadata.namespace=" + testNS, []string{"a"}},
		{"metadata.namespace=other", nil},
	}
	for _, tt := range tests {
		pods, err := listItems[pod](context.Background(), kube.client(t), podsResource, testNS,
			listOptions{FieldSelector: tt.selector})
		if err != nil {
			t.Fatal(err)
		}
		if got := names(pods); !slices.Equal(got, tt.want) {
			t.Errorf("%s: pods = %v, want %v", tt.selector, got, tt.want)
		}
	}
}

func TestListItemsErrors(t *testing.T) {
	t.Parallel()
	_, err := listItems[pod](context.Background(), failingClient(errBoom), podsResource, testNS, listOptions{})
	if !errors.Is(err, errBoom) {
		t.Errorf("client error = %v, want %v", err, errBoom)
	}
	kube := fakeKube(t)
	kube.failList("pods")
	_, err = listItems[pod](context.Background(), kube.client(t), podsResource, testNS, listOptions{})
	if !apierrors.IsInternalError(err) || err.Error() != "listing pods: boom" {
		t.Errorf("list error = %v", err)
	}
	_, err = listItems[pod](context.Background(), fakeKube(t).client(t), podsResource, testNS,
		listOptions{FieldSelector: "spec.unknown=x"})
	if !apierrors.IsBadRequest(err) || err.Error() != "listing pods: field label not supported: spec.unknown" {
		t.Errorf("selector error = %v", err)
	}
	truncated := serverClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) })
	_, err = listItems[pod](context.Background(), truncated, podsResource, testNS, listOptions{})
	if !errors.Is(err, io.ErrUnexpectedEOF) || err.Error() != "listing pods: decoding the response: unexpected EOF" {
		t.Errorf("decode error = %v", err)
	}
}

func secretObj(data obj) kubeObject {
	return kubeObj(secretsResource, "vikunja", obj{"metadata": obj{"name": "agent-claude"}, "data": data})
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
			got, err := secretValue(context.Background(), fakeKube(t, secretObj(tt.data)).client(t), secret)
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
	_, err := secretValue(context.Background(), fakeKube(t).client(t), secret)
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
