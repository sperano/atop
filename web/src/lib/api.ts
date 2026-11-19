// The calls to `vigie serve`'s API (README, "Server").
import { decodeSnapshot } from './decode';
import type { Row, Snapshot } from './model';

export const API_BASE = '/api/v1';
/** Lists the hidden rows too, as H does. */
const SHOW_HIDDEN_PARAM = 'show_hidden';
/** Drops the done rows, as d does. */
const HIDE_DONE_PARAM = 'hide_done';

/** The two independent display filters a request may ask for. */
export interface Layout {
	readonly showHidden: boolean;
	readonly hideDone: boolean;
}

const ACCEPTED = 202;
const NO_CONTENT = 204;
const TOO_MANY_REQUESTS = 429;
const CONFLICT = 409;

/** An answer other than the one asked for, with the server's own words. */
export class ApiError extends Error {
	override name = 'ApiError';
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

/** Refreshing within the server's cooldown: it started none. */
export class TooSoonError extends ApiError {
	override name = 'TooSoonError';
	/** Seconds until a refresh may start, undefined when the server did not say. */
	readonly retryAfter: number | undefined;

	constructor(message: string, retryAfter: number | undefined) {
		super(TOO_MANY_REQUESTS, message);
		this.retryAfter = retryAfter;
	}
}

/** A source's error row, which the server refuses to hide. */
export function isRefusal(err: unknown): boolean {
	return err instanceof ApiError && err.status === CONFLICT;
}

function withLayout(path: string, layout: Layout): string {
	const params = new URLSearchParams();
	if (layout.showHidden) params.set(SHOW_HIDDEN_PARAM, 'true');
	if (layout.hideDone) params.set(HIDE_DONE_PARAM, 'true');
	const query = params.toString();
	return query === '' ? path : `${path}?${query}`;
}

export function snapshotPath(layout: Layout): string {
	return withLayout(`${API_BASE}/snapshot`, layout);
}

export function eventsPath(layout: Layout): string {
	return withLayout(`${API_BASE}/events`, layout);
}

export function refreshPath(layout: Layout): string {
	return withLayout(`${API_BASE}/refresh`, layout);
}

export function hiddenPath(row: Pick<Row, 'source' | 'name'>): string {
	return `${API_BASE}/hidden/${encodeURIComponent(row.source)}/${encodeURIComponent(row.name)}`;
}

/** The error body's message, else the status line. */
async function failure(resp: Response): Promise<string> {
	try {
		const body: unknown = await resp.json();
		if (typeof body === 'object' && body !== null && 'error' in body) {
			const { error } = body;
			if (typeof error === 'string' && error !== '') return error;
		}
	} catch {
		// Not JSON (a proxy's page): the status says enough.
	}
	return `${resp.status} ${resp.statusText}`.trim();
}

function retryAfter(resp: Response): number | undefined {
	const seconds = Number(resp.headers.get('Retry-After') ?? NaN);
	return Number.isFinite(seconds) && seconds >= 0 ? seconds : undefined;
}

export interface Api {
	snapshot(layout: Layout, signal?: AbortSignal): Promise<Snapshot>;
	/** Starts a refresh or joins the running one; throws TooSoonError in the cooldown. */
	refresh(layout: Layout): Promise<Snapshot>;
	setHidden(row: Pick<Row, 'source' | 'name'>, hide: boolean): Promise<void>;
}

export function createApi(fetcher: typeof fetch = (...args) => fetch(...args)): Api {
	return {
		async snapshot(layout, signal) {
			const resp = await fetcher(snapshotPath(layout), { signal });
			if (!resp.ok) throw new ApiError(resp.status, await failure(resp));
			return decodeSnapshot(await resp.json());
		},
		async refresh(layout) {
			const resp = await fetcher(refreshPath(layout), { method: 'POST' });
			if (resp.status === TOO_MANY_REQUESTS) {
				throw new TooSoonError(await failure(resp), retryAfter(resp));
			}
			if (resp.status !== ACCEPTED) throw new ApiError(resp.status, await failure(resp));
			return decodeSnapshot(await resp.json());
		},
		async setHidden(row, hide) {
			const resp = await fetcher(hiddenPath(row), { method: hide ? 'PUT' : 'DELETE' });
			if (resp.status !== NO_CONTENT) throw new ApiError(resp.status, await failure(resp));
		}
	};
}
