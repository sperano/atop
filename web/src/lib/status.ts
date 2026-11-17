// The status line under the table: what is going on, then the last action
// or error, as the TUI's (internal/ui/view.go and hide.go).
import { TITLE_SEP } from './format';
import type { Connection } from './live';

const MS_PER_SECOND = 1000;

/** What starts an error in the status line. */
export const ERROR_PREFIX = 'error: ';
export const STATUS_REFRESHING = 'refreshing…';
export const STATUS_SHOWS_HIDDEN = 'showing hidden rows';
export const STATUS_HIDES_HIDDEN = 'hidden rows are hidden again';
export const STATUS_NOTHING = 'nothing to open';
export const STATUS_CONNECTING = 'connecting…';
export const STATUS_RECONNECTING = 'connection lost, reconnecting…';

export interface StatusInput {
	readonly connection: Connection;
	readonly refreshing: boolean;
	readonly showHidden: boolean;
	/** The server's last hidden-row error, empty when none. */
	readonly serverError: string;
	/** The last action's outcome, empty when none. */
	readonly message: string;
}

function connectionText(c: Connection): string {
	switch (c.state) {
		case 'connecting':
			return STATUS_CONNECTING;
		case 'reconnecting':
			return STATUS_RECONNECTING;
		case 'retrying':
			return `${ERROR_PREFIX}${c.error}${TITLE_SEP}retrying in ${Math.ceil(c.retryInMs / MS_PER_SECOND)}s`;
		case 'live':
			return '';
	}
}

export function statusLine(s: StatusInput): string {
	const parts = [
		connectionText(s.connection),
		s.refreshing ? STATUS_REFRESHING : '',
		s.showHidden ? STATUS_SHOWS_HIDDEN : '',
		s.serverError ? `${ERROR_PREFIX}${s.serverError}` : '',
		s.message
	];
	return parts.filter((p) => p !== '').join(TITLE_SEP);
}

/** Whether the status line reports an error, which the footer shows in red. */
export function hasError(line: string): boolean {
	return line.startsWith(ERROR_PREFIX) || line.includes(`${TITLE_SEP}${ERROR_PREFIX}`);
}

export function describe(row: { readonly source: string; readonly name: string }): string {
	return `${row.source} ${row.name}`;
}

export function hidMessage(row: { readonly source: string; readonly name: string }): string {
	return `hid ${describe(row)}${TITLE_SEP}H shows hidden rows`;
}

export function unhidMessage(row: { readonly source: string; readonly name: string }): string {
	return `unhid ${describe(row)}`;
}

export function tooSoonMessage(retryAfter: number | undefined): string {
	const base = 'refreshed too recently';
	return retryAfter === undefined ? base : `${base}${TITLE_SEP}try again in ${retryAfter}s`;
}
