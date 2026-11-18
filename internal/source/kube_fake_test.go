package source

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// continuePrefix starts the fake API's continue tokens; the rest is the
// offset of the next page.
const continuePrefix = "page-"

// The fields a list can be selected by: the metadata ones, and for pods
// also their phase (a subset of what the API server knows).
var (
	metadataFields = []string{"metadata.name", "metadata.namespace"}
	podFields      = append(slices.Clone(metadataFields), "status.phase")
)

// kubeObject is an object the fake API holds.
type kubeObject struct {
	gvr       schema.GroupVersionResource
	namespace string
	content   obj
}

func kubeObj(gvr schema.GroupVersionResource, namespace string, content obj) kubeObject {
	return kubeObject{gvr: gvr, namespace: namespace, content: content}
}

// fakeAPI is a fake Kubernetes API server. Like the real one, it lists a
// namespace's objects of a resource selected by label and field selector
// (an unknown field is a 400), at most limit per page (fewer when pageSize
// is lower) with a continue token for the rest, and gets one by name. It
// records every request.
type fakeAPI struct {
	srv      *httptest.Server
	objs     map[string][]obj // by collection path
	pageSize int              // zero: limit alone sizes a page
	failing  map[string]bool  // resources whose requests fail with "boom"

	mu       sync.Mutex
	requests []*http.Request
}

func fakeKube(t *testing.T, objs ...kubeObject) *fakeAPI {
	t.Helper()
	f := &fakeAPI{objs: map[string][]obj{}, failing: map[string]bool{}}
	for _, o := range objs {
		path := fakeCollectionPath(o.gvr, o.namespace)
		f.objs[path] = append(f.objs[path], withNamespace(o.content, o.namespace))
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// fakeCollectionPath spells out the API paths, independently of
// collectionPath.
func fakeCollectionPath(gvr schema.GroupVersionResource, namespace string) string {
	if gvr.Group == "" {
		return fmt.Sprintf("/api/%s/namespaces/%s/%s", gvr.Version, namespace, gvr.Resource)
	}
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s", gvr.Group, gvr.Version, namespace, gvr.Resource)
}

// failList makes every request for resource fail with an internal error.
func (f *fakeAPI) failList(resource string) { f.failing[resource] = true }

// client is a vigie client of the fake, built like the real one.
func (f *fakeAPI) client(t *testing.T) KubeClient { return clientOf(t, f.srv.URL) }

func clientOf(t *testing.T, host string) KubeClient {
	t.Helper()
	c, err := newRESTClient(&rest.Config{Host: host})
	if err != nil {
		t.Fatal(err)
	}
	return func() (rest.Interface, error) { return c, nil }
}

// serverClient is a vigie client of an API server that handler answers.
func serverClient(t *testing.T, handler http.HandlerFunc) KubeClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return clientOf(t, srv.URL)
}

// requested lists the requests received for one collection path.
func (f *fakeAPI) requested(path string) []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*http.Request
	for _, r := range f.requests {
		if r.URL.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// Segments of a collection path: api/<version>/namespaces/<ns>/<resource>
// and apis/<group>/<version>/namespaces/<ns>/<resource>.
const (
	coreCollectionSegments  = 5
	groupCollectionSegments = 6
)

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Clone(r.Context()))
	f.mu.Unlock()
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	collection, name := r.URL.Path, ""
	if !isCollection(segments) {
		collection, name = path.Dir(r.URL.Path), path.Base(r.URL.Path)
	}
	resource := path.Base(collection)
	switch {
	case f.failing[resource]:
		writeStatus(w, http.StatusInternalServerError, "InternalError", "boom")
	case name == "":
		f.list(w, r.URL.Query(), resource, f.objs[collection])
	default:
		f.get(w, f.objs[collection], name)
	}
}

func isCollection(segments []string) bool {
	n := len(segments)
	return (segments[0] == "api" && n == coreCollectionSegments) ||
		(segments[0] == "apis" && n == groupCollectionSegments)
}

func (f *fakeAPI) get(w http.ResponseWriter, items []obj, name string) {
	for _, o := range items {
		if objName(o) == name {
			writeJSON(w, o)
			return
		}
	}
	writeStatus(w, http.StatusNotFound, "NotFound", fmt.Sprintf("%q not found", name))
}

func (f *fakeAPI) list(w http.ResponseWriter, q url.Values, resource string, items []obj) {
	selected, err := selectObjs(q, resource, items)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "BadRequest", err.Error())
		return
	}
	offset, size, err := f.page(q, len(selected))
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "BadRequest", err.Error())
		return
	}
	end := min(offset+size, len(selected))
	meta := obj{}
	if end < len(selected) {
		meta["continue"] = continuePrefix + strconv.Itoa(end)
	}
	writeJSON(w, obj{"kind": "List", "apiVersion": "v1", "metadata": meta, "items": selected[offset:end]})
}

// page is where the requested page starts and how many objects it holds.
func (f *fakeAPI) page(q url.Values, total int) (offset, size int, err error) {
	size = total
	if limit := q.Get(limitParam); limit != "" {
		if size, err = strconv.Atoi(limit); err != nil {
			return 0, 0, err
		}
	}
	if f.pageSize > 0 {
		size = min(size, f.pageSize)
	}
	if token := q.Get(continueParam); token != "" {
		n, ok := strings.CutPrefix(token, continuePrefix)
		if offset, err = strconv.Atoi(n); !ok || err != nil {
			return 0, 0, fmt.Errorf("invalid continue token %q", token)
		}
	}
	return offset, size, nil
}

func selectObjs(q url.Values, resource string, items []obj) ([]obj, error) {
	labelSel, err := labels.Parse(q.Get(labelSelectorParam))
	if err != nil {
		return nil, err
	}
	fieldSel, err := fields.ParseSelector(q.Get(fieldSelectorParam))
	if err != nil {
		return nil, err
	}
	known := metadataFields
	if resource == podsResource.Resource {
		known = podFields
	}
	for _, req := range fieldSel.Requirements() {
		if !slices.Contains(known, req.Field) {
			return nil, fmt.Errorf("field label not supported: %s", req.Field)
		}
	}
	selected := []obj{}
	for _, o := range items {
		if labelSel.Matches(objLabels(o)) && fieldSel.Matches(objFields(o)) {
			selected = append(selected, o)
		}
	}
	return selected, nil
}

func objMeta(o obj) obj {
	meta, _ := o["metadata"].(obj)
	return meta
}

func objName(o obj) string {
	name, _ := objMeta(o)["name"].(string)
	return name
}

func objLabels(o obj) labels.Set {
	set := labels.Set{}
	raw, _ := objMeta(o)["labels"].(obj)
	for k, v := range raw {
		set[k], _ = v.(string)
	}
	return set
}

// withNamespace is a copy of content whose metadata names its namespace, as
// the API server returns it.
func withNamespace(content obj, namespace string) obj {
	meta := maps.Clone(objMeta(content))
	if meta == nil {
		meta = obj{}
	}
	meta["namespace"] = namespace
	out := maps.Clone(content)
	if out == nil {
		out = obj{}
	}
	out["metadata"] = meta
	return out
}

func objFields(o obj) fields.Set {
	status, _ := o["status"].(obj)
	phase, _ := status["phase"].(string)
	namespace, _ := objMeta(o)["namespace"].(string)
	return fields.Set{"metadata.name": objName(o), "metadata.namespace": namespace, "status.phase": phase}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeStatus(w http.ResponseWriter, code int, reason, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(obj{"kind": "Status", "apiVersion": "v1", "status": "Failure",
		"reason": reason, "message": message, "code": code})
}
