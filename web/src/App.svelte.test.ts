import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { userEvent } from 'vitest/browser';
import { flushSync } from 'svelte';
import App, { TICK_MS } from './App.svelte';
import { ApiError, TooSoonError } from './lib/api';
import type { Row, Snapshot } from './lib/model';
import { render } from './render';
import { AGENT, BOARD, ERROR, PR, TASK } from './testing/board';
import { HOUR_MS, row, snapshot, TEST_NOW } from './testing/builders';
import { FakeApi, FakeStreams, FakeTimers, settle } from './testing/fakes';

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
/** A cell's text, its blocks' texts apart. */
const cellText = (td: HTMLTableCellElement) =>
	[...td.querySelectorAll(':scope > div')].map((d) => d.textContent).join(' | ') ||
	(td.textContent ?? '').trim();
const cells = (tr: HTMLTableRowElement) => [...tr.cells].map(cellText);
const selectedName = () =>
	page.querySelector('tbody tr[aria-current="true"] .name')?.textContent?.trim();
const statusText = () => page.querySelector('[role="status"]')?.textContent?.trim() ?? '';
const badges = () => [...page.querySelectorAll('header .badge')].map((b) => b.textContent?.trim());
const badgeButton = (kind: 'need-you' | 'busy' | 'idle') =>
	page.querySelector<HTMLButtonElement>(`.badge.${kind} button`)!;
const updated = () => page.querySelector('header .updated')?.textContent ?? '';
/** One CSS pixel, the rounding allowed on measured positions. */
const PIXEL = 1;
const ages = () => bodyRows().map((tr) => tr.cells[4]?.textContent);

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
	test('draws the app bar, the columns and the rows as sent', async () => {
		await mountWith();
		expect(page.querySelector('header h1')?.textContent).toBe('vigie');
		expect(badges()).toEqual(['2 need you', '1 busy', '1 idle']);
		expect(updated()).toBe('updated 0s ago');
		expect(page.querySelector('header .updated')?.getAttribute('title')).toBe(localTime(TEST_NOW));
		expect([...page.querySelectorAll('thead th')].map((th) => th.textContent)).toEqual([
			'needs you',
			'Name',
			'Type',
			'State',
			'Age',
			'actions'
		]);
		expect(bodyRows().map(cells)).toEqual([
			['', '#7 | [claude] serve', 'vikunja | vikunja', 'idle', '2h', 'Hide'],
			['needs you', 'vigie#3 | Serve the board', 'pr | github', 'review', '1h', 'Hide'],
			['', 'fix', 'kelos task | unknown', 'busy', '-', 'Hide'],
			['needs you', '- | down', 'mirror | github', 'error', '-', '']
		]);
	});

	test('counts the hidden rows only when there are some', async () => {
		await mountWith(snapshot([TASK], { counts: { needYou: 0, busy: 0, idle: 1, hidden: 3 } }));
		expect(badges()).toEqual(['0 need you', '0 busy', '1 idle', '3 hidden']);
	});

	test('marks the badges whose count is not zero', async () => {
		await mountWith(snapshot([TASK], { counts: { needYou: 0, busy: 2, idle: 1, hidden: 0 } }));
		const active = [...page.querySelectorAll('header .badge')].map((b) =>
			b.classList.contains('active')
		);
		expect(active).toEqual([false, true, true]);
	});

	test('classes each row by severity and dots the rows that need you', async () => {
		await mountWith();
		const severities = ['quiet', 'attention', 'busy', 'failed'];
		expect(bodyRows().map((tr) => severities.filter((sev) => tr.classList.contains(sev)))).toEqual(
			severities.map((sev) => [sev])
		);
		expect(bodyRows().map((tr) => tr.cells[0]?.querySelector('.dot') !== null)).toEqual([
			false,
			true,
			false,
			true
		]);
	});

	test('indents each row by its depth in the tree', async () => {
		await mountWith();
		const items = bodyRows().map((tr) => tr.querySelector<HTMLElement>('.item')!);
		expect(items.map((td) => td.style.getPropertyValue('--depth'))).toEqual(['0', '1', '2', '0']);
		expect(items.map((td) => td.classList.contains('nested'))).toEqual([false, true, true, false]);
		const left = items.map((td) => parseFloat(getComputedStyle(td).paddingLeft));
		expect(left[1]).toBeGreaterThan(left[0]!);
		expect(left[2]).toBeGreaterThan(left[1]!);
		expect(left[3]).toBe(left[0]);
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
		expect(bodyRows()[0]?.cells[4]?.getAttribute('title')).toBe(localTime(TASK.since!));
		expect(bodyRows()[2]?.cells[4]?.hasAttribute('title')).toBe(false);
	});

	test('"updated … ago" and the ages move on every tick', async () => {
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
		expect(updated()).toBe('updated 0s ago');
		expect(ages()).toEqual(['2h', '1h', '-', '-']);
		vi.advanceTimersByTime(1);
		flushSync();
		expect(updated()).toBe('updated 1h ago');
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
		expect(bodyRows().map(cells)).toEqual([['Nothing to show.']]);
	});

	test('says nothing about rows while the first refresh runs', async () => {
		await mountWith(snapshot([], { fetchedAt: undefined, refreshing: true }));
		expect(bodyRows()).toEqual([]);
		expect(statusText()).toBe('refreshing…');
		expect(updated()).toBe('');
		expect(page.querySelector('header .updated')?.hasAttribute('title')).toBe(false);
	});

	test('shows the key help', async () => {
		await mountWith();
		const hints = [...page.querySelectorAll('footer .help li')].map((li) => [
			[...li.querySelectorAll('kbd')].map((k) => k.textContent),
			li.querySelector('.does')?.textContent
		]);
		expect(hints).toEqual([
			[['↑', '↓', 'j', 'k'], 'move'],
			[['PgUp', 'PgDn', 'Home', 'End'], 'jump'],
			[['Enter'], 'open'],
			[['h'], 'hide'],
			[['H'], 'show hidden'],
			[['r'], 'refresh']
		]);
	});
});

describe('filtering', () => {
	test('clicking a badge shows only the rows it counts, the counts unchanged', async () => {
		await mountWith();
		badgeButton('busy').click();
		flushSync();
		expect(
			bodyRows()
				.map(cells)
				.map((c) => c[1])
		).toEqual(['fix']);
		expect(badges()).toEqual(['2 need you', '1 busy', '1 idle']);
		expect(badgeButton('busy').getAttribute('aria-pressed')).toBe('true');
		expect(badgeButton('need-you').getAttribute('aria-pressed')).toBe('false');
		expect(statusText()).toBe('showing only busy');
	});

	test('clicking the active badge again clears the filter', async () => {
		await mountWith();
		badgeButton('idle').click();
		flushSync();
		expect(bodyRows()).toHaveLength(1);
		badgeButton('idle').click();
		flushSync();
		expect(bodyRows()).toHaveLength(4);
		expect(badgeButton('idle').getAttribute('aria-pressed')).toBe('false');
		expect(statusText()).toBe('');
	});

	test('clicking a different badge switches the filter', async () => {
		await mountWith();
		badgeButton('need-you').click();
		flushSync();
		expect(
			bodyRows()
				.map(cells)
				.map((c) => c[1])
		).toEqual(['vigie#3 | Serve the board', '- | down']);
		badgeButton('idle').click();
		flushSync();
		expect(
			bodyRows()
				.map(cells)
				.map((c) => c[1])
		).toEqual(['#7 | [claude] serve']);
		expect(badgeButton('need-you').getAttribute('aria-pressed')).toBe('false');
		expect(badgeButton('idle').getAttribute('aria-pressed')).toBe('true');
	});

	test('selects the first matching row when the selected one is filtered out', async () => {
		await mountWith();
		expect(selectedName()).toBe('#7');
		badgeButton('busy').click();
		flushSync();
		expect(selectedName()).toBe('fix');
	});

	test('a new snapshot keeps applying the active filter', async () => {
		await mountWith();
		badgeButton('idle').click();
		flushSync();
		send(snapshot([TASK, PR, AGENT], { counts: { needYou: 1, busy: 1, idle: 1, hidden: 0 } }));
		expect(
			bodyRows()
				.map(cells)
				.map((c) => c[1])
		).toEqual(['#7 | [claude] serve']);
	});
});

describe('moving', () => {
	test('arrows and j/k move and stop at the ends', async () => {
		await mountWith();
		expect(selectedName()).toBe('#7');
		await press('{ArrowDown}');
		expect(selectedName()).toBe('vigie#3');
		await press('j');
		expect(selectedName()).toBe('fix');
		await press('jjj');
		expect(selectedName()).toBe('-');
		await press('k');
		expect(selectedName()).toBe('fix');
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
		expect(selectedName()).toBe('fix');
	});

	test('focusing a link selects its row, so enter opens what has focus', async () => {
		await mountWith();
		page.querySelectorAll<HTMLAnchorElement>('tbody a')[1]?.focus();
		flushSync();
		expect(selectedName()).toBe('vigie#3');
		await press('{Enter}');
		expect(opened).toEqual([PR.target]);
	});

	test('the selection follows its row across snapshots', async () => {
		await mountWith();
		await press('j');
		send(snapshot([AGENT, ERROR, TASK, PR]));
		expect(selectedName()).toBe('vigie#3');
		expect(bodyRows()[3]?.getAttribute('aria-current')).toBe('true');
	});

	test('a row that left gives its place to the next one', async () => {
		await mountWith();
		await press('j');
		send(snapshot([TASK, AGENT, ERROR]));
		expect(selectedName()).toBe('fix');
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
		expect(statusText()).toBe('refreshing… · showing hidden rows');
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
		expect(statusText()).toBe('connecting… · showing hidden rows');
		streams.last.open();
		flushSync();
		expect(bodyRows().map((tr) => tr.querySelector('.name .tag')?.textContent)).toEqual([
			undefined,
			'hidden'
		]);
		expect(bodyRows()[1]?.querySelector('.actions button')?.textContent).toBe('Unhide');
		await press('jh');
		expect(api.calls.at(-1)).toBe('unhide pr vigie#3');
		expect(statusText()).toBe('showing hidden rows · unhid pr vigie#3');
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
