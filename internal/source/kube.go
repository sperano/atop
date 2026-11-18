package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeTimeout bounds every Kubernetes API read: a get, or a list with all
// its pages.
const KubeTimeout = 30 * time.Second

// listPageSize is the most objects one list request returns. A Kelos task
// pod is about 67 KB of JSON, so a page stays a few MB however many pods
// Kelos keeps.
const listPageSize = 50

const (
	inClusterEnv = "KUBERNETES_SERVICE_HOST"
	kelosGroup   = "kelos.dev"
	kelosVersion = "v1alpha2"
	coreVersion  = "v1"

	coreAPIPath    = "/api"
	groupsAPIPath  = "/apis"
	namespacesPath = "namespaces"

	limitParam         = "limit"
	continueParam      = "continue"
	labelSelectorParam = "labelSelector"
	fieldSelectorParam = "fieldSelector"
)

var (
	sessionsResource = schema.GroupVersionResource{Group: kelosGroup, Version: kelosVersion, Resource: "sessions"}
	tasksResource    = schema.GroupVersionResource{Group: kelosGroup, Version: kelosVersion, Resource: "tasks"}
	podsResource     = schema.GroupVersionResource{Version: coreVersion, Resource: "pods"}
	secretsResource  = schema.GroupVersionResource{Version: coreVersion, Resource: "secrets"}
)

// KubeClient returns a REST client for the cluster's API, or why there is
// none.
type KubeClient func() (rest.Interface, error)

// NewKubeClient connects on first use, once: in-cluster when
// KUBERNETES_SERVICE_HOST is set, else through the kubeconfig loading rules
// (KUBECONFIG, current context), like kubectl. Without a usable
// configuration only the sources that need the cluster fail.
func NewKubeClient(getenv func(string) string) KubeClient {
	return sync.OnceValues(func() (rest.Interface, error) {
		cfg, err := restConfig(getenv)
		if err != nil {
			return nil, fmt.Errorf("kubernetes config: %w", err)
		}
		return newRESTClient(cfg)
	})
}

func restConfig(getenv func(string) string) (*rest.Config, error) {
	if getenv(inClusterEnv) != "" {
		return rest.InClusterConfig()
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// newRESTClient is a client for raw GETs of any API path. It asks for JSON
// only, which encoding/json decodes straight into the typed structs; the
// dynamic client's serializer only decodes the Status of an error response.
func newRESTClient(in *rest.Config) (rest.Interface, error) {
	cfg := dynamic.ConfigFor(in)
	cfg.AcceptContentTypes = runtime.ContentTypeJSON
	cfg.GroupVersion = nil
	cfg.Timeout = KubeTimeout
	return rest.UnversionedRESTClientFor(cfg)
}

// listOptions selects the objects of a list.
type listOptions struct {
	LabelSelector string
	FieldSelector string
}

// listPage is one page of a list: its items and the token of the next page,
// empty on the last one.
type listPage[T any] struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []T `json:"items"`
}

// listItems lists one resource's objects in a namespace, listPageSize at a
// time, each page streamed into T, which keeps only the fields vigie reads:
// the memory a list takes is one page of JSON and the kept fields, not the
// whole list decoded. KubeTimeout bounds all the pages together.
func listItems[T any](ctx context.Context, client KubeClient, gvr schema.GroupVersionResource, namespace string, opts listOptions) ([]T, error) {
	c, err := client()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, KubeTimeout)
	defer cancel()
	var items []T
	next := ""
	for {
		var page listPage[T]
		if err := getJSON(ctx, listRequest(c, gvr, namespace, opts, next), &page); err != nil {
			return nil, fmt.Errorf("listing %s: %w", gvr.GroupResource(), err)
		}
		items = append(items, page.Items...)
		if page.Metadata.Continue == "" {
			return items, nil
		}
		if page.Metadata.Continue == next {
			return nil, fmt.Errorf("listing %s: the API server repeated continue token %q", gvr.GroupResource(), next)
		}
		next = page.Metadata.Continue
	}
}

// listRequest asks for the page of the list that the continue token next
// names, or for its first page when next is empty.
func listRequest(c rest.Interface, gvr schema.GroupVersionResource, namespace string, opts listOptions, next string) *rest.Request {
	req := c.Get().AbsPath(collectionPath(gvr, namespace)...).
		Param(limitParam, strconv.Itoa(listPageSize))
	if opts.LabelSelector != "" {
		req = req.Param(labelSelectorParam, opts.LabelSelector)
	}
	if opts.FieldSelector != "" {
		req = req.Param(fieldSelectorParam, opts.FieldSelector)
	}
	if next != "" {
		req = req.Param(continueParam, next)
	}
	return req
}

// collectionPath is the API path of a resource's objects in a namespace:
// /api/v1/... for the core group, /apis/<group>/<version>/... for the
// others.
func collectionPath(gvr schema.GroupVersionResource, namespace string) []string {
	prefix := []string{coreAPIPath, gvr.Version}
	if gvr.Group != "" {
		prefix = []string{groupsAPIPath, gvr.Group, gvr.Version}
	}
	return append(prefix, namespacesPath, namespace, gvr.Resource)
}

// getJSON runs req and streams its JSON response into v. The error of a
// failed request is the API's Status (apierrors reads it).
func getJSON(ctx context.Context, req *rest.Request, v any) error {
	body, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer body.Close()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		return fmt.Errorf("decoding the response: %w", err)
	}
	return nil
}

// secretValue reads one key of a Secret, decoded and trimmed. Errors never
// carry the value.
func secretValue(ctx context.Context, client KubeClient, s TokenSecret) (string, error) {
	c, err := client()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, KubeTimeout)
	defer cancel()
	var secret struct {
		Data map[string]string `json:"data"`
	}
	path := append(collectionPath(secretsResource, s.Namespace), s.Name)
	if err := getJSON(ctx, c.Get().AbsPath(path...), &secret); err != nil {
		return "", fmt.Errorf("reading secret %s/%s: %w", s.Namespace, s.Name, err)
	}
	encoded, found := secret.Data[s.Key]
	if !found {
		return "", fmt.Errorf("secret %s/%s has no key %q", s.Namespace, s.Name, s.Key)
	}
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding secret %s/%s key %q: %w", s.Namespace, s.Name, s.Key, err)
	}
	return strings.TrimSpace(string(value)), nil
}
