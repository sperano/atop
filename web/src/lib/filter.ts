// Filtering the rows shown by one of the header badges (need you / busy /
// idle), so a filter always shows exactly as many rows as the badge counts
// (internal/board/counts.go's count()).
import type { Row } from './model';

export const FILTER_KINDS = ['need-you', 'busy', 'idle'] as const;
export type FilterKind = (typeof FILTER_KINDS)[number];

export const FILTER_LABELS: Record<FilterKind, string> = {
	'need-you': 'need you',
	busy: 'busy',
	idle: 'idle'
};

// internal/board/counts.go's agentSources: a busy Vikunja task or PR an
// agent merely speaks for is not one of these, and is not counted as busy.
const AGENT_SOURCES = new Set(['local claude', 'kelos session', 'kelos task']);

export function matchesFilter(row: Row, filter: FilterKind): boolean {
	// count() in internal/board/counts.go counts only the shown (non-hidden)
	// rows, for every kind; a hidden row never matches, even while shown by
	// "show hidden".
	if (row.hidden) return false;
	switch (filter) {
		case 'need-you':
			return row.needsYou;
		case 'busy':
			return !row.needsYou && row.busy && AGENT_SOURCES.has(row.source);
		case 'idle':
			return row.state === 'idle';
	}
}

/**
 * rows matching filter, or all of them when filter is undefined. A filter
 * can keep a child while dropping its parent, so matching rows are
 * flattened to the top level: a stale depth or guide would otherwise point
 * at a parent no longer shown.
 */
export function filterRows(rows: readonly Row[], filter: FilterKind | undefined): readonly Row[] {
	if (filter === undefined) return rows;
	return rows
		.filter((row) => matchesFilter(row, filter))
		.map((row) => ({ ...row, depth: 0, guides: '' }));
}
