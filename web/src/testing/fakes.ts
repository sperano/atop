// Fakes of the browser's EventSource, of timers and of the server's API,
// driven by the tests.
import type { Api } from '../lib/api';
import type { StreamSource, Timer } from '../lib/live';
import type { Row, Snapshot } from '../lib/model';
import { snapshotJSON } from './builders';

const CONNECTING = 0;
const OPEN = 1;
const CLOSED = 2;

export class FakeStream implements StreamSource {
	readyState = CONNECTING;
	closed = false;
	readonly url: string;
	private readonly listeners = new Map<string, ((event: Event) => void)[]>();

	constructor(url: string) {
		this.url = url;
	}

	addEventListener(type: string, listener: (event: Event) => void): void {
		this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
	}

	close(): void {
		this.closed = true;
		this.readyState = CLOSED;
	}

	private emit(type: string, event: Event): void {
		for (const listener of this.listeners.get(type) ?? []) listener(event);
	}

	open(): void {
		this.readyState = OPEN;
		this.emit('open', new Event('open'));
	}

	send(s: Snapshot): void {
		this.sendData(JSON.stringify(snapshotJSON(s)));
	}

	sendData(data: string): void {
		this.emit('snapshot', new MessageEvent('snapshot', { data }));
	}

	/** The connection dropped; the browser reconnects by itself. */
	drop(): void {
		this.readyState = CONNECTING;
		this.emit('error', new Event('error'));
	}

	/** The server refused or ended the stream for good. */
	fail(): void {
		this.readyState = CLOSED;
		this.emit('error', new Event('error'));
	}
}

export class FakeStreams {
	readonly opened: FakeStream[] = [];
	readonly open = (url: string): FakeStream => {
		const s = new FakeStream(url);
		this.opened.push(s);
		return s;
	};

	get last(): FakeStream {
		const s = this.opened.at(-1);
		if (!s) throw new Error('no stream opened');
		return s;
	}
}

export class FakeTimers {
	readonly pending: { retry: () => void; ms: number; cancelled: boolean }[] = [];
	readonly timer: Timer = (retry, ms) => {
		const t = { retry, ms, cancelled: false };
		this.pending.push(t);
		return () => {
			t.cancelled = true;
		};
	};

	/** Runs every timer not cancelled yet. */
	fire(): void {
		for (const t of this.pending.splice(0)) if (!t.cancelled) t.retry();
	}
}

/** A promise the test settles, to decide when an answer arrives. */
export interface Deferred<T> {
	promise: Promise<T>;
	resolve(value: T): void;
	reject(err: unknown): void;
}

export function deferred<T>(): Deferred<T> {
	let resolve!: (value: T) => void;
	let reject!: (err: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

/** An Api whose answers the test queues; every call is recorded. */
export class FakeApi implements Api {
	readonly snapshots: Deferred<Snapshot>[] = [];
	readonly calls: string[] = [];
	/** The signal of every snapshot call, in call order. */
	readonly signals: (AbortSignal | undefined)[] = [];
	refreshAnswer: () => Promise<Snapshot> = () => Promise.reject(new Error('no refresh answer'));
	hiddenAnswer: () => Promise<void> = () => Promise.resolve();

	snapshot(showHidden: boolean, signal?: AbortSignal): Promise<Snapshot> {
		this.calls.push(`snapshot ${showHidden}`);
		this.signals.push(signal);
		const d = deferred<Snapshot>();
		this.snapshots.push(d);
		return d.promise;
	}

	refresh(showHidden: boolean): Promise<Snapshot> {
		this.calls.push(`refresh ${showHidden}`);
		return this.refreshAnswer();
	}

	setHidden(row: Pick<Row, 'source' | 'name'>, hide: boolean): Promise<void> {
		this.calls.push(`${hide ? 'hide' : 'unhide'} ${row.source} ${row.name}`);
		return this.hiddenAnswer();
	}

	/** Answers the oldest snapshot call still waiting. */
	answer(s: Snapshot): void {
		const d = this.snapshots.shift();
		if (!d) throw new Error('no snapshot call waiting');
		d.resolve(s);
	}

	fail(err: unknown): void {
		const d = this.snapshots.shift();
		if (!d) throw new Error('no snapshot call waiting');
		d.reject(err);
	}
}

/** Lets the promise callbacks queued so far run. */
export async function settle(): Promise<void> {
	for (let i = 0; i < 5; i++) await Promise.resolve();
}
