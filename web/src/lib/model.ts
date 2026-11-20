// The snapshot `vigie serve` sends (internal/server/snapshot.go), decoded
// from its JSON. The server nests, sorts, counts and classifies the rows: the
// page only draws them, recomputing each age from since.

/** How a row is coloured (internal/board/severity.go). */
export const SEVERITIES = ['failed', 'blocked', 'busy', 'attention', 'quiet'] as const;
export type Severity = (typeof SEVERITIES)[number];

export interface Row {
	readonly source: string;
	readonly name: string;
	/** The name as shown; `name` stays the row's identity. */
	readonly label: string;
	readonly origin: string;
	/** Depth in the tree, 0 at the top level. */
	readonly depth: number;
	/** The tree guides drawn before the name. */
	readonly guides: string;
	readonly mark: string;
	readonly state: string;
	/** Milliseconds since the epoch, undefined when unknown. */
	readonly since: number | undefined;
	/** The age when the snapshot was made; the page recomputes it from since. */
	readonly age: string;
	readonly detail: string;
	/** The http(s) URL enter opens, empty when the row has none (or another kind). */
	readonly target: string;
	readonly severity: Severity;
	readonly needsYou: boolean;
	readonly busy: boolean;
	readonly hidden: boolean;
	/** False for a source's error row, which the server refuses to hide. */
	readonly hideable: boolean;
}

export interface Counts {
	readonly needYou: number;
	readonly busy: number;
	readonly idle: number;
	readonly hidden: number;
	readonly done: number;
}

export interface Snapshot {
	/** Milliseconds since the epoch, undefined until the first refresh ends. */
	readonly fetchedAt: number | undefined;
	readonly refreshing: boolean;
	/** The last hidden-row update's error, empty when it worked. */
	readonly error: string;
	readonly counts: Counts;
	/** Nested and sorted, as they are drawn. */
	readonly rows: readonly Row[];
}

/** The mark of a row that needs you (internal/board/board.go). */
export const MARK_NEEDS_YOU = '●';

/** A row's identity, which the selection follows across snapshots. */
export function rowKey(row: Pick<Row, 'source' | 'name'>): string {
	return `${row.source}\u0000${row.name}`;
}
