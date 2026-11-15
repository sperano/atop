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

/** Each severity's row colour, as the table draws it. */
const ROW_COLORS: Record<Severity, string> = {
	failed: 'failed',
	blocked: 'blocked',
	busy: 'busy',
	attention: 'fg',
	quiet: 'muted'
};

test.each(SCHEMES)('rows take their severity colour in the %s scheme', async (scheme) => {
	await commands.emulateColorScheme(scheme);
	const api = new FakeApi();
	const page = render(App, {
		api,
		openStream: new FakeStreams().open,
		timer: new FakeTimers().timer,
		now: () => TEST_NOW
	});
	const severities = Object.keys(ROW_COLORS) as Severity[];
	api.answer(
		snapshot(
			severities.map((severity) =>
				row({ source: 'pr', name: severity, severity, mark: severity === 'attention' ? '●' : '' })
			)
		)
	);
	await settle();
	flushSync();
	const rows = [...page.querySelectorAll('tbody tr')];
	expect(rows.map((tr) => getComputedStyle(tr).color)).toEqual(
		severities.map((sev) => computed(token(ROW_COLORS[sev])))
	);
	const mark = page.querySelector('tbody td.needs-you');
	expect(mark && getComputedStyle(mark).color).toBe(computed(token('mark')));
	expect(getComputedStyle(document.documentElement).colorScheme).toBe('light dark');
	expect(getComputedStyle(document.body).backgroundColor).toBe(computed(token('page')));
	expect(rows[0] && getComputedStyle(rows[0]).backgroundColor).toBe(computed(token('selected')));
});

test.each(SCHEMES)('the %s scheme differs from the other', async (scheme) => {
	await commands.emulateColorScheme(scheme);
	const page = computed(token('page'));
	await commands.emulateColorScheme(scheme === 'light' ? 'dark' : 'light');
	expect(computed(token('page'))).not.toBe(page);
});

/** Text and what it is read on. */
const TEXT_PAIRS: [text: string, background: string][] = [
	...['fg', 'muted', 'failed', 'blocked', 'busy'].flatMap((t): [string, string][] => [
		[t, 'page'],
		[t, 'selected']
	]),
	['on-header', 'header']
];
/** Marks and the focus ring. */
const GRAPHIC_PAIRS: [graphic: string, background: string][] = [
	['mark', 'page'],
	['mark', 'selected'],
	['focus', 'page'],
	['focus', 'selected']
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
