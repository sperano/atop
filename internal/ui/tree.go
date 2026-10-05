package ui

import (
	"slices"
	"time"

	"github.com/sperano/atop/internal/source"
)

// Tree guides drawn before a child's name.
const (
	branchMid  = "├─ "
	branchLast = "└─ "
	guideOpen  = "│  "
	guideBlank = "   "
	// noParent marks a root in a parent index.
	noParent = -1
)

// Entry is a row placed in the tree, with the guides drawn before its name.
type Entry struct {
	Row    source.Row
	Prefix string
}

// rank orders subtrees: by their most urgent row, then most recent first.
type rank struct {
	group int
	since time.Time
}

func (a rank) compare(b rank) int {
	if a.group != b.group {
		return a.group - b.group
	}
	return b.since.Compare(a.since)
}

type node struct {
	row      source.Row
	children []int
	rank     rank
}

// Arrange nests each row under the first of its ParentRefs that is shown,
// and orders every level by rank: a tree ranks as its most urgent row, so a
// failing PR lifts the Vikunja task it belongs to.
func Arrange(rows []source.Row) []Entry {
	nodes := make([]node, len(rows))
	for i, r := range rows {
		nodes[i] = node{row: r}
	}
	parents := linkParents(rows)
	var roots []int
	for i, p := range parents {
		if p == noParent {
			roots = append(roots, i)
		} else {
			nodes[p].children = append(nodes[p].children, i)
		}
	}
	for _, r := range roots {
		rankSubtree(nodes, r)
	}
	sortByRank(nodes, roots)
	entries := make([]Entry, 0, len(rows))
	for _, r := range roots {
		entries = flatten(nodes, r, "", "", entries)
	}
	return entries
}

// linkParents resolves each row's parent index, or noParent. A link that
// would close a cycle is dropped, so every row is reachable from a root.
func linkParents(rows []source.Row) []int {
	byRef := make(map[string]int, len(rows))
	for i, r := range rows {
		if _, dup := byRef[r.Ref]; r.Ref != "" && !dup {
			byRef[r.Ref] = i
		}
	}
	parents := make([]int, len(rows))
	for i := range parents {
		parents[i] = noParent
	}
	for i, r := range rows {
		for _, ref := range r.ParentRefs {
			if p, ok := byRef[ref]; ok && p != i && !reaches(parents, p, i) {
				parents[i] = p
				break
			}
		}
	}
	return parents
}

// reaches reports whether walking up from `from` arrives at `to`.
func reaches(parents []int, from, to int) bool {
	for steps := 0; from != noParent && steps < len(parents); steps++ {
		if from == to {
			return true
		}
		from = parents[from]
	}
	return false
}

// rankSubtree sets the rank of i and its descendants, and sorts each
// node's children by rank.
func rankSubtree(nodes []node, i int) rank {
	best := rank{group: group(nodes[i].row), since: nodes[i].row.Since}
	for _, c := range nodes[i].children {
		if r := rankSubtree(nodes, c); r.compare(best) < 0 {
			best = r
		}
	}
	sortByRank(nodes, nodes[i].children)
	nodes[i].rank = best
	return best
}

func sortByRank(nodes []node, ids []int) {
	slices.SortStableFunc(ids, func(a, b int) int { return nodes[a].rank.compare(nodes[b].rank) })
}

// flatten appends i and its descendants in display order. prefix is drawn
// before i's name; guide is what its ancestors draw before its children's.
func flatten(nodes []node, i int, prefix, guide string, out []Entry) []Entry {
	out = append(out, Entry{Row: nodes[i].row, Prefix: prefix})
	children := nodes[i].children
	for n, c := range children {
		connector, continuation := branchMid, guideOpen
		if n == len(children)-1 {
			connector, continuation = branchLast, guideBlank
		}
		out = flatten(nodes, c, guide+connector, guide+continuation, out)
	}
	return out
}
