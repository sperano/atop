import { describe, expect, test } from 'vitest';
import { TEST_NOW } from '../testing/builders';
import { documentTitle, formatAge, formatTime, updatedText } from './format';

const NOW = Date.UTC(2026, 9, 5, 12, 0, 0);
const S = 1000;
const M = 60 * S;
const H = 60 * M;
/** getTimezoneOffset in Montreal on standard time (UTC-5). */
const MONTREAL_WINTER_OFFSET_MIN = 5 * 60;

describe('formatAge', () => {
	// internal/board/age_test.go's cases.
	test.each([
		['unknown', undefined, '-'],
		['zero', NOW, '0s'],
		['future clamps to zero', NOW + H, '0s'],
		['59 seconds', NOW - 59 * S, '59s'],
		['one minute', NOW - M, '1m'],
		['90 minutes is 1h', NOW - 90 * M, '1h'],
		['23 hours', NOW - 23 * H, '23h'],
		['3d5h is 3d', NOW - (3 * 24 + 5) * H, '3d']
	])('%s', (_name, since, want) => {
		expect(formatAge(since, NOW)).toBe(want);
	});
});

describe('formatTime', () => {
	test('runs in a zone that is not UTC', () => {
		// vite.config.ts sets TZ for the node project; in UTC the next tests
		// could not tell local time from UTC.
		expect(new Date(TEST_NOW).getTimezoneOffset()).toBe(MONTREAL_WINTER_OFFSET_MIN);
	});

	test('is local time in the TUI layout', () => {
		// TEST_NOW is 12:00:00 UTC, 07:00:00 in Montreal.
		expect(formatTime(TEST_NOW + 4 * S)).toBe('2026-11-15 07:00:04');
	});
});

describe('updatedText', () => {
	test.each([
		['before the first refresh', undefined, ''],
		['just now', NOW, 'updated 0s ago'],
		['two minutes ago', NOW - 2 * M, 'updated 2m ago']
	])('%s', (_name, fetchedAt, want) => {
		expect(updatedText(fetchedAt, NOW)).toBe(want);
	});
});

describe('documentTitle', () => {
	test.each([
		['nobody needs you', { needYou: 0, busy: 3, idle: 2, hidden: 1 }, 'vigie'],
		['two rows need you', { needYou: 2, busy: 0, idle: 0, hidden: 0 }, '(2) vigie']
	])('%s', (_name, counts, want) => {
		expect(documentTitle(counts)).toBe(want);
	});
});
