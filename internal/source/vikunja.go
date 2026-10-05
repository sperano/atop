package source

import (
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
	// commentsPerPage is one so that the page count is the comment count.
	commentsPerPage     = 1
	firstPage           = 1
	totalPagesHeader    = "X-Pagination-Total-Pages"
	apiSuffix           = "/api/v1"
	stateInProgress     = "in progress"
	stateWaitingOnAgent = "waiting on agent"
	replyFromPrefix     = "reply from "
	unknownAuthor       = "?"
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
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Updated string `json:"updated"`
}

type vikunjaComment struct {
	Comment string `json:"comment"`
	Created string `json:"created"`
	Author  struct {
		Username string `json:"username"`
	} `json:"author"`
}

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
	StaleDays       int
	Client          *http.Client
	Run             Runner
	Now             func() time.Time
}

// Fetch lists the in-progress tasks with their newest comment.
func (v Vikunja) Fetch(ctx context.Context) ([]Row, error) {
	token, err := v.token(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := v.inProgress(ctx, token)
	if err != nil {
		return nil, err
	}
	comments := make([]*vikunjaComment, len(tasks))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(commentFetchWorkers)
	for i, t := range tasks {
		g.Go(func() error {
			c, err := v.newestComment(gctx, token, t.ID)
			comments[i] = c
			return err
		})
	}
	if err := g.Wait(); err != nil {
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

func (v Vikunja) token(ctx context.Context) (string, error) {
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
func (v Vikunja) get(ctx context.Context, token, path string, params url.Values, out any) (http.Header, error) {
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

func (v Vikunja) inProgress(ctx context.Context, token string) ([]vikunjaTask, error) {
	params := url.Values{
		"filter":   {fmt.Sprintf("labels in %d && done = false", v.InProgressLabel)},
		"per_page": {strconv.Itoa(VikunjaPageSize)},
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
// Comments are paginated oldest first, so with one per page the page count
// is the comment count and the last page holds the newest.
func (v Vikunja) newestComment(ctx context.Context, token string, taskID int) (*vikunjaComment, error) {
	path := fmt.Sprintf("/tasks/%d/comments", taskID)
	params := url.Values{"per_page": {strconv.Itoa(commentsPerPage)}, "page": {strconv.Itoa(firstPage)}}
	var page []vikunjaComment
	header, err := v.get(ctx, token, path, params, &page)
	if err != nil {
		return nil, err
	}
	if total, _ := strconv.Atoi(header.Get(totalPagesHeader)); total > 1 {
		params.Set("page", strconv.Itoa(total))
		page = nil
		if _, err := v.get(ctx, token, path, params, &page); err != nil {
			return nil, err
		}
	}
	if len(page) == 0 {
		return nil, nil
	}
	return &page[len(page)-1], nil
}

func (v Vikunja) row(t vikunjaTask, last *vikunjaComment, stale Staleness, web string) Row {
	row := Row{
		Source: SourceVikunja,
		Name:   "#" + strconv.Itoa(t.ID),
		Detail: t.Title,
		Target: Target{URL: VikunjaTaskURL(web, strconv.Itoa(t.ID))},
	}
	if last == nil {
		row.State, row.Since = stateInProgress, parseTime(t.Updated)
		return row
	}
	author := last.Author.Username
	if author == "" {
		author = unknownAuthor
	}
	row.Since = parseTime(last.Created)
	if author == v.Operator {
		row.State = stateWaitingOnAgent
		return row
	}
	row.State = replyFromPrefix + author
	row.Detail = t.Title + " — " + snippet(last.Comment)
	if stale.IsStale(row.Since) {
		row.State, row.Detail = StateStale, replyFromPrefix+author+": "+row.Detail
		return row
	}
	row.NeedsYou = true
	return row
}
