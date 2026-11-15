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
// Expressions holding braces of their own are not read.
const STYLE_BLOCK = /(<style\b[^>]*>)([\s\S]*?)<\/style>/gi;
const STYLE_ATTRIBUTE = /(\bstyle=(["']))([\s\S]*?)\2/g;
const STYLE_DIRECTIVE = /(\bstyle:[\w-]+=(["']))([\s\S]*?)\2/g;
const STYLE_ATTRIBUTE_EXPRESSION = /(\bstyle=\{)([^{}]*)\}/g;
const STYLE_DIRECTIVE_EXPRESSION = /(\bstyle:[\w-]+=\{)([^{}]*)\}/g;
const INTERPOLATION = /\{([^{}]*)\}/g;
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

/** The named colours of a quoted style value's {…} expressions, and the value without them. */
function interpolations(text) {
	const colors = [...text.matchAll(INTERPOLATION)].flatMap((m) =>
		namedColorsOfScript(m[1], m.index + 1)
	);
	return { css: blankCss(text.replace(INTERPOLATION, blank)), colors };
}

/** How each kind of CSS part yields its named colours, offsets relative to it. */
const SCANNERS = {
	block: (text) => namedColorsOfDeclarations(blankCss(text)),
	attribute: (text) => {
		const { css, colors } = interpolations(text);
		return [...colors, ...namedColorsOfDeclarations(css)];
	},
	directive: (text) => {
		const { css, colors } = interpolations(text);
		return [...colors, ...namedColorsOfValue(css, 0)];
	},
	expression: (text) => namedColorsOfScript(text, 0)
};

/** The CSS parts of a file: their text, offset and kind (a key of SCANNERS). */
function cssParts(source, extension) {
	if (extension === '.css') return [{ text: source, offset: 0, kind: 'block' }];
	const parts = [];
	const add = (re, group, kind) => {
		for (const m of source.matchAll(re)) {
			parts.push({ text: m[group], offset: m.index + m[1].length, kind });
		}
	};
	add(STYLE_BLOCK, 2, 'block');
	add(STYLE_ATTRIBUTE, 3, 'attribute');
	add(STYLE_DIRECTIVE, 3, 'directive');
	add(STYLE_ATTRIBUTE_EXPRESSION, 2, 'expression');
	add(STYLE_DIRECTIVE_EXPRESSION, 2, 'expression');
	return parts;
}

/** Every colour of source that is not a theme token, in file order. */
export function colorViolations(source, extension) {
	const found = [...source.matchAll(HEX_COLOR)].map((m) => ({ index: m.index, text: m[0] }));
	if (STYLE_EXTENSIONS.has(extension)) {
		for (const m of source.matchAll(COLOR_FUNCTION)) found.push({ index: m.index, text: m[0] });
		for (const part of cssParts(source, extension)) {
			for (const c of SCANNERS[part.kind](part.text)) {
				found.push({ index: part.offset + c.index, text: c.text });
			}
		}
	}
	return found.sort((a, b) => a.index - b.index);
}
