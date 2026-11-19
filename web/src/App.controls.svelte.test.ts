import { beforeEach, describe, expect, test } from 'vitest';
import { userEvent } from 'vitest/browser';
import { flushSync } from 'svelte';
import App from './App.svelte';
import { ApiError } from './lib/api';
import type { Snapshot } from './lib/model';
import { render } from './render';
import { BOARD, PR, TASK } from './testing/board';
import { snapshot, TEST_NOW } from './testing/builders';
import { FakeApi, FakeStreams, FakeTimers, settle } from './testing/fakes';

let api: FakeApi;
let streams: FakeStreams;
let opened: string[];
let page: HTMLElement;

beforeEach(() => {
	api = new FakeApi();
	streams = new FakeStreams();
	opened = [];
});

async function mountWith(first: Snapshot = BOARD): Promise<void> {
	page = render(App, {
		api,
		openStream: streams.open,
		timer: new FakeTimers().timer,
		now: () => TEST_NOW,
		openUrl: (url: string) => {
			opened.push(url);
			return true;
		}
	});
	api.answer(first);
	await settle();
	streams.last.open();
	flushSync();
}

async function click(element: HTMLElement | null | undefined): Promise<void> {
	if (!element) throw new Error('nothing to click');
	element.click();
	await settle();
	flushSync();
}

const button = (name: string) =>
	[...page.querySelectorAll<HTMLButtonElement>('header button')].find(
		(b) => b.textContent?.trim() === name
	);
const rowButton = (i: number) =>
	page.querySelectorAll('tbody tr')[i]?.querySelector<HTMLButtonElement>('.actions button');
const selectedName = () =>
	page.querySelector('tbody tr[aria-current="true"] .name')?.textContent?.trim();
const status = () => page.querySelector<HTMLElement>('[role="status"]')!;

describe('the app bar', () => {
	test('Refresh refreshes, and says so while the refresh runs', async () => {
		await mountWith();
		api.refreshAnswer = () => Promise.resolve({ ...BOARD, refreshing: true });
		await click(button('Refresh'));
		expect(api.calls.at(-1)).toBe('refresh false false');
		expect(button('Refreshing…')?.disabled).toBe(true);
		streams.last.send(BOARD);
		flushSync();
		expect(button('Refresh')?.disabled).toBe(false);
	});

	test('Show hidden is pressed while the hidden rows are listed', async () => {
		await mountWith();
		const toggle = button('Show hidden')!;
		expect(toggle.getAttribute('aria-pressed')).toBe('false');
		await click(toggle);
		expect(api.calls.at(-1)).toBe('snapshot true false');
		expect(toggle.getAttribute('aria-pressed')).toBe('true');
		await click(toggle);
		expect(api.calls.at(-1)).toBe('snapshot false false');
		expect(toggle.getAttribute('aria-pressed')).toBe('false');
	});

	test('Hide done is pressed while done rows are dropped', async () => {
		await mountWith();
		const toggle = button('Hide done')!;
		expect(toggle.getAttribute('aria-pressed')).toBe('false');
		await click(toggle);
		expect(api.calls.at(-1)).toBe('snapshot false true');
		expect(toggle.getAttribute('aria-pressed')).toBe('true');
		expect(status().textContent?.trim()).toBe('connecting… · hiding done tasks');
		api.answer(BOARD);
		await settle();
		streams.last.open();
		flushSync();
		await click(toggle);
		expect(api.calls.at(-1)).toBe('snapshot false false');
		expect(toggle.getAttribute('aria-pressed')).toBe('false');
		expect(status().textContent?.trim()).toBe('connecting… · showing done tasks again');
	});

	test('d toggles hide done the same way the button does', async () => {
		await mountWith();
		await userEvent.keyboard('d');
		await settle();
		flushSync();
		expect(api.calls.at(-1)).toBe('snapshot false true');
		expect(button('Hide done')?.getAttribute('aria-pressed')).toBe('true');
		expect(status().textContent?.trim()).toBe('connecting… · hiding done tasks');
	});

	test('Enter on a focused button presses it rather than opening the selected row', async () => {
		await mountWith();
		button('Refresh')?.focus();
		await userEvent.keyboard('{Enter}');
		await settle();
		expect(api.calls.at(-1)).toBe('refresh false false');
		expect(opened).toEqual([]);
	});
});

describe('the row buttons', () => {
	test('Hide hides its own row and selects it', async () => {
		await mountWith();
		expect(selectedName()).toBe(TASK.name);
		await click(rowButton(1));
		expect(api.calls.at(-1)).toBe('hide pr vigie#3');
		expect(selectedName()).toBe(PR.name);
		expect(status().textContent?.trim()).toBe('hid pr vigie#3 · H shows hidden rows');
	});

	test('Unhide unhides a hidden row', async () => {
		await mountWith(snapshot([TASK, { ...PR, mark: '-', severity: 'quiet', hidden: true }]));
		expect(rowButton(1)?.textContent).toBe('Unhide');
		await click(rowButton(1));
		expect(api.calls.at(-1)).toBe('unhide pr vigie#3');
	});

	test("a source's error row has none", async () => {
		await mountWith();
		expect(rowButton(3)).toBeNull();
	});
});

describe('the page', () => {
	test('the tab title counts the rows that need you', async () => {
		await mountWith();
		expect(document.title).toBe('(2) vigie');
		streams.last.send(snapshot([TASK]));
		flushSync();
		expect(document.title).toBe('vigie');
	});

	test('the status line is red on an error only', async () => {
		await mountWith();
		await userEvent.keyboard('{Enter}');
		flushSync();
		expect(status().classList.contains('error')).toBe(false);
		api.refreshAnswer = () => Promise.reject(new ApiError(502, '502 Bad Gateway'));
		await click(button('Refresh'));
		expect(status().textContent?.trim()).toBe('error: refresh: 502 Bad Gateway');
		expect(status().classList.contains('error')).toBe(true);
	});
});
