// Checks the server's JSON before the page relies on it: a snapshot of
// another shape (an older or newer server) fails with what is wrong, rather
// than drawing undefined cells.
import { SEVERITIES, type Counts, type Row, type Severity, type Snapshot } from './model';

export class DecodeError extends Error {
	override name = 'DecodeError';
}

type Fields = Record<string, unknown>;

function object(value: unknown, path: string): Fields {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) {
		throw new DecodeError(`${path} is not an object`);
	}
	return value as Fields;
}

function string(o: Fields, key: string, path: string): string {
	const v = o[key];
	if (typeof v !== 'string') throw new DecodeError(`${path}.${key} is not a string`);
	return v;
}

function boolean(o: Fields, key: string, path: string): boolean {
	const v = o[key];
	if (typeof v !== 'boolean') throw new DecodeError(`${path}.${key} is not a boolean`);
	return v;
}

function count(o: Fields, key: string, path: string): number {
	const v = o[key];
	if (typeof v !== 'number' || !Number.isInteger(v) || v < 0) {
		throw new DecodeError(`${path}.${key} is not a count`);
	}
	return v;
}

/** An RFC 3339 time or null, as milliseconds since the epoch or undefined. */
function optionalTime(o: Fields, key: string, path: string): number | undefined {
	const v = o[key];
	if (v === null) return undefined;
	const ms = typeof v === 'string' ? Date.parse(v) : NaN;
	if (Number.isNaN(ms)) throw new DecodeError(`${path}.${key} is not a time or null`);
	return ms;
}

function severity(o: Fields, key: string, path: string): Severity {
	const v = string(o, key, path);
	if (!(SEVERITIES as readonly string[]).includes(v)) {
		throw new DecodeError(`${path}.${key} is not a severity: ${v}`);
	}
	return v as Severity;
}

// An http or https scheme, then // and the first character of a host, in the
// raw text: the WHATWG parser alone also takes http:///x and https:/x, whose
// host it finds after the slashes, while Go's url.Parse finds none.
const WEB_URL_START = /^https?:\/\/[^/?#\\\s]/i;

/**
 * A target the page may link and open: an absolute http or https URL with a
 * host, as the TUI's opener accepts (internal/ui/open.go). Targets come from
 * Kelos Tasks' results and prompts, so anything else (a javascript: URL)
 * becomes no target rather than script run on this page's origin.
 */
function webTarget(o: Fields, key: string, path: string): string {
	const v = string(o, key, path);
	return WEB_URL_START.test(v) && URL.canParse(v) && new URL(v).host !== '' ? v : '';
}

function decodeRow(value: unknown, path: string): Row {
	const o = object(value, path);
	return {
		source: string(o, 'source', path),
		name: string(o, 'name', path),
		origin: string(o, 'origin', path),
		depth: count(o, 'depth', path),
		guides: string(o, 'guides', path),
		mark: string(o, 'mark', path),
		state: string(o, 'state', path),
		since: optionalTime(o, 'since', path),
		age: string(o, 'age', path),
		detail: string(o, 'detail', path),
		target: webTarget(o, 'target', path),
		severity: severity(o, 'severity', path),
		needsYou: boolean(o, 'needs_you', path),
		busy: boolean(o, 'busy', path),
		hidden: boolean(o, 'hidden', path)
	};
}

function decodeCounts(value: unknown, path: string): Counts {
	const o = object(value, path);
	return {
		needYou: count(o, 'need_you', path),
		busy: count(o, 'busy', path),
		idle: count(o, 'idle', path),
		hidden: count(o, 'hidden', path)
	};
}

/** Decodes a snapshot, throwing a DecodeError that says what is wrong. */
export function decodeSnapshot(value: unknown): Snapshot {
	const path = 'snapshot';
	const o = object(value, path);
	const rows = o['rows'];
	if (!Array.isArray(rows)) throw new DecodeError(`${path}.rows is not an array`);
	return {
		fetchedAt: optionalTime(o, 'fetched_at', path),
		refreshing: boolean(o, 'refreshing', path),
		error: string(o, 'error', path),
		counts: decodeCounts(o['counts'], `${path}.counts`),
		rows: rows.map((r, i) => decodeRow(r, `${path}.rows[${i}]`))
	};
}
