package source

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	taskPodSelector     = "kelos.dev/component=task"
	waitForBlockers     = "wait-for-blockers"
	vikunjaTaskLabel    = "vikunja.spe.quebec/task"
	vikunjaProjectLabel = "vikunja.spe.quebec/project"
	activeCondition     = "Active"
	conditionTrue       = "True"
	phasePending        = "Pending"
	phaseReady          = "Ready"
	phaseSucceeded      = "Succeeded"
	phaseFailed         = "Failed"
	stateDone           = "done"
	stateFailed         = "failed"
	stateRunning        = "running"
	stateBlocked        = "blocked"
	resultPR            = "pr"
	resultBranch        = "branch"
	// detailSep separates the parts of a Task's DETAIL.
	detailSep = " · "
	// RecentDone is how long a finished Task stays listed; its PR then shows
	// among the PR rows.
	RecentDone = time.Hour
)

type objectMeta struct {
	Name              string            `json:"name"`
	Labels            map[string]string `json:"labels"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

type kubeCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

type kelosSession struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Phase      string          `json:"phase"`
		Conditions []kubeCondition `json:"conditions"`
	} `json:"status"`
}

type kelosTask struct {
	Metadata objectMeta `json:"metadata"`
	Spec     struct {
		Prompt string `json:"prompt"`
		Branch string `json:"branch"`
	} `json:"spec"`
	Status struct {
		Phase          string            `json:"phase"`
		PodName        string            `json:"podName"`
		StartTime      string            `json:"startTime"`
		CompletionTime string            `json:"completionTime"`
		Message        string            `json:"message"`
		Results        map[string]string `json:"results"`
	} `json:"status"`
}

type containerStatus struct {
	Name  string         `json:"name"`
	State map[string]any `json:"state"`
}

type pod struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
	} `json:"status"`
}

type itemList[T any] struct {
	Items []T `json:"items"`
}

// Kelos reads Kelos Sessions and Tasks with kubectl.
type Kelos struct {
	Namespace  string
	ConsoleURL string
	VikunjaWeb string // Vikunja web base, for a Task's Vikunja link
	Run        Runner
	Now        func() time.Time
}

func (k Kelos) items(ctx context.Context, v any, args ...string) error {
	full := append([]string{"-n", k.Namespace, "get"}, args...)
	return runJSON(ctx, k.Run, v, "kubectl", append(full, "-o", "json")...)
}

// Sessions lists the namespace's Kelos Sessions.
func (k Kelos) Sessions(ctx context.Context) ([]Row, error) {
	var list itemList[kelosSession]
	if err := k.items(ctx, &list, "sessions.kelos.dev"); err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(list.Items))
	for _, s := range list.Items {
		rows = append(rows, k.sessionRow(s))
	}
	return rows, nil
}

func (k Kelos) sessionRow(s kelosSession) Row {
	phase := s.Status.Phase
	if phase == "" {
		phase = phasePending
	}
	active := findCondition(s.Status.Conditions, activeCondition)
	since := s.Metadata.CreationTimestamp
	if active != nil && active.LastTransitionTime != "" {
		since = active.LastTransitionTime
	}
	row := Row{
		Source: SourceKelosSession,
		Name:   s.Metadata.Name,
		Since:  parseTime(since),
		Detail: s.Metadata.Labels[vikunjaProjectLabel],
		Target: Target{URL: k.ConsoleURL},
	}
	switch {
	case phase != phaseReady:
		row.State, row.NeedsYou = strings.ToLower(phase), phase == phaseFailed
	case active != nil && active.Status == conditionTrue:
		row.State, row.Busy = StateBusy, true
	default:
		row.State = StateIdle
	}
	return row
}

func findCondition(conds []kubeCondition, kind string) *kubeCondition {
	for i := range conds {
		if conds[i].Type == kind {
			return &conds[i]
		}
	}
	return nil
}

// Tasks lists the namespace's Kelos Tasks, hiding those finished more than
// RecentDone ago.
func (k Kelos) Tasks(ctx context.Context) ([]Row, error) {
	var pods itemList[pod]
	if err := k.items(ctx, &pods, "pods", "-l", taskPodSelector); err != nil {
		return nil, err
	}
	var tasks itemList[kelosTask]
	if err := k.items(ctx, &tasks, "tasks.kelos.dev"); err != nil {
		return nil, err
	}
	byName := make(map[string]pod, len(pods.Items))
	for _, p := range pods.Items {
		byName[p.Metadata.Name] = p
	}
	now := k.Now()
	var rows []Row
	for _, t := range tasks.Items {
		if row, ok := k.taskRow(t, byName, now); ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (k Kelos) taskRow(t kelosTask, pods map[string]pod, now time.Time) (Row, bool) {
	st := t.Status
	pr := taskPR(t)
	row := Row{
		Source:     SourceKelosTask,
		Name:       t.Metadata.Name,
		Target:     k.taskTarget(t, pr),
		ParentRefs: taskParents(t, pr),
	}
	switch st.Phase {
	case phaseSucceeded:
		row.State, row.Since = stateDone, parseTime(st.CompletionTime)
		row.Detail = taskDetail(t, pr, row.State)
		return row, !row.Since.IsZero() && now.Sub(row.Since) <= RecentDone
	case phaseFailed:
		row.State, row.Since, row.NeedsYou = stateFailed, parseTime(st.CompletionTime), true
		row.Detail = taskDetail(t, pr, row.State)
		if st.Message != "" {
			row.Detail = strings.Join(nonEmpty(taskActivity(t), st.Message), detailSep)
		}
		return row, true
	}
	row.Since = parseTime(cmp.Or(st.StartTime, t.Metadata.CreationTimestamp))
	row.State = liveState(t, pods)
	row.Busy = row.State == stateRunning
	if activity := taskActivity(t); row.Busy && activity != "" {
		row.State = activity
	}
	row.Detail = taskDetail(t, pr, row.State)
	return row, true
}

// liveState is the state of an unfinished Task: blocked while its
// wait-for-blockers init container runs, else its lowercased phase.
func liveState(t kelosTask, pods map[string]pod) string {
	if p, ok := pods[t.Status.PodName]; ok && waitingOnBlockers(p) {
		return stateBlocked
	}
	if t.Status.Phase == "" {
		return strings.ToLower(phasePending)
	}
	return strings.ToLower(t.Status.Phase)
}

// waitingOnBlockers reports whether the pod is held by its wait-for-blockers
// init container.
func waitingOnBlockers(p pod) bool {
	for _, s := range p.Status.InitContainerStatuses {
		if _, running := s.State[stateRunning]; s.Name == waitForBlockers && running {
			return true
		}
	}
	return false
}

// taskPR is the PR a Task produced (its result) or works on (named in the
// prompt Kelos gives conflict, follow-up and plumbing Tasks; an
// implementation prompt is the owner's text and is not read).
func taskPR(t kelosTask) string {
	if pr := t.Status.Results[resultPR]; pr != "" {
		return pr
	}
	if t.Metadata.Labels[roleLabel] != "" {
		return promptPR(t.Spec.Prompt)
	}
	return ""
}

// taskDetail lists the Task's activity (unless it is the state), its
// Vikunja task and its PR.
func taskDetail(t kelosTask, pr, state string) string {
	var parts []string
	if activity := taskActivity(t); activity != "" && activity != state {
		parts = append(parts, activity)
	}
	if id := t.Metadata.Labels[vikunjaTaskLabel]; id != "" {
		parts = append(parts, "vikunja #"+id)
	}
	if pr != "" {
		parts = append(parts, pr)
	}
	return strings.Join(parts, detailSep)
}

func (k Kelos) taskTarget(t kelosTask, pr string) Target {
	if pr != "" {
		return Target{URL: pr}
	}
	if id := t.Metadata.Labels[vikunjaTaskLabel]; id != "" {
		return Target{URL: VikunjaTaskURL(k.VikunjaWeb, id)}
	}
	logs := fmt.Sprintf("%s/api/resources/tasks/%s/%s/logs",
		strings.TrimSuffix(k.ConsoleURL, "/"), url.PathEscape(k.Namespace), url.PathEscape(t.Metadata.Name))
	return Target{URL: logs}
}

// taskParents links a Task to the PR it produced or works on, then to the
// Vikunja task it works on: its label, else the agent branch it runs on.
func taskParents(t kelosTask, pr string) []string {
	var prRef, task string
	if pr != "" {
		prRef = PRRef(pr)
	}
	if id := t.Metadata.Labels[vikunjaTaskLabel]; id != "" {
		task = VikunjaRef(id)
	} else if id, ok := branchTask(cmp.Or(t.Spec.Branch, t.Status.Results[resultBranch])); ok {
		task = VikunjaRef(id)
	}
	return nonEmpty(prRef, task)
}
