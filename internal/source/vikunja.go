package source

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// HTTPTimeout bounds every Vikunja request.
	HTTPTimeout = 15 * time.Second
	// VikunjaPageSize is the page size used when listing tasks.
	VikunjaPageSize = 50
	// commentFetchWorkers caps concurrent newest-comment lookups.
	commentFetchWorkers = 8
	// commentsPerPage is one because only the newest comment is needed.
	commentsPerPage  = 1
	firstPage        = 1
	totalPagesHeader = "X-Pagination-Total-Pages"
	newestFirst      = "desc"
	apiSuffix        = "/api/v1"
	replyFromPrefix  = "reply from "
	unknownAuthor    = "?"
	// StatePROpen marks a task whose PR is open (Kelos labels it so).
	StatePROpen = "PR open"
)

// VikunjaWebBase is the web base of a Vikunja API URL.
func VikunjaWebBase(apiURL string) string {
	return strings.TrimSuffix(strings.TrimSuffix(apiURL, "/"), apiSuffix)
}

// VikunjaTaskURL is the web page of a Vikunja task.
func VikunjaTaskURL(webBase, id string) string {
	return strings.TrimSuffix(webBase, "/") + "/tasks/" + url.PathEscape(id)
}

type vikunjaTask struct {
	ID           int              `json:"id"`
	Title        string           `json:"title"`
	Updated      string           `json:"updated"`
	Comments     []vikunjaComment `json:"comments,omitempty"`
	CommentCount *int             `json:"comment_count,omitempty"`
	Labels       []struct {
		Title string `json:"title"`
	} `json:"labels"`
	Assignees []struct {
		Username string `json:"username"`
	} `json:"assignees"`
}

type vikunjaComment struct {
	Comment string `json:"comment"`
	Created string `json:"created"`
	Author  struct {
		Username string `json:"username"`
	} `json:"author"`
}

type commentCacheEntry struct {
	updated string
	count   int
	last    *vikunjaComment
}

type vikunjaCommentCache map[int]commentCacheEntry

// TokenSecret names the Kubernetes Secret holding the Vikunja token.
type TokenSecret struct {
	Namespace, Name, Key string
}

// Vikunja lists in-progress tasks and the newest comment on each.
type Vikunja struct {
	APIURL          string
	Token           string // when empty, read from Secret
	Secret          TokenSecret
	Operator        string
	InProgressLabel int
	PROpenLabel     string // title of the label Kelos puts on a task whose PR is open
	StaleDays       int
	Client          *http.Client
	Run             Runner
	Now             func() time.Time
	commentCache    vikunjaCommentCache
}

// Fetch lists the in-progress tasks with their newest comment.
func (v *Vikunja) Fetch(ctx context.Context) ([]Row, error) {
	token, err := v.token(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := v.inProgress(ctx, token)
	if err != nil {
		return nil, err
	}
	comments, err := v.comments(ctx, token, tasks)
	if err != nil {
		return nil, err
	}
	stale := Staleness{Days: v.StaleDays, Now: v.Now()}
	web := VikunjaWebBase(v.APIURL)
	rows := make([]Row, 0, len(tasks))
	for i, t := range tasks {
		rows = append(rows, v.row(t, comments[i], stale, web))
	}
	return rows, nil
}

func (v *Vikunja) comments(ctx context.Context, token string, tasks []vikunjaTask) ([]*vikunjaComment, error) {
	comments := make([]*vikunjaComment, len(tasks))
	lookups := make([]int, 0, len(tasks))
	for i, task := range tasks {
		if last, ok := v.cachedComment(task); ok {
			comments[i] = last
			continue
		}
		if last, complete := embeddedNewest(task); complete {
			comments[i] = last
			continue
		}
		lookups = append(lookups, i)
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(commentFetchWorkers)
	for _, i := range lookups {
		g.Go(func() error {
			c, err := v.newestComment(gctx, token, tasks[i].ID)
			comments[i] = c
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	for i, t := range tasks {
		v.cacheComment(t, comments[i])
	}
	v.pruneCommentCache(tasks)
	return comments, nil
}

func (v *Vikunja) token(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(v.Token); token != "" {
		return token, nil
	}
	jsonpath := fmt.Sprintf("jsonpath={.data.%s}", v.Secret.Key)
	out, err := v.Run(ctx, "kubectl", "-n", v.Secret.Namespace, "get", "secret", v.Secret.Name, "-o", jsonpath)
	if err != nil {
		return "", err
	}
	token, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("decoding Vikunja token: %w", err)
	}
	return strings.TrimSpace(string(token)), nil
}

// get fetches a Vikunja API path into v and returns the response headers.
func (v *Vikunja) get(ctx context.Context, token, path string, params url.Values, out any) (http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, HTTPTimeout)
	defer cancel()
	u := strings.TrimSuffix(v.APIURL, "/") + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vikunja GET %s: %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("vikunja GET %s: %w", path, err)
	}
	return resp.Header, nil
}

func (v *Vikunja) inProgress(ctx context.Context, token string) ([]vikunjaTask, error) {
	params := url.Values{
		"filter":   {fmt.Sprintf("labels in %d && done = false", v.InProgressLabel)},
		"per_page": {strconv.Itoa(VikunjaPageSize)},
		"expand":   {"comments", "comment_count"},
	}
	var tasks []vikunjaTask
	for page := firstPage; ; page++ {
		params.Set("page", strconv.Itoa(page))
		var batch []vikunjaTask
		header, err := v.get(ctx, token, "/tasks", params, &batch)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, batch...)
		if lastPage(header, page, len(batch)) {
			return tasks, nil
		}
	}
}

// lastPage reports whether page is the last one: by the total-pages header
// when present, since the server may cap pages below VikunjaPageSize, else
// by a short or empty batch.
func lastPage(header http.Header, page, batchLen int) bool {
	if total, err := strconv.Atoi(header.Get(totalPagesHeader)); err == nil {
		return page >= total
	}
	return batchLen < VikunjaPageSize
}

// newestComment returns a task's newest comment, or nil when it has none.
func (v *Vikunja) newestComment(ctx context.Context, token string, taskID int) (*vikunjaComment, error) {
	path := fmt.Sprintf("/tasks/%d/comments", taskID)
	params := url.Values{
		"order_by": {newestFirst},
		"per_page": {strconv.Itoa(commentsPerPage)},
		"page":     {strconv.Itoa(firstPage)},
	}
	var page []vikunjaComment
	_, err := v.get(ctx, token, path, params, &page)
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		return nil, nil
	}
	return &page[0], nil
}

// embeddedNewest uses expanded comments only when comment_count proves the
// per-task slice is complete. Vikunja caps expanded comments across the page.
func embeddedNewest(task vikunjaTask) (*vikunjaComment, bool) {
	if task.CommentCount == nil || *task.CommentCount < 0 {
		return nil, false
	}
	if len(task.Comments) != *task.CommentCount {
		return nil, false
	}
	if len(task.Comments) == 0 {
		return nil, true
	}
	return &task.Comments[len(task.Comments)-1], true
}

func (v *Vikunja) cachedComment(task vikunjaTask) (*vikunjaComment, bool) {
	if task.Updated == "" || !knownCommentCount(task) || v.commentCache == nil {
		return nil, false
	}
	entry, ok := v.commentCache[task.ID]
	if !ok || entry.updated != task.Updated || entry.count != *task.CommentCount {
		return nil, false
	}
	return entry.last, true
}

func (v *Vikunja) cacheComment(task vikunjaTask, last *vikunjaComment) {
	if task.Updated == "" || !knownCommentCount(task) {
		return
	}
	if v.commentCache == nil {
		v.commentCache = make(vikunjaCommentCache)
	}
	v.commentCache[task.ID] = commentCacheEntry{updated: task.Updated, count: *task.CommentCount, last: last}
}

func knownCommentCount(task vikunjaTask) bool {
	return task.CommentCount != nil && *task.CommentCount >= 0
}

func (v *Vikunja) pruneCommentCache(tasks []vikunjaTask) {
	active := make(map[int]struct{}, len(tasks))
	for _, task := range tasks {
		active[task.ID] = struct{}{}
	}
	for id := range v.commentCache {
		if _, ok := active[id]; !ok {
			delete(v.commentCache, id)
		}
	}
}

// row maps a task to its state: PR open while its PR is open, a reply
// when someone else spoke last (needing you unless stale), else idle. A
// running Kelos Task later overrides this (Enrich).
func (v *Vikunja) row(t vikunjaTask, last *vikunjaComment, stale Staleness, web string) Row {
	row := Row{
		Source: SourceVikunja,
		Origin: OriginVikunja,
		Name:   "#" + strconv.Itoa(t.ID),
		Since:  parseTime(t.Updated),
		Detail: taskTitle(t),
		Target: Target{URL: VikunjaTaskURL(web, strconv.Itoa(t.ID))},
		Ref:    VikunjaRef(strconv.Itoa(t.ID)),
	}
	author := unknownAuthor
	if last != nil {
		row.Since = parseTime(last.Created)
		author = cmp.Or(last.Author.Username, unknownAuthor)
	}
	switch {
	case hasLabel(t, v.PROpenLabel):
		row.State = StatePROpen
	case last == nil || author == v.Operator:
		row.State = StateIdle
	case stale.IsStale(row.Since):
		row.State = StateStale
		row.Detail = replyFromPrefix + author + ": " + row.Detail + " — " + snippet(last.Comment)
	default:
		row.State, row.NeedsYou = replyFromPrefix+author, true
		row.Detail += " — " + snippet(last.Comment)
	}
	return row
}

// taskTitle prefixes the title with the agent user the task is assigned to.
func taskTitle(t vikunjaTask) string {
	if len(t.Assignees) > 0 && t.Assignees[0].Username != "" {
		return "[" + t.Assignees[0].Username + "] " + t.Title
	}
	return t.Title
}

func hasLabel(t vikunjaTask, title string) bool {
	if title == "" {
		return false
	}
	for _, l := range t.Labels {
		if l.Title == title {
			return true
		}
	}
	return false
}
