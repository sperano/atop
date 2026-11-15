// Which row is selected: an index into the rows as drawn, kept on the same
// row (by source and name) across snapshots, as the TUI does
// (internal/ui/model.go).
import type { Move } from './keys';
import { rowKey, type Row } from './model';

/** i within the rows, 0 when there are none. */
export function clamp(i: number, count: number): number {
	return Math.max(0, Math.min(i, count - 1));
}

/** The index a move goes to; page is the number of rows a page shows. */
export function moveTo(selected: number, move: Move, count: number, page: number): number {
	switch (move) {
		case 'up':
			return clamp(selected - 1, count);
		case 'down':
			return clamp(selected + 1, count);
		case 'pageUp':
			return clamp(selected - page, count);
		case 'pageDown':
			return clamp(selected + page, count);
		case 'home':
			return 0;
		case 'end':
			return clamp(count - 1, count);
	}
}

/**
 * The index of the selected row in the next rows: the same row when it is
 * still there, else the same place, so a row that left gives it to the next.
 */
export function reselect(
	previous: Row | undefined,
	selected: number,
	next: readonly Row[]
): number {
	if (previous) {
		const key = rowKey(previous);
		const i = next.findIndex((r) => rowKey(r) === key);
		if (i >= 0) return i;
	}
	return clamp(selected, next.length);
}

/** The rows a page of height viewport shows, at least one. */
export function pageRows(viewport: number, rowHeight: number): number {
	if (!(rowHeight > 0)) return 1;
	return Math.max(1, Math.floor(viewport / rowHeight));
}
