import { beforeEach, describe, expect, test } from 'vitest';
import { row, snapshot } from '../testing/builders';
import { FakeApi, FakeStreams, FakeTimers, settle } from '../testing/fakes';
import { ApiError } from './api';
import { LiveBoard, RETRY_BASE_MS, RETRY_MAX_MS, type Connection } from './live';
import type { Snapshot } from './model';

const FIRST = snapshot([row({ source: 'pr', name: 'a' })]);
const SECOND = snapshot([row({ source: 'pr', name: 'b' })]);
const HIDDEN = snapshot([row({ source: 'pr', name: 'h', hidden: true })]);

let api: FakeApi;
let streams: FakeStreams;
let timers: FakeTimers;
let got: { snapshot: Snapshot; showHidden: boolean }[];
let connections: Connection[];
let live: LiveBoard;

beforeEach(() => {
	api = new FakeApi();
	streams = new FakeStreams();
	timers = new FakeTimers();
	got = [];
	connections = [];
	live = new LiveBoard({
		api,
		openStream: streams.open,
		timer: timers.timer,
		onsnapshot: (s, showHidden) => got.push({ snapshot: s, showHidden }),
		onconnection: (c) => connections.push(c)
	});
});

const lastConnection = () => connections.at(-1);

/** Starts and lets the first snapshot arrive. */
async function started(showHidden = false): Promise<void> {
	live.start(showHidden);
	api.answer(FIRST);
	await settle();
}

describe('LiveBoard', () => {
	test('loads the snapshot, then opens the stream', async () => {
		live.start(false);
		expect(lastConnection()).toEqual({ state: 'connecting' });
		expect(streams.opened).toHaveLength(0);
		api.answer(FIRST);
		await settle();
		expect(got).toEqual([{ snapshot: FIRST, showHidden: false }]);
		expect(streams.last.url).toBe('/api/v1/events');
		streams.last.open();
		expect(lastConnection()).toEqual({ state: 'live' });
	});

	test('shows every snapshot event', async () => {
		await started();
		streams.last.open();
		streams.last.send(SECOND);
		expect(got.map((g) => g.snapshot)).toEqual([FIRST, SECOND]);
	});

	test('says it reconnects while the browser does, and keeps the stream', async () => {
		await started();
		streams.last.open();
		streams.last.drop();
		expect(lastConnection()).toEqual({ state: 'reconnecting' });
		expect(streams.last.closed).toBe(false);
		streams.last.open();
		streams.last.send(SECOND);
		expect(lastConnection()).toEqual({ state: 'live' });
		expect(got.at(-1)?.snapshot).toEqual(SECOND);
		expect(timers.pending).toHaveLength(0);
	});

	test('retries from the snapshot when the stream closed for good', async () => {
		await started();
		streams.last.fail();
		expect(lastConnection()).toEqual({
			state: 'retrying',
			error: 'the event stream closed',
			retryInMs: RETRY_BASE_MS
		});
		expect(streams.last.closed).toBe(true);
		timers.fire();
		expect(api.calls).toEqual(['snapshot false', 'snapshot false']);
		api.answer(SECOND);
		await settle();
		expect(streams.opened).toHaveLength(2);
		expect(got.at(-1)?.snapshot).toEqual(SECOND);
	});

	test('waits twice as long after each failure, up to the maximum', async () => {
		live.start(false);
		const waits: number[] = [];
		for (let i = 0; i < 7; i++) {
			api.fail(new ApiError(503, 'too many event streams'));
			await settle();
			const c = lastConnection();
			if (c?.state !== 'retrying') throw new Error(`state ${c?.state}`);
			expect(c.error).toBe('too many event streams');
			waits.push(c.retryInMs);
			timers.fire();
		}
		expect(waits).toEqual([1000, 2000, 4000, 8000, 16000, RETRY_MAX_MS, RETRY_MAX_MS]);
	});

	test('a snapshot event resets the wait', async () => {
		live.start(false);
		api.fail(new Error('offline'));
		await settle();
		timers.fire();
		api.answer(FIRST);
		await settle();
		streams.last.send(SECOND);
		streams.last.fail();
		expect(lastConnection()).toMatchObject({ retryInMs: RETRY_BASE_MS });
	});

	test('retries on an event it cannot read, without showing it', async () => {
		await started();
		streams.last.sendData('{"rows": 3}');
		expect(got).toHaveLength(1);
		expect(lastConnection()).toEqual({
			state: 'retrying',
			error: 'unreadable event: snapshot.rows is not an array',
			retryInMs: RETRY_BASE_MS
		});
		expect(streams.last.closed).toBe(true);
	});

	test('showing hidden rows reconnects to their layout', async () => {
		await started();
		const first = streams.last;
		live.setShowHidden(true);
		expect(first.closed).toBe(true);
		expect(api.calls.at(-1)).toBe('snapshot true');
		api.answer(HIDDEN);
		await settle();
		expect(got.at(-1)).toEqual({ snapshot: HIDDEN, showHidden: true });
		expect(streams.last.url).toBe('/api/v1/events?show_hidden=true');
	});

	test('setting the same layout changes nothing', async () => {
		await started();
		live.setShowHidden(false);
		expect(api.calls).toEqual(['snapshot false']);
		expect(streams.last.closed).toBe(false);
	});

	test("drops the former layout's late snapshot and events", async () => {
		live.start(false);
		live.setShowHidden(true);
		api.answer(FIRST); // the answer to the first, show_hidden=false call
		await settle();
		expect(got).toEqual([]);
		expect(streams.opened).toHaveLength(0);
		api.answer(HIDDEN);
		await settle();
		expect(got).toEqual([{ snapshot: HIDDEN, showHidden: true }]);
	});

	test("drops the former layout's failed snapshot", async () => {
		live.start(false);
		live.setShowHidden(true);
		api.fail(new Error('aborted')); // the aborted show_hidden=false call
		await settle();
		expect(lastConnection()).toEqual({ state: 'connecting' });
		expect(timers.pending).toHaveLength(0);
		api.answer(HIDDEN);
		await settle();
		expect(got).toEqual([{ snapshot: HIDDEN, showHidden: true }]);
	});

	test("switching the layout aborts the former layout's snapshot", () => {
		live.start(false);
		live.setShowHidden(true);
		expect(api.signals.map((s) => s?.aborted)).toEqual([true, false]);
	});

	test('switching the layout starts the wait over', async () => {
		live.start(false);
		for (let i = 0; i < 2; i++) {
			api.fail(new Error('offline'));
			await settle();
			timers.fire();
		}
		expect(lastConnection()).toEqual({ state: 'connecting' });
		live.setShowHidden(true);
		api.fail(new Error('offline')); // the abandoned third show_hidden=false call
		api.fail(new Error('offline'));
		await settle();
		expect(lastConnection()).toEqual({
			state: 'retrying',
			error: 'offline',
			retryInMs: RETRY_BASE_MS
		});
	});

	test('a retry pending for the former layout does not fire', async () => {
		live.start(false);
		api.fail(new Error('offline'));
		await settle();
		live.setShowHidden(true);
		timers.fire();
		expect(api.calls).toEqual(['snapshot false', 'snapshot true']);
	});

	test('offer shows a snapshot of the current layout only', async () => {
		await started();
		live.offer(HIDDEN, true);
		expect(got).toHaveLength(1);
		live.offer(SECOND, false);
		expect(got.at(-1)?.snapshot).toEqual(SECOND);
	});

	test('stop closes the stream and ignores what comes after', async () => {
		await started();
		const stream = streams.last;
		live.stop();
		expect(stream.closed).toBe(true);
		stream.send(SECOND);
		live.offer(SECOND, false);
		live.setShowHidden(true);
		expect(got).toHaveLength(1);
		expect(api.calls).toEqual(['snapshot false']);
	});

	test('stop cancels a pending retry', async () => {
		live.start(false);
		api.fail(new Error('offline'));
		await settle();
		expect(lastConnection()).toMatchObject({ state: 'retrying' });
		expect(timers.pending).toHaveLength(1);
		live.stop();
		expect(timers.pending[0]?.cancelled).toBe(true);
	});

	test('stop aborts a snapshot in flight', () => {
		live.start(false);
		live.stop();
		expect(api.signals.map((s) => s?.aborted)).toEqual([true]);
	});
});
