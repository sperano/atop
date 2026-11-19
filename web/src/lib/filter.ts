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
	switch (filter) {
		case 'need-you':
			return row.needsYou;
		case 'busy':
			return !row.needsYou && row.busy && AGENT_SOURCES.has(row.source);
		case 'idle':
			return row.state === 'idle';
	}
}

/** rows matching filter, or all of them when filter is undefined. */
export function filterRows(rows: readonly Row[], filter: FilterKind | undefined): readonly Row[] {
	return filter === undefined ? rows : rows.filter((row) => matchesFilter(row, filter));
}
