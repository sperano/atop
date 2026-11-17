import { afterEach, expect, test } from 'vitest';
import { commands } from 'vitest/browser';
import { flushSync } from 'svelte';
import App from './App.svelte';
import { contrast, GRAPHIC_CONTRAST, parseColor, TEXT_CONTRAST } from './lib/contrast';
import type { Severity } from './lib/model';
import { render } from './render';
import { row, snapshot, TEST_NOW } from './testing/builders';
import { FakeApi, FakeStreams, FakeTimers, settle } from './testing/fakes';

const SCHEMES = ['light', 'dark'] as const;

afterEach(async () => {
	await commands.emulateColorScheme(null);
});

/** A colour as the page computes it. */
function computed(value: string): string {
	const probe = document.createElement('div');
	probe.style.setProperty('color', value);
	document.body.append(probe);
	try {
		return getComputedStyle(probe).color;
	} finally {
		probe.remove();
	}
}

const token = (name: string) => `var(--color-${name})`;

function ratio(foreground: string, background: string): number {
	return contrast(parseColor(computed(token(foreground))), parseColor(computed(token(background))));
}

const SEVERITIES: readonly Severity[] = ['failed', 'blocked', 'busy', 'attention', 'quiet'];

test.each(SCHEMES)('state pills take their severity colours in the %s scheme', async (scheme) => {
	await commands.emulateColorScheme(scheme);
	const api = new FakeApi();
	const page = render(App, {
		api,
		openStream: new FakeStreams().open,
		timer: new FakeTimers().timer,
		now: () => TEST_NOW
	});
	api.answer(
		snapshot(
			SEVERITIES.map((severity) =>
				row({ source: 'pr', name: severity, severity, mark: severity === 'attention' ? '●' : '' })
			)
		)
	);
	await settle();
	flushSync();
	const pills = [...page.querySelectorAll('tbody .pill')].map((p) => getComputedStyle(p));
	expect(pills.map((p) => [p.color, p.backgroundColor])).toEqual(
		SEVERITIES.map((sev) => [computed(token(sev)), computed(token(`${sev}-bg`))])
	);
	const dot = page.querySelector('tbody .dot');
	expect(dot && getComputedStyle(dot).backgroundColor).toBe(computed(token('mark')));
	expect(getComputedStyle(document.documentElement).colorScheme).toBe('light dark');
	expect(getComputedStyle(document.body).backgroundColor).toBe(computed(token('page')));
	const card = page.querySelector('.viewport');
	expect(card && getComputedStyle(card).backgroundColor).toBe(computed(token('surface')));
	const first = page.querySelector('tbody tr');
	expect(first && getComputedStyle(first).backgroundColor).toBe(computed(token('selected')));
	const header = page.querySelector('header');
	expect(header && getComputedStyle(header).backgroundColor).toBe(computed(token('header')));
});

test.each(SCHEMES)('the %s scheme differs from the other', async (scheme) => {
	await commands.emulateColorScheme(scheme);
	const page = computed(token('page'));
	await commands.emulateColorScheme(scheme === 'light' ? 'dark' : 'light');
	expect(computed(token('page'))).not.toBe(page);
});

/** What the board's rows are drawn on: the card, a hovered and the selected row. */
const ROW_BACKGROUNDS = ['surface', 'hover', 'selected'];

/** Text and what it is read on. */
const TEXT_PAIRS: [text: string, background: string][] = [
	...['fg', 'muted'].flatMap((t): [string, string][] =>
		[...ROW_BACKGROUNDS, 'page'].map((bg) => [t, bg])
	),
	...SEVERITIES.map((sev): [string, string] => [sev, `${sev}-bg`]),
	['failed', 'surface'],
	...['header', 'badge', 'badge-need-you', 'badge-busy'].map((bg): [string, string] => [
		'on-header',
		bg
	]),
	['header-muted', 'header']
];
/** Marks, borders of controls and the focus rings. */
const GRAPHIC_PAIRS: [graphic: string, background: string][] = [
	...['mark', 'focus', 'control-border'].flatMap((g): [string, string][] =>
		ROW_BACKGROUNDS.map((bg) => [g, bg])
	),
	['accent', 'selected'],
	['focus', 'page'],
	['focus-on-header', 'header'],
	['focus-on-header', 'badge'],
	['header-muted', 'header']
];

test.each(SCHEMES)('the %s scheme meets WCAG AA contrast', async (scheme) => {
	await commands.emulateColorScheme(scheme);
	for (const [fg, bg] of TEXT_PAIRS) {
		expect(ratio(fg, bg), `${fg} on ${bg}`).toBeGreaterThanOrEqual(TEXT_CONTRAST);
	}
	for (const [fg, bg] of GRAPHIC_PAIRS) {
		expect(ratio(fg, bg), `${fg} on ${bg}`).toBeGreaterThanOrEqual(GRAPHIC_CONTRAST);
	}
});
