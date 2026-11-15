// What check-colors.mjs flags in a source file: a colour that does not come
// from the theme's tokens.

const HEX_COLOR = /#[\da-f]{3,8}\b/gi;
// A colour function is a colour too, in what styles the page; tests compare
// computed styles, which the browser reports as rgb(), so they may name one.
const COLOR_FUNCTION = /\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/gi;
export const STYLE_EXTENSIONS = new Set(['.css', '.html', '.svelte']);

// CSS Color 4's named colours. System colours (Canvas, CanvasText…) follow
// the browser's scheme and stay allowed.
export const NAMED_COLORS = new Set(
	`aliceblue antiquewhite aqua aquamarine azure beige bisque black
	blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse
	chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan
	darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta
	darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen
	darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink
	deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen
	fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey
	honeydew hotpink indianred indigo ivory khaki lavender lavenderblush
	lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow
	lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen
	lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime
	limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid
	mediumpurple mediumseagreen mediumslateblue mediumspringgreen
	mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin
	navajowhite navy oldlace olive olivedrab orange orangered orchid
	palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff
	peru pink plum powderblue purple rebeccapurple red rosybrown royalblue
	saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue
	slateblue slategray slategrey snow springgreen steelblue tan teal thistle
	tomato turquoise violet wheat white whitesmoke yellow yellowgreen`.split(/\s+/)
);

// Where CSS sits in a .svelte or .html file: style blocks and style
// attributes hold declarations, a style: directive (modifiers such as
// |important included) a value, and the string literals of a Svelte
// expression (style={…}, style:x={…}, or a {…} inside a quoted value) are
// scanned as values. A .css file is CSS throughout. An expression runs to its
// matching }, nested braces, braces in its string literals and its comments
// skipped, so a quote inside a quoted value's {…} does not end the value. A {
// that closes nowhere is text. A quote in a regex literal is read as opening a
// string: telling a regex from a division takes a JavaScript parser.
const STYLE_BLOCK = /(<style\b[^>]*>)([\s\S]*?)<\/style>/gi;
const STYLE_ATTRIBUTE_START = /\bstyle(:[\w-]+(?:\|[\w-]+)*)?=(["'{])/g;
const OPEN_BRACE = '{';
const CLOSE_BRACE = '}';
const ESCAPE = '\\';
const SCRIPT_QUOTES = new Set(['"', "'", '`']);
const LINE_COMMENT = '//';
const BLOCK_COMMENT = '/*';
const BLOCK_COMMENT_END = '*/';
const COMMENT_OPENER_LENGTH = 2;
const NEWLINE = '\n';
// What a value names that is no colour: comments, url()s (an asset such as
// gold.svg) and strings (a font such as 'Gold Sans').
const CSS_COMMENT = /\/\*[\s\S]*?\*\//g;
const CSS_URL = /\burl\((?:"[^"]*"|'[^']*'|[^)]*)\)/gi;
const CSS_STRING = /"[^"]*"|'[^']*'/g;
const DECLARATION = /([\w-]+\s*:\s*)([^;{}]+)/g;
// A bare word of a value: not part of a custom property, a class, an id or
// another ident.
const VALUE_WORD = /(?<![\w.#-])[a-z]+(?![\w-])/gi;

/** Spaces as long as text, so that offsets stay. */
const blank = (text) => ' '.repeat(text.length);

/** css with what names no colour blanked. */
function blankCss(css) {
	return css.replace(CSS_COMMENT, blank).replace(CSS_URL, blank).replace(CSS_STRING, blank);
}

/** The named colours of a blanked value, offsets relative to it plus offset. */
function namedColorsOfValue(value, offset) {
	return [...value.matchAll(VALUE_WORD)]
		.filter((w) => NAMED_COLORS.has(w[0].toLowerCase()))
		.map((w) => ({ index: offset + w.index, text: w[0] }));
}

/** The named colours in the declaration values of blanked css. */
function namedColorsOfDeclarations(css) {
	return [...css.matchAll(DECLARATION)].flatMap((d) =>
		namedColorsOfValue(d[2], d.index + d[1].length)
	);
}

/** The named colours of text's string literals, given as [start, end] of their insides. */
function namedColorsOfStrings(text, strings) {
	return strings.flatMap(([start, end]) =>
		namedColorsOfValue(blankCss(text.slice(start, end)), start)
	);
}

/** The index of the quote closing the string literal opening at text[open], or -1. */
function stringEnd(text, open) {
	for (let i = open + 1; i < text.length; i++) {
		if (text[i] === ESCAPE) i++;
		else if (text[i] === text[open]) return i;
	}
	return -1;
}

/**
 * The index of the last character of the script comment opening at
 * text[open] (text.length if it never ends), or null if none opens there.
 */
function commentEnd(text, open) {
	const opener = text.slice(open, open + COMMENT_OPENER_LENGTH);
	if (opener === LINE_COMMENT) {
		const end = text.indexOf(NEWLINE, open);
		return end === -1 ? text.length : end;
	}
	if (opener !== BLOCK_COMMENT) return null;
	const end = text.indexOf(BLOCK_COMMENT_END, open + COMMENT_OPENER_LENGTH);
	return end === -1 ? text.length : end + BLOCK_COMMENT_END.length - 1;
}

/**
 * The script expression opening at text[open]: the index of its closing }
 * (-1 if none) and its string literals as [start, end] of their insides.
 */
function scriptExpression(text, open) {
	const strings = [];
	let depth = 0;
	for (let i = open; i < text.length; i++) {
		const comment = commentEnd(text, i);
		if (comment !== null) i = comment;
		else if (SCRIPT_QUOTES.has(text[i])) {
			const end = stringEnd(text, i);
			if (end === -1) return { close: -1, strings };
			strings.push([i + 1, end]);
			i = end;
		} else if (text[i] === OPEN_BRACE) depth++;
		else if (text[i] === CLOSE_BRACE && --depth === 0) return { close: i, strings };
	}
	return { close: -1, strings };
}

/**
 * The quoted value opening at source[open]: the index of its closing quote
 * (-1 if none), its {…} expressions as [open, close] of their braces, and
 * their string literals as [start, end] of their insides.
 */
function quotedValue(source, open) {
	const expressions = [];
	const strings = [];
	for (let i = open + 1; i < source.length; i++) {
		if (source[i] === source[open]) return { end: i, expressions, strings };
		if (source[i] !== OPEN_BRACE) continue;
		const expression = scriptExpression(source, i);
		if (expression.close === -1) continue;
		expressions.push([i, expression.close]);
		strings.push(...expression.strings);
		i = expression.close;
	}
	return { end: -1, expressions, strings };
}

/** The named colours of a quoted style value's {…} expressions, and the value without them. */
function interpolations({ text, expressions, strings }) {
	let css = text;
	for (const [open, close] of expressions) {
		css = css.slice(0, open) + blank(text.slice(open, close + 1)) + css.slice(close + 1);
	}
	return { css: blankCss(css), colors: namedColorsOfStrings(text, strings) };
}

/** How each kind of CSS part yields its named colours, offsets relative to it. */
const SCANNERS = {
	block: (part) => namedColorsOfDeclarations(blankCss(part.text)),
	attribute: (part) => {
		const { css, colors } = interpolations(part);
		return [...colors, ...namedColorsOfDeclarations(css)];
	},
	directive: (part) => {
		const { css, colors } = interpolations(part);
		return [...colors, ...namedColorsOfValue(css, 0)];
	},
	expression: (part) => namedColorsOfStrings(part.text, part.strings)
};

/** ranges, each [start, end], made relative to offset. */
const shift = (ranges, offset) => ranges.map(([start, end]) => [start - offset, end - offset]);

/** The style attribute or directive whose value opens at source[open], or null if it never ends. */
function styleValuePart(source, open, kind) {
	const offset = open + 1;
	if (source[open] === OPEN_BRACE) {
		const { close, strings } = scriptExpression(source, open);
		if (close === -1) return null;
		const text = source.slice(offset, close);
		return { text, offset, kind: 'expression', strings: shift(strings, offset) };
	}
	const { end, expressions, strings } = quotedValue(source, open);
	if (end === -1) return null;
	return {
		text: source.slice(offset, end),
		offset,
		kind,
		expressions: shift(expressions, offset),
		strings: shift(strings, offset)
	};
}

/** The CSS parts of a file: their text, offset, kind (a key of SCANNERS) and script ranges. */
function cssParts(source, extension) {
	if (extension === '.css') return [{ text: source, offset: 0, kind: 'block' }];
	const parts = [...source.matchAll(STYLE_BLOCK)].map((m) => ({
		text: m[2],
		offset: m.index + m[1].length,
		kind: 'block'
	}));
	for (const m of source.matchAll(STYLE_ATTRIBUTE_START)) {
		const open = m.index + m[0].length - 1;
		const part = styleValuePart(source, open, m[1] ? 'directive' : 'attribute');
		if (part) parts.push(part);
	}
	return parts;
}

/** Every colour of source that is not a theme token, in file order. */
export function colorViolations(source, extension) {
	const found = [...source.matchAll(HEX_COLOR)].map((m) => ({ index: m.index, text: m[0] }));
	if (STYLE_EXTENSIONS.has(extension)) {
		for (const m of source.matchAll(COLOR_FUNCTION)) found.push({ index: m.index, text: m[0] });
		for (const part of cssParts(source, extension)) {
			for (const c of SCANNERS[part.kind](part)) {
				found.push({ index: part.offset + c.index, text: c.text });
			}
		}
	}
	return found.sort((a, b) => a.index - b.index);
}
