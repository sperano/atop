import { describe, expect, test } from 'vitest';
import { colorViolations, NAMED_COLORS } from './colors.mjs';

/** The flagged texts of source. */
const flagged = (source, extension) => colorViolations(source, extension).map((v) => v.text);

describe('colorViolations', () => {
	test('lists all 148 named colours of CSS Color 4', () => {
		expect(NAMED_COLORS.size).toBe(148);
	});

	test.each([
		['a hex colour anywhere', 'const c = "#fff";', '.ts', ['#fff']],
		['a colour function in a style file', 'a { color: rgb(0 0 0); }', '.css', ['rgb(']],
		['a colour function in a test', 'expect(c).toBe("rgb(0, 0, 0)");', '.ts', []],
		['a named colour in a declaration', 'a { color: White; }', '.css', ['White']],
		['a named colour among other values', 'a { border: 1px solid red; }', '.css', ['red']],
		[
			'a named colour in a style block',
			'<p>red</p><style>p { background: navy; }</style>',
			'.svelte',
			['navy']
		],
		['a named colour in a style attribute', '<p style="color: teal">x</p>', '.html', ['teal']],
		['a named colour in a style: directive', '<p style:color="tan">x</p>', '.svelte', ['tan']],
		['a word of the markup', '<p class="red">red or white</p>', '.svelte', []],
		['a word of a script', "const state = 'red';", '.ts', []],
		['a class or id selector', '.red, #white { color: var(--color-red); }', '.css', []],
		['a CSS comment', 'a { /* not black */ color: var(--color-text); }', '.css', []],
		['a system colour', 'a { color: CanvasText; }', '.css', []]
	])('%s', (_name, source, extension, want) => {
		expect(flagged(source, extension)).toEqual(want);
	});

	test('gives each colour its offset in the file', () => {
		const source = '<p>x</p>\n<style>\n\tp { color: black; }\n</style>';
		expect(colorViolations(source, '.svelte')).toEqual([
			{ index: source.indexOf('black'), text: 'black' }
		]);
	});
});
