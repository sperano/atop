import { describe, expect, test } from 'vitest';
import type { Connection } from './live';
import { hidMessage, statusLine, tooSoonMessage, unhidMessage, type StatusInput } from './status';

const LIVE: Connection = { state: 'live' };
const quiet: StatusInput = {
	connection: LIVE,
	refreshing: false,
	showHidden: false,
	serverError: '',
	message: ''
};

describe('statusLine', () => {
	test.each<[string, Partial<StatusInput>, string]>([
		['nothing going on', {}, ''],
		['connecting', { connection: { state: 'connecting' } }, 'connecting…'],
		['reconnecting', { connection: { state: 'reconnecting' } }, 'connection lost, reconnecting…'],
		[
			'retrying, rounding the wait up',
			{ connection: { state: 'retrying', error: 'offline', retryInMs: 1500 } },
			'error: offline · retrying in 2s'
		],
		['refreshing', { refreshing: true }, 'refreshing…'],
		['showing hidden rows', { showHidden: true }, 'showing hidden rows (marked -)'],
		['server error', { serverError: 'hidden rows: denied' }, 'error: hidden rows: denied'],
		['message', { message: 'opened https://x' }, 'opened https://x'],
		[
			'all, in order',
			{
				connection: { state: 'reconnecting' },
				refreshing: true,
				showHidden: true,
				serverError: 'hidden rows: denied',
				message: 'hid pr a'
			},
			'connection lost, reconnecting… · refreshing… · showing hidden rows (marked -) · error: hidden rows: denied · hid pr a'
		]
	])('%s', (_name, fields, want) => {
		expect(statusLine({ ...quiet, ...fields })).toBe(want);
	});
});

describe('messages', () => {
	const pr = { source: 'pr', name: 'vigie#3' };

	test('hid', () => {
		expect(hidMessage(pr)).toBe('hid pr vigie#3 · H shows hidden rows');
	});

	test('unhid', () => {
		expect(unhidMessage(pr)).toBe('unhid pr vigie#3');
	});

	test.each([
		[7, 'refreshed too recently · try again in 7s'],
		[undefined, 'refreshed too recently']
	])('too soon, retry after %s', (retryAfter, want) => {
		expect(tooSoonMessage(retryAfter)).toBe(want);
	});
});
