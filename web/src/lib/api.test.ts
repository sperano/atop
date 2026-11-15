import { describe, expect, test } from 'vitest';
import { row, snapshot, snapshotJSON } from '../testing/builders';
import {
	ApiError,
	createApi,
	eventsPath,
	hiddenPath,
	isRefusal,
	refreshPath,
	snapshotPath,
	TooSoonError
} from './api';
import { DecodeError } from './decode';

interface Call {
	url: string;
	method: string;
}

/** A fetch that answers every call with respond's answer and records it. */
function fakeFetch(respond: () => Response): { fetcher: typeof fetch; calls: Call[] } {
	const calls: Call[] = [];
	const fetcher = (async (input: RequestInfo | URL, init?: RequestInit) => {
		calls.push({ url: String(input), method: init?.method ?? 'GET' });
		return respond();
	}) as typeof fetch;
	return { fetcher, calls };
}

const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
	new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json', ...headers }
	});

const SNAPSHOT = snapshot([row({ source: 'pr', name: 'vigie#3' })]);

describe('paths', () => {
	test.each([
		[snapshotPath(false), '/api/v1/snapshot'],
		[snapshotPath(true), '/api/v1/snapshot?show_hidden=true'],
		[eventsPath(false), '/api/v1/events'],
		[eventsPath(true), '/api/v1/events?show_hidden=true'],
		[refreshPath(false), '/api/v1/refresh'],
		[refreshPath(true), '/api/v1/refresh?show_hidden=true'],
		[hiddenPath({ source: 'pr', name: 'vigie#3' }), '/api/v1/hidden/pr/vigie%233'],
		[hiddenPath({ source: 'kelos task', name: 'a/b' }), '/api/v1/hidden/kelos%20task/a%2Fb']
	])('%s', (got, want) => {
		expect(got).toBe(want);
	});
});

describe('snapshot', () => {
	test('gets and decodes the snapshot of the layout', async () => {
		const { fetcher, calls } = fakeFetch(() => json(snapshotJSON(SNAPSHOT)));
		expect(await createApi(fetcher).snapshot(true)).toEqual(SNAPSHOT);
		expect(calls).toEqual([{ url: '/api/v1/snapshot?show_hidden=true', method: 'GET' }]);
	});

	test("throws the server's error", async () => {
		const { fetcher } = fakeFetch(() => json({ error: 'show_hidden must be true or false' }, 400));
		await expect(createApi(fetcher).snapshot(false)).rejects.toThrowError(
			new ApiError(400, 'show_hidden must be true or false')
		);
	});

	test('throws the status when the body is not the server’s', async () => {
		const { fetcher } = fakeFetch(
			() => new Response('<html>bad gateway</html>', { status: 502, statusText: 'Bad Gateway' })
		);
		const err = await createApi(fetcher)
			.snapshot(false)
			.catch((e: unknown) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect(err).toMatchObject({ status: 502, message: '502 Bad Gateway' });
	});

	test('throws a DecodeError on a snapshot of another shape', async () => {
		const { fetcher } = fakeFetch(() => json({ rows: [] }));
		await expect(createApi(fetcher).snapshot(false)).rejects.toBeInstanceOf(DecodeError);
	});
});

describe('refresh', () => {
	test('posts and returns the 202 snapshot', async () => {
		const running = snapshot([], { refreshing: true });
		const { fetcher, calls } = fakeFetch(() => json(snapshotJSON(running), 202));
		expect(await createApi(fetcher).refresh(false)).toEqual(running);
		expect(calls).toEqual([{ url: '/api/v1/refresh', method: 'POST' }]);
	});

	test('throws TooSoonError with Retry-After in the cooldown', async () => {
		const { fetcher } = fakeFetch(() =>
			json({ error: 'refreshed too recently' }, 429, { 'Retry-After': '7' })
		);
		const err = await createApi(fetcher)
			.refresh(true)
			.catch((e: unknown) => e);
		expect(err).toBeInstanceOf(TooSoonError);
		expect(err).toMatchObject({ status: 429, retryAfter: 7, message: 'refreshed too recently' });
	});

	test('leaves retryAfter undefined without the header', async () => {
		const { fetcher } = fakeFetch(() => json({ error: 'refreshed too recently' }, 429));
		const err = await createApi(fetcher)
			.refresh(false)
			.catch((e: unknown) => e);
		expect(err).toMatchObject({ retryAfter: undefined });
	});

	test('throws on any other status', async () => {
		const { fetcher } = fakeFetch(() => json({ error: 'boom' }, 500));
		await expect(createApi(fetcher).refresh(false)).rejects.toThrowError(new ApiError(500, 'boom'));
	});
});

describe('setHidden', () => {
	const PR = { source: 'pr', name: 'vigie#3' };

	test.each([
		[true, 'PUT'],
		[false, 'DELETE']
	])('hide=%s sends %s', async (hide, method) => {
		const { fetcher, calls } = fakeFetch(() => new Response(null, { status: 204 }));
		await createApi(fetcher).setHidden(PR, hide);
		expect(calls).toEqual([{ url: '/api/v1/hidden/pr/vigie%233', method }]);
	});

	test('a 409 is a refusal', async () => {
		const { fetcher } = fakeFetch(() => json({ error: 'a source error cannot be hidden' }, 409));
		const err = await createApi(fetcher)
			.setHidden(PR, true)
			.catch((e: unknown) => e);
		expect(err).toMatchObject({ message: 'a source error cannot be hidden' });
		expect(isRefusal(err)).toBe(true);
	});

	test('a 500 is an error, not a refusal', async () => {
		const { fetcher } = fakeFetch(() => json({ error: 'hidden rows: read-only' }, 500));
		const err = await createApi(fetcher)
			.setHidden(PR, false)
			.catch((e: unknown) => e);
		expect(err).toMatchObject({ status: 500, message: 'hidden rows: read-only' });
		expect(isRefusal(err)).toBe(false);
	});
});
