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

// An http or https scheme, //, then an authority made only of what Go's
// url.Parse takes there: userinfo of ASCII letters, digits and
// -._~!$&'()*+,;=:@, then a host of those (no @) plus [] and non-ASCII. The
// WHATWG parser alone also takes http:///x, https:/x, a backslash or a %
// escape in the host, and userinfo Go refuses. No i flag: under /iu, \w and
// [a-z] also match U+017F and U+212A, non-ASCII userinfo Go refuses.
const WEB_URL =
	/^[Hh][Tt][Tt][Pp][Ss]?:\/\/(?:[\w.~!$&'()*+,;=:@-]*@)?[\w.~!$&'()*+,;=:[\]\u{80}-\u{10ffff}-]+(?:[/?#]|$)/u;
// A % that starts no escape, which Go's url.Parse refuses in a path or a
// fragment; the query it leaves unchecked.
const BAD_ESCAPE = /%(?![\da-f]{2})/i;
// Go's url.Parse refuses any ASCII control byte: below a space, and DEL.
const FIRST_PRINTABLE = ' ';
const DELETE = '\x7f';

function hasControl(s: string): boolean {
	return [...s].some((c) => c < FIRST_PRINTABLE || c === DELETE);
}

/**
 * A target the page may link and open: an absolute http or https URL with a
 * host, kept only when both the browser's parser and Go's url.Parse, which
 * the TUI's opener checks with (internal/ui/open.go), accept it; the page may
 * drop a target the TUI opens (a bad % escape in the query). Targets come
 * from Kelos Tasks' results and prompts, so anything else (a javascript: URL)
 * becomes no target rather than script run on this page's origin.
 */
function webTarget(o: Fields, key: string, path: string): string {
	const v = string(o, key, path);
	const kept = WEB_URL.test(v) && !BAD_ESCAPE.test(v) && !hasControl(v) && URL.canParse(v);
	return kept ? v : '';
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
		hidden: boolean(o, 'hidden', path),
		hideable: boolean(o, 'hideable', path)
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
