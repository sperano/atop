import { describe, expect, test } from 'vitest';
import { row } from '../testing/builders';
import type { Move } from './keys';
import { clamp, moveTo, pageRows, reselect } from './selection';

describe('clamp', () => {
	test.each([
		[-1, 3, 0],
		[0, 3, 0],
		[2, 3, 2],
		[5, 3, 2],
		[4, 0, 0]
	])('clamp(%i, %i) = %i', (i, count, want) => {
		expect(clamp(i, count)).toBe(want);
	});
});

describe('moveTo', () => {
	const COUNT = 10;
	const PAGE = 4;
	test.each<[Move, number, number]>([
		['up', 5, 4],
		['up', 0, 0],
		['down', 5, 6],
		['down', 9, 9],
		['pageUp', 5, 1],
		['pageUp', 2, 0],
		['pageDown', 5, 9],
		['pageDown', 2, 6],
		['pageDown', 8, 9],
		['pageDown', 9, 9],
		['home', 7, 0],
		['end', 2, 9]
	])('%s from %i goes to %i', (move, from, want) => {
		expect(moveTo(from, move, COUNT, PAGE)).toBe(want);
	});

	test.each<Move>(['up', 'down', 'pageUp', 'pageDown', 'home', 'end'])(
		'%s without rows stays at 0',
		(move) => {
			expect(moveTo(0, move, 0, PAGE)).toBe(0);
		}
	);
});

describe('reselect', () => {
	const a = row({ source: 'pr', name: 'a' });
	const b = row({ source: 'pr', name: 'b' });
	const c = row({ source: 'vikunja', name: 'b' });

	test('follows the row to its new place', () => {
		expect(reselect(b, 1, [c, a, { ...b, state: 'review' }])).toBe(2);
	});

	test('tells rows apart by source and name', () => {
		expect(reselect(c, 0, [b, c])).toBe(1);
	});

	test('a row that left gives its place to the next', () => {
		expect(reselect(b, 1, [a, c])).toBe(1);
	});

	test('clamps the place when the rows shrink', () => {
		expect(reselect(b, 5, [a])).toBe(0);
	});

	test('keeps the place without a previous row', () => {
		expect(reselect(undefined, 1, [a, b, c])).toBe(1);
	});
});

describe('pageRows', () => {
	test.each([
		[400, 20, 20],
		[410, 20, 20],
		[10, 20, 1],
		[400, 0, 1],
		[0, 20, 1]
	])('%ipx of %ipx rows is %i', (viewport, rowHeight, want) => {
		expect(pageRows(viewport, rowHeight)).toBe(want);
	});
});
