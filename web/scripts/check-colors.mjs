import { readdir, readFile } from 'node:fs/promises';
import { extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { colorViolations } from './colors.mjs';

const SOURCE_ROOT = fileURLToPath(new URL('../src/', import.meta.url));
const THEME_FILE = 'app.css';
const SOURCE_EXTENSIONS = new Set(['.css', '.html', '.js', '.svelte', '.ts']);

async function sourceFiles(directory) {
	const entries = await readdir(directory, { withFileTypes: true });
	const files = await Promise.all(
		entries.map((entry) => {
			const path = join(directory, entry.name);
			return entry.isDirectory() ? sourceFiles(path) : [path];
		})
	);
	return files.flat();
}

const violations = [];
for (const path of await sourceFiles(SOURCE_ROOT)) {
	const name = relative(SOURCE_ROOT, path);
	if (name === THEME_FILE || !SOURCE_EXTENSIONS.has(extname(path))) continue;
	const source = await readFile(path, 'utf8');
	for (const v of colorViolations(source, extname(path))) {
		const line = source.slice(0, v.index).split('\n').length;
		violations.push(`${name}:${line}: ${v.text}`);
	}
}

if (violations.length > 0) {
	console.error(`Colors must be defined in src/${THEME_FILE}:\n${violations.join('\n')}`);
	process.exitCode = 1;
}
