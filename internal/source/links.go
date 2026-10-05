package source

import "strings"

// Agent branches are named after the Vikunja task they work on.
const (
	agentBranchPrefix = "kelos/vikunja-"
	vikunjaRefPrefix  = "vikunja:"
	prRefPrefix       = "pr:"
)

// VikunjaRef is the ref of the row for Vikunja task id.
func VikunjaRef(id string) string {
	return vikunjaRefPrefix + id
}

// PRRef is the ref of the row for the PR at url.
func PRRef(url string) string {
	return prRefPrefix + strings.TrimSuffix(url, "/")
}

// branchTask returns the Vikunja task id an agent branch works on, if any.
func branchTask(branch string) (string, bool) {
	id, ok := strings.CutPrefix(branch, agentBranchPrefix)
	if !ok || id == "" || strings.Trim(id, "0123456789") != "" {
		return "", false
	}
	return id, true
}

// nonEmpty drops empty refs.
func nonEmpty(refs ...string) []string {
	var out []string
	for _, r := range refs {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}
