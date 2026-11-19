// Following the board live: load the snapshot, then open the event stream,
// whose every `snapshot` event is the whole board (README, "Server").
//
// - While the browser's EventSource reconnects by itself (a dropped
//   connection), the page says so and keeps the rows it has.
// - A stream the server refused or ended for good (an error status, a
//   shutdown) is retried from the snapshot, waiting longer after each
//   failure, as is a snapshot that cannot be loaded or read.
// - Toggling the hidden rows reconnects: the tree differs, so the page
//   cannot derive one layout from the other. Late answers of the former
//   layout are dropped.
import { eventsPath, type Api, type Layout } from './api';
import { decodeSnapshot } from './decode';
import type { Snapshot } from './model';

/** The event every snapshot frame carries (internal/server/events.go); keepalive frames carry none. */
export const SNAPSHOT_EVENT = 'snapshot';
/** EventSource.readyState once it gave up, or was closed. */
const CLOSED = 2;

/** The first wait before trying again after a failure; it doubles up to RETRY_MAX_MS. */
export const RETRY_BASE_MS = 1000;
export const RETRY_MAX_MS = 30_000;

/** What the controller needs of an EventSource. */
export interface StreamSource {
	readonly readyState: number;
	addEventListener(type: string, listener: (event: Event) => void): void;
	close(): void;
}

export type OpenStream = (url: string) => StreamSource;
/** Runs retry after ms and returns a cancel. */
export type Timer = (retry: () => void, ms: number) => () => void;

export type Connection =
	| { readonly state: 'connecting' }
	| { readonly state: 'live' }
	/** The browser lost the stream and reconnects by itself. */
	| { readonly state: 'reconnecting' }
	/** The last try failed; the next one is in retryInMs. */
	| { readonly state: 'retrying'; readonly error: string; readonly retryInMs: number };

export interface LiveOptions {
	readonly api: Api;
	readonly onsnapshot: (snapshot: Snapshot, layout: Layout) => void;
	readonly onconnection: (connection: Connection) => void;
	readonly openStream?: OpenStream;
	readonly timer?: Timer;
}

const defaultOpenStream: OpenStream = (url) => new EventSource(url);
const defaultTimer: Timer = (retry, ms) => {
	const id = setTimeout(retry, ms);
	return () => clearTimeout(id);
};

function messageOf(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}

function sameLayout(a: Layout, b: Layout): boolean {
	return a.showHidden === b.showHidden && a.hideDone === b.hideDone;
}

/** Keeps the page's board current until stopped. */
export class LiveBoard {
	private readonly options: LiveOptions;
	private layout: Layout = { showHidden: false, hideDone: false };
	/** Bumped by every connect, so that a former one's callbacks do nothing. */
	private generation = 0;
	private failures = 0;
	private source: StreamSource | undefined;
	private loading: AbortController | undefined;
	private cancelRetry: (() => void) | undefined;
	private stopped = false;

	constructor(options: LiveOptions) {
		this.options = options;
	}

	start(layout: Layout): void {
		this.layout = layout;
		void this.connect();
	}

	/** Switches the layout, reconnecting when either field differs from the stored one. */
	setLayout(layout: Layout): void {
		if (sameLayout(layout, this.layout) || this.stopped) return;
		this.layout = layout;
		this.failures = 0;
		void this.connect();
	}

	/** Shows a snapshot another call returned, unless it is of another layout. */
	offer(snapshot: Snapshot, layout: Layout): void {
		if (!this.stopped && sameLayout(layout, this.layout)) {
			this.options.onsnapshot(snapshot, layout);
		}
	}

	stop(): void {
		this.stopped = true;
		this.generation++;
		this.disconnect();
	}

	private disconnect(): void {
		this.source?.close();
		this.source = undefined;
		this.loading?.abort();
		this.loading = undefined;
		this.cancelRetry?.();
		this.cancelRetry = undefined;
	}

	private async connect(): Promise<void> {
		const generation = ++this.generation;
		const layout = this.layout;
		this.disconnect();
		this.options.onconnection({ state: 'connecting' });
		const loading = new AbortController();
		this.loading = loading;
		let snapshot: Snapshot;
		try {
			snapshot = await this.options.api.snapshot(layout, loading.signal);
		} catch (err) {
			if (generation === this.generation) this.retry(generation, messageOf(err));
			return;
		}
		if (generation !== this.generation) return;
		this.loading = undefined;
		this.options.onsnapshot(snapshot, layout);
		this.open(generation, layout);
	}

	private open(generation: number, layout: Layout): void {
		const source = (this.options.openStream ?? defaultOpenStream)(eventsPath(layout));
		this.source = source;
		const current = () => generation === this.generation;
		source.addEventListener('open', () => {
			if (current()) this.options.onconnection({ state: 'live' });
		});
		source.addEventListener(SNAPSHOT_EVENT, (event) => {
			if (!current()) return;
			let snapshot: Snapshot;
			try {
				snapshot = decodeSnapshot(JSON.parse((event as MessageEvent<string>).data));
			} catch (err) {
				this.retry(generation, `unreadable event: ${messageOf(err)}`);
				return;
			}
			this.failures = 0;
			this.options.onsnapshot(snapshot, layout);
		});
		source.addEventListener('error', () => {
			if (!current()) return;
			if (source.readyState === CLOSED) this.retry(generation, 'the event stream closed');
			else this.options.onconnection({ state: 'reconnecting' });
		});
	}

	private retry(generation: number, error: string): void {
		this.disconnect();
		const retryInMs = Math.min(RETRY_BASE_MS * 2 ** this.failures, RETRY_MAX_MS);
		this.failures++;
		this.options.onconnection({ state: 'retrying', error, retryInMs });
		this.cancelRetry = (this.options.timer ?? defaultTimer)(() => {
			if (generation === this.generation) void this.connect();
		}, retryInMs);
	}
}
