// Snapshots and rows for the tests, in the server's JSON and decoded.
import type { Row, Snapshot } from '../lib/model';

/** 2026-11-15 12:00:00 UTC, the server tests' clock. */
export const TEST_NOW = Date.UTC(2026, 10, 15, 12, 0, 0);
export const HOUR_MS = 60 * 60 * 1000;

export function row(fields: Partial<Row> & Pick<Row, 'source' | 'name'>): Row {
	return {
		label: fields.name,
		origin: 'github',
		depth: 0,
		guides: '',
		mark: '',
		state: 'idle',
		since: undefined,
		age: '-',
		detail: '',
		target: '',
		severity: 'quiet',
		needsYou: false,
		busy: false,
		hidden: false,
		hideable: true,
		...fields
	};
}

export function snapshot(rows: readonly Row[], fields: Partial<Snapshot> = {}): Snapshot {
	return {
		fetchedAt: TEST_NOW,
		refreshing: false,
		error: '',
		counts: { needYou: 0, busy: 0, idle: 0, hidden: 0, done: 0 },
		rows,
		...fields
	};
}

/** A row as `vigie serve` sends it. */
export function rowJSON(r: Row): Record<string, unknown> {
	return {
		source: r.source,
		name: r.name,
		label: r.label,
		origin: r.origin,
		depth: r.depth,
		guides: r.guides,
		mark: r.mark,
		state: r.state,
		since: r.since === undefined ? null : new Date(r.since).toISOString(),
		age: r.age,
		detail: r.detail,
		target: r.target,
		severity: r.severity,
		needs_you: r.needsYou,
		busy: r.busy,
		hidden: r.hidden,
		hideable: r.hideable
	};
}

/** A snapshot as `vigie serve` sends it. */
export function snapshotJSON(s: Snapshot): Record<string, unknown> {
	return {
		fetched_at: s.fetchedAt === undefined ? null : new Date(s.fetchedAt).toISOString(),
		refreshing: s.refreshing,
		error: s.error,
		counts: {
			need_you: s.counts.needYou,
			busy: s.counts.busy,
			idle: s.counts.idle,
			hidden: s.counts.hidden,
			done: s.counts.done
		},
		rows: s.rows.map(rowJSON)
	};
}
