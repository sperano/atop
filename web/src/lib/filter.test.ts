import { describe, expect, test } from 'vitest';
import { row } from '../testing/builders';
import type { Row } from './model';
import { filterRows, matchesFilter, type FilterKind } from './filter';

const needsYou = row({ source: 'pr', name: 'a', state: 'review', needsYou: true });
const busyAgent = row({ source: 'kelos task', name: 'b', state: 'busy', busy: true });
const busyAgentLocalClaude = row({ source: 'local claude', name: 'b2', state: 'busy', busy: true });
const busyAgentKelosSession = row({
	source: 'kelos session',
	name: 'b3',
	state: 'busy',
	busy: true
});
const busyNonAgent = row({ source: 'pr', name: 'c', state: 'busy', busy: true });
const busyAndNeedsYou = row({
	source: 'kelos task',
	name: 'd',
	state: 'busy',
	busy: true,
	needsYou: true
});
const idle = row({ source: 'vikunja', name: 'e', state: 'idle' });
const idleAndNeedsYou = row({ source: 'vikunja', name: 'e2', state: 'idle', needsYou: true });
const quiet = row({ source: 'pr', name: 'f', state: 'review' });
const hiddenNeedsYou = row({
	source: 'pr',
	name: 'g',
	state: 'review',
	needsYou: true,
	hidden: true
});
const hiddenBusyAgent = row({
	source: 'kelos task',
	name: 'h',
	state: 'busy',
	busy: true,
	hidden: true
});
const hiddenIdle = row({ source: 'vikunja', name: 'i', state: 'idle', hidden: true });

describe('matchesFilter', () => {
	test.each<[FilterKind, Row, boolean]>([
		['need-you', needsYou, true],
		['need-you', quiet, false],
		['need-you', hiddenNeedsYou, false],
		['busy', busyAgent, true],
		['busy', busyAgentLocalClaude, true],
		['busy', busyAgentKelosSession, true],
		['busy', busyNonAgent, false],
		['busy', busyAndNeedsYou, false],
		['busy', hiddenBusyAgent, false],
		['idle', idle, true],
		['idle', idleAndNeedsYou, true],
		['idle', quiet, false],
		['idle', hiddenIdle, false]
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

	test('flattens a kept row whose parent was filtered out', () => {
		const parent = row({ source: 'kelos task', name: 'm', state: 'busy', busy: true });
		const child = row({
			source: 'pr',
			name: 'm#1',
			state: 'review',
			needsYou: true,
			depth: 1,
			guides: '└─ '
		});
		expect(filterRows([parent, child], 'need-you')).toEqual([{ ...child, depth: 0, guides: '' }]);
	});
});
