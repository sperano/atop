package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeTimeout bounds every Kubernetes API request.
const KubeTimeout = 30 * time.Second

const (
	inClusterEnv = "KUBERNETES_SERVICE_HOST"
	kelosGroup   = "kelos.dev"
	kelosVersion = "v1alpha2"
	coreVersion  = "v1"
	secretData   = "data"
)

var (
	sessionsResource = schema.GroupVersionResource{Group: kelosGroup, Version: kelosVersion, Resource: "sessions"}
	tasksResource    = schema.GroupVersionResource{Group: kelosGroup, Version: kelosVersion, Resource: "tasks"}
	podsResource     = schema.GroupVersionResource{Version: coreVersion, Resource: "pods"}
	secretsResource  = schema.GroupVersionResource{Version: coreVersion, Resource: "secrets"}
)

// DynamicClient returns a client for the cluster, or why there is none.
type DynamicClient func() (dynamic.Interface, error)

// NewDynamicClient connects on first use, once: in-cluster when
// KUBERNETES_SERVICE_HOST is set, else through the kubeconfig loading rules
// (KUBECONFIG, current context), like kubectl. Without a usable
// configuration only the sources that need the cluster fail.
func NewDynamicClient(getenv func(string) string) DynamicClient {
	return sync.OnceValues(func() (dynamic.Interface, error) {
		cfg, err := restConfig(getenv)
		if err != nil {
			return nil, fmt.Errorf("kubernetes config: %w", err)
		}
		cfg.Timeout = KubeTimeout
		return dynamic.NewForConfig(cfg)
	})
}

func restConfig(getenv func(string) string) (*rest.Config, error) {
	if getenv(inClusterEnv) != "" {
		return rest.InClusterConfig()
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// listJSON lists one resource's objects in a namespace and decodes the
// list, as `kubectl get -o json` prints it, into v.
func listJSON(ctx context.Context, client DynamicClient, gvr schema.GroupVersionResource, namespace, selector string, v any) error {
	c, err := client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, KubeTimeout)
	defer cancel()
	list, err := c.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("listing %s: %w", gvr.GroupResource(), err)
	}
	data, err := list.MarshalJSON()
	if err != nil {
		return fmt.Errorf("encoding %s: %w", gvr.GroupResource(), err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decoding %s: %w", gvr.GroupResource(), err)
	}
	return nil
}

// secretValue reads one key of a Secret, decoded and trimmed. Errors never
// carry the value.
func secretValue(ctx context.Context, client DynamicClient, s TokenSecret) (string, error) {
	c, err := client()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, KubeTimeout)
	defer cancel()
	obj, err := c.Resource(secretsResource).Namespace(s.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("reading secret %s/%s: %w", s.Namespace, s.Name, err)
	}
	encoded, found, err := unstructured.NestedString(obj.Object, secretData, s.Key)
	if err != nil || !found {
		return "", fmt.Errorf("secret %s/%s has no key %q", s.Namespace, s.Name, s.Key)
	}
	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding secret %s/%s key %q: %w", s.Namespace, s.Name, s.Key, err)
	}
	return strings.TrimSpace(string(value)), nil
}
