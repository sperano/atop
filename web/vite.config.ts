import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import type { BrowserCommand } from 'vitest/node';

// `vigie serve`, which serves the API while `vite` serves the UI.
const DEV_API_TARGET = 'http://localhost:8080';

// Tests of Svelte components (Foo.svelte.test.ts) run in Chromium, the rest in Node.
const COMPONENT_TESTS = 'src/**/*.svelte.test.ts';
// The Node tests' time zone: not UTC, so that a local time read through the
// UTC getters fails them on a UTC host (CI).
const NODE_TEST_TZ = 'America/Montreal';

const emulateColorScheme: BrowserCommand<[colorScheme: 'light' | 'dark' | null]> = async (
	context,
	colorScheme
) => {
	if (context.provider.name !== 'playwright') {
		throw new Error(`emulateColorScheme requires Playwright, not ${context.provider.name}`);
	}
	await context.page.emulateMedia({ colorScheme });
};

// A static single-page application: the Go binary embeds dist/ and serves
// it next to its API, so there is no Node server at runtime.
export default defineConfig({
	plugins: [svelte()],
	build: {
		outDir: 'dist',
		// dist/.vite/license.md: the licenses of the dependencies in the bundle,
		// which the image ships under /licenses. go:embed skips dot directories,
		// so the binary does not serve it.
		license: true
	},
	server: {
		proxy: {
			'/api': { target: DEV_API_TARGET },
			'/healthz': { target: DEV_API_TARGET }
		}
	},
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: true,
				test: {
					name: 'browser',
					include: [COMPONENT_TESTS],
					setupFiles: ['src/browser-setup.ts'],
					browser: {
						enabled: true,
						headless: true,
						provider: playwright(),
						instances: [{ browser: 'chromium' }],
						commands: { emulateColorScheme }
					}
				}
			},
			{
				extends: true,
				test: {
					name: 'node',
					environment: 'node',
					env: { TZ: NODE_TEST_TZ },
					include: ['src/**/*.test.ts', 'scripts/**/*.test.mjs'],
					exclude: [COMPONENT_TESTS]
				}
			}
		]
	}
});
