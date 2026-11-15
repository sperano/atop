import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { userEvent } from 'vitest/browser';
import { flushSync } from 'svelte';
import App, { TICK_MS } from './App.svelte';
import { ApiError, TooSoonError } from './lib/api';
import type { Row, Snapshot } from './lib/model';
import { render } from './render';
import { HOUR_MS, row, snapshot, TEST_NOW } from './testing/builders';
import { FakeApi, FakeStreams, FakeTimers, settle } from './testing/fakes';

const TASK = row({
	source: 'vikunja',
	name: '#7',
	origin: 'vikunja',
	state: 'idle',
	since: TEST_NOW - 2 * HOUR_MS,
	age: '2h',
	detail: '[claude] serve',
	target: 'https://vikunja.example/tasks/7'
});
const PR = row({
	source: 'pr',
	name: 'vigie#3',
	depth: 1,
	guides: '└─ ',
	mark: '●',
	state: 'review',
	since: TEST_NOW - HOUR_MS,
	age: '1h',
	detail: 'Serve the board',
	target: 'https://github.com/o/vigie/pull/3',
	severity: 'attention',
	needsYou: true
});
const AGENT = row({
	source: 'kelos task',
	name: 'fix',
	origin: 'unknown',
	depth: 2,
	guides: '   └─ ',
	state: 'busy',
	since: undefined,
	age: '-',
	severity: 'busy',
	busy: true
});
const ERROR = row({
	source: 'mirror',
	name: '-',
	state: 'error',
	detail: 'down',
	mark: '●',
	severity: 'failed',
	needsYou: true
});
const BOARD = snapshot([TASK, PR, AGENT, ERROR], {
	counts: { needYou: 2, busy: 1, idle: 1, hidden: 0 }
});

let api: FakeApi;
let streams: FakeStreams;
let timers: FakeTimers;
let opened: string[];
let openAnswer: boolean;
let page: HTMLElement;

beforeEach(() => {
	api = new FakeApi();
	streams = new FakeStreams();
	timers = new FakeTimers();
	opened = [];
	openAnswer = true;
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

/** Mounts the page and lets its first snapshot and stream arrive. */
async function mountWith(first: Snapshot = BOARD): Promise<void> {
	page = render(App, {
		api,
		openStream: streams.open,
		timer: timers.timer,
		now: () => TEST_NOW,
		openUrl: (url: string) => {
			opened.push(url);
			return openAnswer;
		}
	});
	api.answer(first);
	await settle();
	streams.last.open();
	flushSync();
}

const bodyRows = () => [...page.querySelectorAll<HTMLTableRowElement>('tbody tr')];
const cells = (tr: HTMLTableRowElement) => [...tr.cells].map((c) => c.textContent ?? '');
const selectedName = () =>
	page
		.querySelector<HTMLTableRowElement>('tbody tr[aria-current="true"]')
		?.cells[3]?.textContent?.trim();
const statusText = () => page.querySelector('[role="status"]')?.textContent ?? '';
const headerText = () => page.querySelector('header')?.textContent ?? '';
/** One CSS pixel, the rounding allowed on measured positions. */
const PIXEL = 1;
const ages = () => bodyRows().map((tr) => tr.cells[5]?.textContent);

/** ms in the browser's zone, in the TUI's layout. */
function localTime(ms: number): string {
	const t = new Date(ms);
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${t.getFullYear()}-${pad(t.getMonth() + 1)}-${pad(t.getDate())} ${pad(t.getHours())}:${pad(t.getMinutes())}:${pad(t.getSeconds())}`;
}

async function press(keys: string): Promise<void> {
	await userEvent.keyboard(keys);
	await settle();
	flushSync();
}

function send(s: Snapshot): void {
	streams.last.send(s);
	flushSync();
}

describe('the board', () => {
	test('draws the header, the columns and the rows as sent', async () => {
		await mountWith();
		expect(headerText()).toBe(`vigie · ${localTime(TEST_NOW)} · 2 need you · 1 busy · 1 idle`);
		expect([...page.querySelectorAll('thead th')].map((th) => th.textContent)).toEqual([
			'mark',
			'TYPE',
			'ORIGIN',
			'NAME',
			'STATE',
			'AGE',
			'DETAIL'
		]);
		expect(bodyRows().map(cells)).toEqual([
			['', 'vikunja', 'vikunja', '#7', 'idle', '2h', '[claude] serve'],
			['●', 'pr', 'github', '└─ vigie#3', 'review', '1h', 'Serve the board'],
			['', 'kelos task', 'unknown', '   └─ fix', 'busy', '-', ''],
			['●', 'mirror', 'github', '-', 'error', '-', 'down']
		]);
	});

	test('classes each row by severity and marks the needs-you mark', async () => {
		await mountWith();
		const severities = ['quiet', 'attention', 'busy', 'failed'];
		expect(bodyRows().map((tr) => severities.filter((sev) => tr.classList.contains(sev)))).toEqual(
			severities.map((sev) => [sev])
		);
		expect(bodyRows().map((tr) => tr.cells[0]?.classList.contains('needs-you'))).toEqual([
			false,
			true,
			false,
			true
		]);
	});

	test('computes the ages from the clock, not the snapshot', async () => {
		page = render(App, {
			api,
			openStream: streams.open,
			timer: timers.timer,
			now: () => TEST_NOW + 3 * HOUR_MS
		});
		api.answer(BOARD);
		await settle();
		flushSync();
		expect(ages()).toEqual(['5h', '4h', '-', '-']);
	});

	test('the header time and the ages move on every tick', async () => {
		vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
		let clock = TEST_NOW;
		page = render(App, {
			api,
			openStream: streams.open,
			timer: timers.timer,
			now: () => clock
		});
		api.answer(BOARD);
		await settle();
		flushSync();
		clock = TEST_NOW + HOUR_MS;
		vi.advanceTimersByTime(TICK_MS - 1);
		flushSync();
		expect(headerText()).toContain(localTime(TEST_NOW));
		expect(ages()).toEqual(['2h', '1h', '-', '-']);
		vi.advanceTimersByTime(1);
		flushSync();
		expect(headerText()).toContain(localTime(TEST_NOW + HOUR_MS));
		expect(ages()).toEqual(['3h', '2h', '-', '-']);
	});

	test('names with a target are links that open a new tab; the others are not', async () => {
		await mountWith();
		const links = [...page.querySelectorAll<HTMLAnchorElement>('tbody a')];
		expect(links.map((a) => [a.textContent, a.getAttribute('href'), a.target, a.rel])).toEqual([
			['#7', TASK.target, '_blank', 'noopener noreferrer'],
			['vigie#3', PR.target, '_blank', 'noopener noreferrer']
		]);
		expect(bodyRows()[2]?.querySelector('a')).toBeNull();
	});

	test('says there are no rows once a refresh found none', async () => {
		await mountWith(snapshot([]));
		expect(bodyRows().map(cells)).toEqual([['', 'no rows']]);
	});

	test('says nothing about rows while the first refresh runs', async () => {
		await mountWith(snapshot([], { fetchedAt: undefined, refreshing: true }));
		expect(bodyRows()).toEqual([]);
		expect(statusText()).toBe('refreshing…');
	});

	test('shows the key help', async () => {
		await mountWith();
		expect(page.querySelector('footer .help')?.textContent).toBe(
			'↑/↓ j/k move · pgup/pgdn home/end · enter open · h hide · H show hidden · r refresh'
		);
	});
});

describe('moving', () => {
	test('arrows and j/k move and stop at the ends', async () => {
		await mountWith();
		expect(selectedName()).toBe('#7');
		await press('{ArrowDown}');
		expect(selectedName()).toBe('└─ vigie#3');
		await press('j');
		expect(selectedName()).toBe('└─ fix');
		await press('jjj');
		expect(selectedName()).toBe('-');
		await press('k');
		expect(selectedName()).toBe('└─ fix');
		await press('{ArrowUp}{ArrowUp}{ArrowUp}{ArrowUp}');
		expect(selectedName()).toBe('#7');
	});

	test('home and end go to the first and last rows', async () => {
		await mountWith();
		await press('{End}');
		expect(selectedName()).toBe('-');
		await press('{Home}');
		expect(selectedName()).toBe('#7');
	});

	test('pgdn and pgup move by the rows the table shows', async () => {
		const many = Array.from({ length: 200 }, (_, i) => row({ source: 'pr', name: `r${i}` }));
		await mountWith(snapshot(many));
		const viewport = page.querySelector<HTMLElement>('.viewport')!;
		const head = page.querySelector<HTMLElement>('thead')!;
		const first = bodyRows()[0]!;
		const pageSize = Math.floor((viewport.clientHeight - head.offsetHeight) / first.offsetHeight);
		expect(pageSize).toBeGreaterThan(1);
		await press('{PageDown}');
		expect(selectedName()).toBe(`r${pageSize}`);
		// A second page lands past the rows shown first: the table scrolls to it.
		await press('{PageDown}');
		expect(selectedName()).toBe(`r${2 * pageSize}`);
		// Rows are fractions of a pixel high: allow one pixel of rounding.
		const inView = (i: number) => {
			const shown = viewport.getBoundingClientRect();
			const r = bodyRows()[i]!.getBoundingClientRect();
			return r.top >= shown.top - PIXEL && r.bottom <= shown.bottom + PIXEL;
		};
		expect(inView(2 * pageSize)).toBe(true);
		expect(inView(0)).toBe(false);
		await press('{PageUp}{PageUp}');
		expect(selectedName()).toBe('r0');
		expect(inView(0)).toBe(true);
	});

	test('a click selects the row', async () => {
		await mountWith();
		bodyRows()[2]?.click();
		flushSync();
		expect(selectedName()).toBe('└─ fix');
	});

	test('focusing a link selects its row, so enter opens what has focus', async () => {
		await mountWith();
		page.querySelectorAll<HTMLAnchorElement>('tbody a')[1]?.focus();
		flushSync();
		expect(selectedName()).toBe('└─ vigie#3');
		await press('{Enter}');
		expect(opened).toEqual([PR.target]);
	});

	test('the selection follows its row across snapshots', async () => {
		await mountWith();
		await press('j');
		send(snapshot([AGENT, ERROR, TASK, PR]));
		expect(selectedName()).toBe('└─ vigie#3');
		expect(bodyRows()[3]?.getAttribute('aria-current')).toBe('true');
	});

	test('a row that left gives its place to the next one', async () => {
		await mountWith();
		await press('j');
		send(snapshot([TASK, AGENT, ERROR]));
		expect(selectedName()).toBe('└─ fix');
	});
});

describe('enter', () => {
	test("opens the selected row's target", async () => {
		await mountWith();
		await press('j{Enter}');
		expect(opened).toEqual([PR.target]);
		expect(statusText()).toBe(`opened ${PR.target}`);
	});

	test('says when the row has nothing to open', async () => {
		await mountWith();
		await press('jj{Enter}');
		expect(opened).toEqual([]);
		expect(statusText()).toBe('nothing to open');
	});

	test('says when the browser did not open it', async () => {
		openAnswer = false;
		await mountWith();
		await press('{Enter}');
		expect(statusText()).toBe(`error: the browser did not open ${TASK.target}`);
	});

	test('opens a new tab by default, cut off from this page', async () => {
		const tab = { opener: window } as unknown as Window;
		const windowOpen = vi.spyOn(window, 'open').mockReturnValue(tab);
		page = render(App, { api, openStream: streams.open, timer: timers.timer });
		api.answer(BOARD);
		await settle();
		streams.last.open();
		flushSync();
		await press('{Enter}');
		expect(windowOpen.mock.calls).toEqual([[TASK.target, '_blank']]);
		expect(tab.opener).toBeNull();
		expect(statusText()).toBe(`opened ${TASK.target}`);
	});

	test('by default, says when the browser blocked the tab', async () => {
		vi.spyOn(window, 'open').mockReturnValue(null);
		page = render(App, { api, openStream: streams.open, timer: timers.timer });
		api.answer(BOARD);
		await settle();
		streams.last.open();
		flushSync();
		await press('{Enter}');
		expect(statusText()).toBe(`error: the browser did not open ${TASK.target}`);
	});
});

describe('r', () => {
	test('refreshes and shows the refresh running over the previous rows', async () => {
		await mountWith();
		api.refreshAnswer = () => Promise.resolve({ ...BOARD, refreshing: true });
		await press('r');
		expect(api.calls.at(-1)).toBe('refresh false');
		expect(statusText()).toBe('refreshing…');
		expect(bodyRows()).toHaveLength(4);
		send(BOARD);
		expect(statusText()).toBe('');
	});

	test('refreshes the layout with the hidden rows while they are shown', async () => {
		await mountWith();
		await press('H');
		api.answer(snapshot([TASK]));
		await settle();
		streams.last.open();
		flushSync();
		api.refreshAnswer = () => Promise.resolve(snapshot([TASK, PR], { refreshing: true }));
		await press('r');
		expect(api.calls.at(-1)).toBe('refresh true');
		expect(statusText()).toBe('refreshing… · showing hidden rows (marked -)');
		expect(bodyRows()).toHaveLength(2);
	});

	test('says when the server refreshed too recently', async () => {
		await mountWith();
		api.refreshAnswer = () => Promise.reject(new TooSoonError('refreshed too recently', 4));
		await press('r');
		expect(statusText()).toBe('refreshed too recently · try again in 4s');
	});

	test('says when the refresh failed', async () => {
		await mountWith();
		api.refreshAnswer = () => Promise.reject(new ApiError(502, '502 Bad Gateway'));
		await press('r');
		expect(statusText()).toBe('error: refresh: 502 Bad Gateway');
	});
});

describe('hiding', () => {
	test('h hides the selected row', async () => {
		await mountWith();
		await press('jh');
		expect(api.calls.at(-1)).toBe('hide pr vigie#3');
		expect(statusText()).toBe('hid pr vigie#3 · H shows hidden rows');
	});

	test("h on a source's error row says the server's refusal", async () => {
		await mountWith();
		api.hiddenAnswer = () => Promise.reject(new ApiError(409, 'a source error cannot be hidden'));
		await press('{End}h');
		expect(statusText()).toBe('a source error cannot be hidden');
	});

	test('h says when the hidden rows cannot be stored', async () => {
		await mountWith();
		api.hiddenAnswer = () => Promise.reject(new ApiError(500, 'hidden rows: read-only'));
		await press('h');
		expect(statusText()).toBe('error: hidden rows: read-only');
	});

	test('H lists the hidden rows, where h unhides, and H again hides them', async () => {
		await mountWith();
		await press('H');
		expect(api.calls.at(-1)).toBe('snapshot true');
		const hiddenRow: Row = { ...PR, mark: '-', severity: 'quiet', hidden: true };
		api.answer(
			snapshot([TASK, hiddenRow], { counts: { needYou: 0, busy: 0, idle: 1, hidden: 1 } })
		);
		await settle();
		flushSync();
		expect(streams.last.url).toBe('/api/v1/events?show_hidden=true');
		expect(statusText()).toBe('connecting… · showing hidden rows (marked -)');
		streams.last.open();
		flushSync();
		expect(bodyRows()[1]?.cells[0]?.textContent).toBe('-');
		await press('jh');
		expect(api.calls.at(-1)).toBe('unhide pr vigie#3');
		expect(statusText()).toBe('showing hidden rows (marked -) · unhid pr vigie#3');
		await press('H');
		expect(api.calls.at(-1)).toBe('snapshot false');
		api.answer(BOARD);
		await settle();
		streams.last.open();
		flushSync();
		expect(statusText()).toBe('hidden rows are hidden again');
	});

	test("shows the server's last hidden-row error", async () => {
		await mountWith();
		send({ ...BOARD, error: 'hidden rows: read-only' });
		expect(statusText()).toBe('error: hidden rows: read-only');
	});
});

describe('the connection', () => {
	test('keeps the rows and says so while the stream reconnects', async () => {
		await mountWith();
		streams.last.drop();
		flushSync();
		expect(statusText()).toBe('connection lost, reconnecting…');
		expect(bodyRows()).toHaveLength(4);
		streams.last.open();
		flushSync();
		expect(statusText()).toBe('');
	});

	test('retries after the server closed the stream, keeping the rows', async () => {
		await mountWith();
		streams.last.fail();
		flushSync();
		expect(statusText()).toBe('error: the event stream closed · retrying in 1s');
		expect(bodyRows()).toHaveLength(4);
		timers.fire();
		api.answer(snapshot([TASK]));
		await settle();
		flushSync();
		expect(bodyRows()).toHaveLength(1);
	});

	test.each(['ctrlKey', 'metaKey', 'altKey'])(
		'leaves a key with %s to the browser',
		async (modifier) => {
			await mountWith();
			const event = new KeyboardEvent('keydown', { key: 'r', [modifier]: true, cancelable: true });
			window.dispatchEvent(event);
			await settle();
			expect(event.defaultPrevented).toBe(false);
			expect(api.calls).toEqual(['snapshot false']);
		}
	);

	test('takes a key without a modifier from the browser', async () => {
		await mountWith();
		const event = new KeyboardEvent('keydown', { key: 'r', cancelable: true });
		window.dispatchEvent(event);
		await settle();
		expect(event.defaultPrevented).toBe(true);
		expect(api.calls).toEqual(['snapshot false', 'refresh false']);
	});
});
