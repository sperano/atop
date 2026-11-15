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
// attributes hold declarations, a style: directive a value. A .css file is
// CSS throughout.
const STYLE_BLOCK = /(<style\b[^>]*>)([\s\S]*?)<\/style>/gi;
const STYLE_ATTRIBUTE = /(\bstyle=(["']))(.*?)\2/g;
const STYLE_DIRECTIVE = /(\bstyle:[\w-]+=(["']))(.*?)\2/g;
const CSS_COMMENT = /\/\*[\s\S]*?\*\//g;
const DECLARATION = /([\w-]+\s*:\s*)([^;{}]+)/g;
// A bare word of a value: not part of a custom property, a class, an id or
// another ident.
const VALUE_WORD = /(?<![\w.#-])[a-z]+(?![\w-])/gi;

/** The CSS parts of a file: their text, offset and whether they are a value. */
function cssParts(source, extension) {
	if (extension === '.css') return [{ text: source, offset: 0, value: false }];
	const parts = [];
	const add = (re, group, value) => {
		for (const m of source.matchAll(re)) {
			parts.push({ text: m[group], offset: m.index + m[1].length, value });
		}
	};
	add(STYLE_BLOCK, 2, false);
	add(STYLE_ATTRIBUTE, 3, false);
	add(STYLE_DIRECTIVE, 3, true);
	return parts;
}

/** The named colours of a value, offsets relative to it. */
function namedColorsOfValue(value, offset) {
	return [...value.matchAll(VALUE_WORD)]
		.filter((w) => NAMED_COLORS.has(w[0].toLowerCase()))
		.map((w) => ({ index: offset + w.index, text: w[0] }));
}

/** The named colours in the declaration values of css, offsets relative to it. */
function namedColors(css, isValue) {
	const blanked = css.replace(CSS_COMMENT, (c) => ' '.repeat(c.length));
	if (isValue) return namedColorsOfValue(blanked, 0);
	return [...blanked.matchAll(DECLARATION)].flatMap((d) =>
		namedColorsOfValue(d[2], d.index + d[1].length)
	);
}

/** Every colour of source that is not a theme token, in file order. */
export function colorViolations(source, extension) {
	const found = [...source.matchAll(HEX_COLOR)].map((m) => ({ index: m.index, text: m[0] }));
	if (STYLE_EXTENSIONS.has(extension)) {
		for (const m of source.matchAll(COLOR_FUNCTION)) found.push({ index: m.index, text: m[0] });
		for (const part of cssParts(source, extension)) {
			for (const c of namedColors(part.text, part.value)) {
				found.push({ index: part.offset + c.index, text: c.text });
			}
		}
	}
	return found.sort((a, b) => a.index - b.index);
}
