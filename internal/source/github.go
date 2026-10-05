package source

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// PRSearchLimit is how many open PRs the search returns.
	PRSearchLimit    = 50
	agentLabelPrefix = "kelos:"
	mergeConflicting = "CONFLICTING"
	changesRequested = "CHANGES_REQUESTED"
	stateDraft       = "draft"
	stateConflicts   = "conflicts"
	stateChecksFail  = "checks failing"
	stateChecksRun   = "checks running"
	stateChanges     = "changes requested"
	stateReview      = "review"
)

const prQuery = `
query($q: String!, $n: Int!) {
  search(query: $q, type: ISSUE, first: $n) {
    nodes { ... on PullRequest {
      number title url headRefName isDraft updatedAt mergeable reviewDecision
      repository { name }
      labels(first: 10) { nodes { name } }
      commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
    } }
  }
}`

var (
	checksFailed  = map[string]bool{"FAILURE": true, "ERROR": true}
	checksPending = map[string]bool{"PENDING": true, "EXPECTED": true}
)

type pullRequest struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	HeadRefName    string `json:"headRefName"`
	IsDraft        bool   `json:"isDraft"`
	UpdatedAt      string `json:"updatedAt"`
	Mergeable      string `json:"mergeable"`
	ReviewDecision string `json:"reviewDecision"`
	Repository     struct {
		Name string `json:"name"`
	} `json:"repository"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type prSearch struct {
	Data struct {
		Search struct {
			// Pointers: a non-PR search hit decodes as an empty object.
			Nodes []*pullRequest `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
}

// GitHub lists the owner's open PRs with the gh CLI.
type GitHub struct {
	Owner     string
	StaleDays int
	Run       Runner
	Now       func() time.Time
}

// Fetch lists open PRs in the owner's non-archived repositories.
func (g GitHub) Fetch(ctx context.Context) ([]Row, error) {
	query := fmt.Sprintf("is:pr is:open owner:%s archived:false", g.Owner)
	var res prSearch
	err := runJSON(ctx, g.Run, &res, "gh", "api", "graphql",
		"-f", "query="+prQuery, "-f", "q="+query, "-F", "n="+strconv.Itoa(PRSearchLimit))
	if err != nil {
		return nil, err
	}
	stale := Staleness{Days: g.StaleDays, Now: g.Now()}
	var rows []Row
	for _, pr := range res.Data.Search.Nodes {
		if pr != nil && pr.URL != "" {
			rows = append(rows, prRow(*pr, stale))
		}
	}
	return rows, nil
}

func checksState(pr pullRequest) string {
	nodes := pr.Commits.Nodes
	if len(nodes) == 0 || nodes[0].Commit.StatusCheckRollup == nil {
		return ""
	}
	return nodes[0].Commit.StatusCheckRollup.State
}

// prState maps an open PR to its state, and whether it needs you or is busy.
func prState(pr pullRequest) (state string, needsYou, busy bool) {
	checks := checksState(pr)
	switch {
	case pr.IsDraft:
		return stateDraft, false, false
	case pr.Mergeable == mergeConflicting:
		return stateConflicts, true, false
	case checksFailed[checks]:
		return stateChecksFail, true, false
	case checksPending[checks]:
		return stateChecksRun, false, true
	case pr.ReviewDecision == changesRequested:
		return stateChanges, false, false
	default:
		return stateReview, true, false
	}
}

func agentVariant(pr pullRequest) string {
	for _, l := range pr.Labels.Nodes {
		if variant, ok := strings.CutPrefix(l.Name, agentLabelPrefix); ok {
			return variant
		}
	}
	return ""
}

func prRow(pr pullRequest, stale Staleness) Row {
	state, needsYou, busy := prState(pr)
	title := pr.Title
	if variant := agentVariant(pr); variant != "" {
		title = "[" + variant + "] " + title
	}
	since := parseTime(pr.UpdatedAt)
	if needsYou && stale.IsStale(since) {
		state, needsYou = StateStale, false
	}
	return Row{
		Source:     SourcePR,
		Name:       pr.Repository.Name + "#" + strconv.Itoa(pr.Number),
		State:      state,
		Since:      since,
		Detail:     title,
		NeedsYou:   needsYou,
		Busy:       busy,
		Target:     Target{URL: pr.URL},
		Ref:        PRRef(pr.URL),
		ParentRefs: prParents(pr),
	}
}

// prParents links an agent PR to the Vikunja task its branch is named after.
func prParents(pr pullRequest) []string {
	if id, ok := branchTask(pr.HeadRefName); ok {
		return []string{VikunjaRef(id)}
	}
	return nil
}
