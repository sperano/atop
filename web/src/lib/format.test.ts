import { describe, expect, test } from 'vitest';
import { formatAge, formatCounts, formatTime, titleLine } from './format';

const NOW = Date.UTC(2026, 9, 5, 12, 0, 0);
const S = 1000;
const M = 60 * S;
const H = 60 * M;

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

describe('formatCounts', () => {
	test('without hidden rows', () => {
		expect(formatCounts({ needYou: 2, busy: 3, idle: 1, hidden: 0 })).toBe(
			'2 need you · 3 busy · 1 idle'
		);
	});

	test('with hidden rows', () => {
		expect(formatCounts({ needYou: 0, busy: 0, idle: 0, hidden: 4 })).toBe(
			'0 need you · 0 busy · 0 idle · 4 hidden'
		);
	});
});

describe('formatTime', () => {
	test('is local time in the TUI layout', () => {
		// The node project runs in UTC unless TZ says otherwise: build the
		// expectation from local fields.
		const local = new Date(2026, 0, 2, 3, 4, 5).getTime();
		expect(formatTime(local)).toBe('2026-01-02 03:04:05');
	});
});

describe('titleLine', () => {
	test('is name, time and counts', () => {
		const local = new Date(2026, 10, 15, 21, 55, 4).getTime();
		expect(titleLine({ needYou: 2, busy: 3, idle: 2, hidden: 0 }, local)).toBe(
			'vigie · 2026-11-15 21:55:04 · 2 need you · 3 busy · 2 idle'
		);
	});
});
