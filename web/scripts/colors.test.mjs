import { describe, expect, test } from 'vitest';
import { colorViolations, NAMED_COLORS } from './colors.mjs';

// CSS Color 4's named colours, as its table lists them (§6.1).
const SPEC_NAMED_COLORS = [
	'aliceblue',
	'antiquewhite',
	'aqua',
	'aquamarine',
	'azure',
	'beige',
	'bisque',
	'black',
	'blanchedalmond',
	'blue',
	'blueviolet',
	'brown',
	'burlywood',
	'cadetblue',
	'chartreuse',
	'chocolate',
	'coral',
	'cornflowerblue',
	'cornsilk',
	'crimson',
	'cyan',
	'darkblue',
	'darkcyan',
	'darkgoldenrod',
	'darkgray',
	'darkgreen',
	'darkgrey',
	'darkkhaki',
	'darkmagenta',
	'darkolivegreen',
	'darkorange',
	'darkorchid',
	'darkred',
	'darksalmon',
	'darkseagreen',
	'darkslateblue',
	'darkslategray',
	'darkslategrey',
	'darkturquoise',
	'darkviolet',
	'deeppink',
	'deepskyblue',
	'dimgray',
	'dimgrey',
	'dodgerblue',
	'firebrick',
	'floralwhite',
	'forestgreen',
	'fuchsia',
	'gainsboro',
	'ghostwhite',
	'gold',
	'goldenrod',
	'gray',
	'green',
	'greenyellow',
	'grey',
	'honeydew',
	'hotpink',
	'indianred',
	'indigo',
	'ivory',
	'khaki',
	'lavender',
	'lavenderblush',
	'lawngreen',
	'lemonchiffon',
	'lightblue',
	'lightcoral',
	'lightcyan',
	'lightgoldenrodyellow',
	'lightgray',
	'lightgreen',
	'lightgrey',
	'lightpink',
	'lightsalmon',
	'lightseagreen',
	'lightskyblue',
	'lightslategray',
	'lightslategrey',
	'lightsteelblue',
	'lightyellow',
	'lime',
	'limegreen',
	'linen',
	'magenta',
	'maroon',
	'mediumaquamarine',
	'mediumblue',
	'mediumorchid',
	'mediumpurple',
	'mediumseagreen',
	'mediumslateblue',
	'mediumspringgreen',
	'mediumturquoise',
	'mediumvioletred',
	'midnightblue',
	'mintcream',
	'mistyrose',
	'moccasin',
	'navajowhite',
	'navy',
	'oldlace',
	'olive',
	'olivedrab',
	'orange',
	'orangered',
	'orchid',
	'palegoldenrod',
	'palegreen',
	'paleturquoise',
	'palevioletred',
	'papayawhip',
	'peachpuff',
	'peru',
	'pink',
	'plum',
	'powderblue',
	'purple',
	'rebeccapurple',
	'red',
	'rosybrown',
	'royalblue',
	'saddlebrown',
	'salmon',
	'sandybrown',
	'seagreen',
	'seashell',
	'sienna',
	'silver',
	'skyblue',
	'slateblue',
	'slategray',
	'slategrey',
	'snow',
	'springgreen',
	'steelblue',
	'tan',
	'teal',
	'thistle',
	'tomato',
	'turquoise',
	'violet',
	'wheat',
	'white',
	'whitesmoke',
	'yellow',
	'yellowgreen'
];

/** The flagged texts of source. */
const flagged = (source, extension) => colorViolations(source, extension).map((v) => v.text);

describe('colorViolations', () => {
	test('lists exactly the named colours of CSS Color 4', () => {
		expect([...NAMED_COLORS].sort()).toEqual(SPEC_NAMED_COLORS);
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
		[
			'a named colour in a style attribute spanning lines',
			'<p style="border: 1px solid\n\tteal">x</p>',
			'.html',
			['teal']
		],
		[
			'a named colour in a style: directive spanning lines',
			'<p style:color="\n\ttan">x</p>',
			'.svelte',
			['tan']
		],
		['a style attribute expression', '<p style={"color: red"}>x</p>', '.svelte', ['red']],
		[
			'a style: directive expression',
			"<p style:color={on ? 'red' : 'gray'}>x</p>",
			'.svelte',
			['red', 'gray']
		],
		[
			'an expression in a style attribute',
			'<p style="width: 1px; color: {on ? \'red\' : `navy`}">x</p>',
			'.svelte',
			['red', 'navy']
		],
		[
			'an expression in a style: directive',
			'<p style:color="{on ? \'red\' : c}">x</p>',
			'.svelte',
			['red']
		],
		[
			'a same-quote string in a style attribute expression',
			'<p style="color: {on ? "red" : "blue"}">x</p>',
			'.svelte',
			['red', 'blue']
		],
		[
			'a same-quote string in a style: directive expression',
			"<p style:color='{on ? 'red' : c}'>x</p>",
			'.svelte',
			['red']
		],
		[
			'a style attribute around a nested-brace expression',
			'<p style="color: {f({a: 1})}; background: white">x</p>',
			'.svelte',
			['white']
		],
		[
			'a style: directive around a nested-brace expression',
			'<p style:border="{f({a: 1})} solid tan">x</p>',
			'.svelte',
			['tan']
		],
		[
			'the strings of a nested-brace expression in a style attribute',
			"<p style=\"color: {f({a: 1}) ? 'red' : 'blue'}\">x</p>",
			'.svelte',
			['red', 'blue']
		],
		[
			'the strings of a nested-brace expression in a style: directive',
			'<p style:color="{f({a: 1}) ? \'red\' : c}">x</p>',
			'.svelte',
			['red']
		],
		[
			'a nested-brace style attribute expression',
			"<p style={f({a: 1}) ? 'color: red' : ''}>x</p>",
			'.svelte',
			['red']
		],
		[
			'a nested-brace style: directive expression',
			"<p style:color={f({a: 1}) ? 'red' : 'gray'}>x</p>",
			'.svelte',
			['red', 'gray']
		],
		[
			'a string of an inner {…} once',
			'<p style="color: {f({c: \'red\'})}">x</p>',
			'.svelte',
			['red']
		],
		[
			'a value around an expression with a brace in a string',
			"<p style=\"color: {on ? '}' : 'red'}; background: white\">x</p>",
			'.svelte',
			['red', 'white']
		],
		[
			'a backtick string holding a brace in a style attribute expression',
			'<p style="color: {on ? `}` : \'red\'}">x</p>',
			'.svelte',
			['red']
		],
		[
			'an escaped quote in a style attribute expression',
			"<p style={on ? 'a\\'}' : 'color: red'}>x</p>",
			'.svelte',
			['red']
		],
		[
			'a quote in a line comment of a style attribute expression',
			"<p style={\n\t// don't\n\ton ? 'color: red' : 'color: blue'\n}>x</p>",
			'.svelte',
			['red', 'blue']
		],
		[
			'a quote in a block comment of a style: directive expression',
			"<p style:color={/* don't */ on ? 'red' : 'blue'}>x</p>",
			'.svelte',
			['red', 'blue']
		],
		[
			'a brace in a comment of an expression in a style attribute',
			'<p style="color: {on /* } */ ? \'red\' : c}; background: white">x</p>',
			'.svelte',
			['red', 'white']
		],
		[
			'two expressions in a style attribute',
			"<p style=\"color: {on ? 'red' : c}; background: {off ? 'navy' : d}\">x</p>",
			'.svelte',
			['red', 'navy']
		],
		[
			'a style: directive with a modifier',
			'<p style:color|important="red">x</p>',
			'.svelte',
			['red']
		],
		[
			'a style: directive expression with a modifier',
			"<p style:color|important={on ? 'red' : 'gray'}>x</p>",
			'.svelte',
			['red', 'gray']
		],
		[
			'an unterminated style attribute expression',
			"<p style={on ? 'red' : ''>x</p>\n<p>{'navy'}</p>",
			'.svelte',
			[]
		],
		['an unterminated style attribute', '<p style="color: tan>x</p>\n<p>red</p>', '.html', []],
		['a value around a { that closes nowhere', '<p style="color: tan; {">x</p>', '.html', ['tan']],
		[
			'an expression naming no colour',
			"<p style:display={on ? 'block' : 'none'}>x</p>",
			'.svelte',
			[]
		],
		['an asset path in url()', 'a { background: url(/icons/gold.svg) no-repeat; }', '.css', []],
		['a quoted asset path in url()', 'a { background: url("tan.png"), red; }', '.css', ['red']],
		['a quoted font name', "a { font-family: 'Gold Sans', serif; }", '.css', []],
		['a url() in a style attribute', '<p style="background: url(gold.svg)">x</p>', '.html', []],
		['a word of the markup', '<p class="red">red or white</p>', '.svelte', []],
		['a word of a script', "const state = 'red';", '.ts', []],
		['a class or id selector', '.red, #white { color: var(--color-red); }', '.css', []],
		['a CSS comment', 'a { /* not black */ color: var(--color-text); }', '.css', []],
		['a system colour', 'a { color: CanvasText; }', '.css', []]
	])('%s', (_name, source, extension, want) => {
		expect(flagged(source, extension)).toEqual(want);
	});

	test.each([
		['a style block', '<p>x</p>\n<style>\n\tp { color: black; }\n</style>', '.svelte'],
		['a style attribute', '<p>x</p>\n<p style="width: 1px; color: black">x</p>', '.html'],
		['a style: directive', '<p>x</p>\n<p style:color="black">x</p>', '.svelte'],
		['a style attribute expression', '<p>x</p>\n<p style={"color: black"}>x</p>', '.svelte'],
		['a style: directive expression', "<p>x</p>\n<p style:color={'black'}>x</p>", '.svelte'],
		[
			'an expression in a style attribute',
			'<p>x</p>\n<p style="color: {\'black\'}">x</p>',
			'.svelte'
		],
		[
			'a same-quote string in a style attribute expression',
			'<p>x</p>\n<p style="width: 1px; color: {"black"}">x</p>',
			'.svelte'
		],
		[
			'a nested-brace expression in a style attribute',
			'<p>x</p>\n<p style="color: {f({a: 1}) ? \'black\' : c}">x</p>',
			'.svelte'
		],
		[
			'a nested-brace style attribute expression',
			"<p>x</p>\n<p style={f({a: 1}) ? 'color: black' : ''}>x</p>",
			'.svelte'
		],
		[
			'the second expression in a style attribute',
			'<p>x</p>\n<p style="color: {c}; background: {on ? \'black\' : d}">x</p>',
			'.svelte'
		],
		[
			'a style: directive with a modifier',
			'<p>x</p>\n<p style:color|important="black">x</p>',
			'.svelte'
		]
	])('gives a colour of %s its offset in the file', (_name, source, extension) => {
		expect(colorViolations(source, extension)).toEqual([
			{ index: source.indexOf('black'), text: 'black' }
		]);
	});

	test('lists the colours in file order, whatever finds them', () => {
		const source =
			'<p style="color: red">x</p>\n<p>#fff rgb(0 0 0)</p>\n<p style:color="navy">x</p>';
		expect(colorViolations(source, '.svelte')).toEqual([
			{ index: source.indexOf('red'), text: 'red' },
			{ index: source.indexOf('#fff'), text: '#fff' },
			{ index: source.indexOf('rgb('), text: 'rgb(' },
			{ index: source.indexOf('navy'), text: 'navy' }
		]);
	});
});
