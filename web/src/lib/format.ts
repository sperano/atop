// The text the header and the AGE column show, as the TUI draws it
// (internal/ui/view.go, internal/board/age.go and counts.go).
import type { Counts } from './model';

export const APP_NAME = 'vigie';
export const TITLE_SEP = ' · ';
/** The age of a row whose time is unknown. */
export const UNKNOWN_AGE = '-';

const SECOND_MS = 1000;
const MINUTE_MS = 60 * SECOND_MS;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

const AGE_UNITS: readonly { readonly suffix: string; readonly ms: number }[] = [
	{ suffix: 'd', ms: DAY_MS },
	{ suffix: 'h', ms: HOUR_MS },
	{ suffix: 'm', ms: MINUTE_MS },
	{ suffix: 's', ms: SECOND_MS }
];

/** The time since `since` in its largest whole unit: 59s, 1h, 3d. */
export function formatAge(since: number | undefined, now: number): string {
	if (since === undefined) return UNKNOWN_AGE;
	const elapsed = Math.max(now - since, 0);
	for (const unit of AGE_UNITS) {
		if (elapsed >= unit.ms) return `${Math.floor(elapsed / unit.ms)}${unit.suffix}`;
	}
	return '0s';
}

/** `N need you · N busy · N idle`, then `· N hidden` when some are. */
export function formatCounts(c: Counts): string {
	const parts = [`${c.needYou} need you`, `${c.busy} busy`, `${c.idle} idle`];
	if (c.hidden > 0) parts.push(`${c.hidden} hidden`);
	return parts.join(TITLE_SEP);
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Local time as 2006-01-02 15:04:05, the TUI's layout. */
export function formatTime(ms: number): string {
	const d = new Date(ms);
	const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
	return `${date} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** The header line: name, local time and counts. */
export function titleLine(counts: Counts, now: number): string {
	return [APP_NAME, formatTime(now), formatCounts(counts)].join(TITLE_SEP);
}
