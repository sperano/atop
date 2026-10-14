package source

import (
	"regexp"
	"strings"
)

// Kelos labels every spawned Task with the kind of work it does; an
// implementation Task has no role, and a discuss Task is named after its
// mode instead.
const (
	roleLabel         = "vikunja.spe.quebec/role"
	rolePRConflict    = "pr-conflict"
	rolePRFollowUp    = "pr-followup"
	roleTaskFollowUp  = "task-followup"
	discussTaskPrefix = "vikunja-discuss-"
)

// Activities: what a Task is doing, shown as its state while it runs.
const (
	ActivityImplementing = "implementing"
	ActivityDiscussing   = "discussing"
	ActivityConflictFix  = "conflict fix"
	ActivityPRFollowUp   = "pr follow-up"
	ActivityTaskFollowUp = "task follow-up"
)

// How a PR follow-up prompt tells a review bot's report from the owner's
// request.
const (
	// reviewFixSuffix follows the review bot's short name: "cubic fix".
	reviewFixSuffix = " fix"
	// commandPrefix starts an owner's follow-up request ("/kelos ..."); a
	// review bot's report never does.
	commandPrefix = "/"
	// botNameEnds are the characters that end a review bot's short name:
	// "cubic-dev-ai[bot]" is shown as "cubic".
	botNameEnds = "-["
)

var (
	// promptPRRE finds the "PR: <url>" line a conflict, follow-up or
	// plumbing Task's prompt starts with.
	promptPRRE = regexp.MustCompile(`(?mi)^pr: (\S+)\s*$`)
	// promptBotRE finds the review bot a PR follow-up prompt names.
	promptBotRE = regexp.MustCompile(`(?m)^Review bot: (\S+)\s*$`)
	// promptRequestRE captures the request a PR follow-up prompt carries.
	promptRequestRE = regexp.MustCompile(`(?s)<request>\s*(.*?)\s*</request>`)
)

// taskActivity names what a Task does, from its role label, its name and
// its prompt. It is empty for a Task atop does not know.
func taskActivity(t kelosTask) string {
	switch role := t.Metadata.Labels[roleLabel]; role {
	case rolePRConflict:
		return ActivityConflictFix
	case rolePRFollowUp:
		if bot := reviewBotRequest(t.Spec.Prompt); bot != "" {
			return bot + reviewFixSuffix
		}
		return ActivityPRFollowUp
	case roleTaskFollowUp:
		return ActivityTaskFollowUp
	case "":
		return unlabelledActivity(t)
	default:
		return role
	}
}

func unlabelledActivity(t kelosTask) string {
	switch {
	case strings.HasPrefix(t.Metadata.Name, discussTaskPrefix):
		return ActivityDiscussing
	case t.Metadata.Labels[vikunjaTaskLabel] != "":
		return ActivityImplementing
	default:
		return ""
	}
}

// reviewBotRequest returns the short name of the review bot whose report a
// PR follow-up prompt carries as its request, or "" when the request is
// the owner's command.
func reviewBotRequest(prompt string) string {
	bot := promptBotRE.FindStringSubmatch(prompt)
	req := promptRequestRE.FindStringSubmatch(prompt)
	if bot == nil || req == nil || strings.HasPrefix(req[1], commandPrefix) {
		return ""
	}
	return shortBotName(bot[1])
}

func shortBotName(login string) string {
	if i := strings.IndexAny(login, botNameEnds); i > 0 {
		return login[:i]
	}
	return login
}

// promptPR returns the PR URL a Task's prompt names, if any.
func promptPR(prompt string) string {
	if m := promptPRRE.FindStringSubmatch(prompt); m != nil {
		return m[1]
	}
	return ""
}
