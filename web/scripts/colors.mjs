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
// attributes hold declarations, a style: directive a value, and the string
// literals of a Svelte expression (style={…}, style:x={…}, or a {…} inside
// a quoted value) are scanned as values. A .css file is CSS throughout.
// An expression runs to its matching }, nested braces and braces in its
// string literals included, so a quote inside a quoted value's {…} does not
// end the value. A { that closes nowhere is text.
const STYLE_BLOCK = /(<style\b[^>]*>)([\s\S]*?)<\/style>/gi;
const STYLE_ATTRIBUTE_START = /\bstyle(:[\w-]+)?=(["'{])/g;
const OPEN_BRACE = '{';
const CLOSE_BRACE = '}';
const ESCAPE = '\\';
const SCRIPT_QUOTES = new Set(['"', "'", '`']);
const STRING_LITERAL = /(["'`])((?:\\[\s\S]|(?!\1)[^\\])*)\1/g;
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

/** The named colours of the string literals of a script expression. */
function namedColorsOfScript(expression, offset) {
	return [...expression.matchAll(STRING_LITERAL)].flatMap((s) =>
		namedColorsOfValue(blankCss(s[2]), offset + s.index + s[1].length)
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

/** The index of the } closing the expression opening at text[open], or -1. */
function closingBrace(text, open) {
	let depth = 0;
	for (let i = open; i < text.length; i++) {
		if (SCRIPT_QUOTES.has(text[i])) {
			i = stringEnd(text, i);
			if (i === -1) return -1;
		} else if (text[i] === OPEN_BRACE) depth++;
		else if (text[i] === CLOSE_BRACE && --depth === 0) return i;
	}
	return -1;
}

/**
 * The quoted value opening at source[open]: the index of its closing quote
 * (-1 if none) and its {…} expressions as [start, end] of their insides.
 */
function quotedValue(source, open) {
	const expressions = [];
	for (let i = open + 1; i < source.length; i++) {
		if (source[i] === source[open]) return { end: i, expressions };
		const close = source[i] === OPEN_BRACE ? closingBrace(source, i) : -1;
		if (close === -1) continue;
		expressions.push([i + 1, close]);
		i = close;
	}
	return { end: -1, expressions };
}

/** The named colours of a quoted style value's {…} expressions, and the value without them. */
function interpolations({ text, expressions }) {
	let css = text;
	const colors = [];
	for (const [start, end] of expressions) {
		colors.push(...namedColorsOfScript(text.slice(start, end), start));
		css = css.slice(0, start - 1) + blank(text.slice(start - 1, end + 1)) + css.slice(end + 1);
	}
	return { css: blankCss(css), colors };
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
	expression: (part) => namedColorsOfScript(part.text, 0)
};

/** The style attribute or directive whose value opens at source[open], or null if it never ends. */
function styleValuePart(source, open, kind) {
	const offset = open + 1;
	if (source[open] === OPEN_BRACE) {
		const close = closingBrace(source, open);
		if (close === -1) return null;
		return { text: source.slice(offset, close), offset, kind: 'expression', expressions: [] };
	}
	const { end, expressions } = quotedValue(source, open);
	if (end === -1) return null;
	const relative = expressions.map(([start, close]) => [start - offset, close - offset]);
	return { text: source.slice(offset, end), offset, kind, expressions: relative };
}

/** The CSS parts of a file: their text, offset, kind (a key of SCANNERS) and {…} expressions. */
function cssParts(source, extension) {
	if (extension === '.css') return [{ text: source, offset: 0, kind: 'block', expressions: [] }];
	const parts = [...source.matchAll(STYLE_BLOCK)].map((m) => ({
		text: m[2],
		offset: m.index + m[1].length,
		kind: 'block',
		expressions: []
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
