import { describe, expect, test } from 'vitest';
import { actionOf, type KeyInput } from './keys';

const key = (k: string, mods: Partial<KeyInput> = {}): KeyInput => ({
	key: k,
	ctrlKey: false,
	metaKey: false,
	altKey: false,
	isComposing: false,
	...mods
});

describe('actionOf', () => {
	test.each([
		['ArrowUp', { kind: 'move', to: 'up' }],
		['k', { kind: 'move', to: 'up' }],
		['ArrowDown', { kind: 'move', to: 'down' }],
		['j', { kind: 'move', to: 'down' }],
		['PageUp', { kind: 'move', to: 'pageUp' }],
		['PageDown', { kind: 'move', to: 'pageDown' }],
		['Home', { kind: 'move', to: 'home' }],
		['End', { kind: 'move', to: 'end' }],
		['Enter', { kind: 'open' }],
		['r', { kind: 'refresh' }],
		['h', { kind: 'hide' }],
		['H', { kind: 'showHidden' }],
		['d', { kind: 'hideDone' }]
	])('%s', (k, want) => {
		expect(actionOf(key(k))).toEqual(want);
	});

	test.each(['q', 'R', 'K', 'Escape', ' ', 'x'])('%s does nothing', (k) => {
		expect(actionOf(key(k))).toBeUndefined();
	});

	test.each([
		['ctrl', { ctrlKey: true }],
		['meta', { metaKey: true }],
		['alt', { altKey: true }],
		['composition', { isComposing: true }]
	])('leaves r with %s to the browser', (_name, mods) => {
		expect(actionOf(key('r', mods))).toBeUndefined();
	});
});
