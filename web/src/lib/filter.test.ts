import { describe, expect, test } from 'vitest';
import { row } from '../testing/builders';
import type { Row } from './model';
import { filterRows, matchesFilter, type FilterKind } from './filter';

const needsYou = row({ source: 'pr', name: 'a', state: 'review', needsYou: true });
const busyAgent = row({ source: 'kelos task', name: 'b', state: 'busy', busy: true });
const busyNonAgent = row({ source: 'pr', name: 'c', state: 'busy', busy: true });
const busyAndNeedsYou = row({
	source: 'kelos task',
	name: 'd',
	state: 'busy',
	busy: true,
	needsYou: true
});
const idle = row({ source: 'vikunja', name: 'e', state: 'idle' });
const quiet = row({ source: 'pr', name: 'f', state: 'review' });

describe('matchesFilter', () => {
	test.each<[FilterKind, Row, boolean]>([
		['need-you', needsYou, true],
		['need-you', quiet, false],
		['busy', busyAgent, true],
		['busy', busyNonAgent, false],
		['busy', busyAndNeedsYou, false],
		['idle', idle, true],
		['idle', quiet, false]
	])('%s on a matching row', (filter, candidate, want) => {
		expect(matchesFilter(candidate, filter)).toBe(want);
	});
});

describe('filterRows', () => {
	const rows = [needsYou, busyAgent, busyNonAgent, busyAndNeedsYou, idle, quiet];

	test('keeps every row without a filter', () => {
		expect(filterRows(rows, undefined)).toBe(rows);
	});

	test('keeps only the rows a kind counts', () => {
		expect(filterRows(rows, 'need-you')).toEqual([needsYou, busyAndNeedsYou]);
		expect(filterRows(rows, 'busy')).toEqual([busyAgent]);
		expect(filterRows(rows, 'idle')).toEqual([idle]);
	});
});
