package source

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// vikunjaProject is the part of a Vikunja project vigie reads.
type vikunjaProject struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// labelSeparator joins a task's global id to its project and number.
const labelSeparator = " - "

// projectTitles maps every project the token can read, archived ones
// included, to its title.
func (v *Vikunja) projectTitles(ctx context.Context, token string) (map[int]string, error) {
	params := url.Values{
		"per_page":    {strconv.Itoa(VikunjaPageSize)},
		"is_archived": {"true"},
	}
	titles := make(map[int]string)
	for page := firstPage; ; page++ {
		params.Set("page", strconv.Itoa(page))
		var batch []vikunjaProject
		header, err := v.get(ctx, token, "/projects", params, &batch)
		if err != nil {
			return nil, err
		}
		for _, p := range batch {
			titles[p.ID] = p.Title
		}
		if lastPage(header, page, len(batch)) {
			return titles, nil
		}
	}
}

// taskLabel is how a task's NAME is shown: its global id, then its project
// and its number in that project ("#1647 - Hollingsworth #577"), or the id
// alone when either is unknown.
func taskLabel(t vikunjaTask, projects map[int]string) string {
	name := vikunjaName(t)
	title := printable(projects[t.ProjectID])
	if title == "" || t.Index <= 0 {
		return name
	}
	return name + labelSeparator + title + " #" + strconv.Itoa(t.Index)
}

// printable flattens a title to one line of printable text: control
// characters (ESC, newlines) become spaces, then runs of spaces collapse,
// so that a project title cannot drive the terminal or break the table.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// vikunjaName is a task row's NAME, its identity: the global id.
func vikunjaName(t vikunjaTask) string {
	return "#" + strconv.Itoa(t.ID)
}

// projects lists the project titles, or nothing when no task needs them.
func (v *Vikunja) projects(ctx context.Context, token string, tasks []vikunjaTask) (map[int]string, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	return v.projectTitles(ctx, token)
}
