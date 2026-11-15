import { describe, expect, test } from 'vitest';
import { HOUR_MS, row, snapshot, snapshotJSON, TEST_NOW } from '../testing/builders';
import { decodeSnapshot, DecodeError } from './decode';

const PR = row({
	source: 'pr',
	name: 'vigie#3',
	depth: 1,
	guides: '└─ ',
	mark: '●',
	state: 'review',
	since: TEST_NOW - 1000,
	age: '1s',
	detail: 'Serve the board',
	target: 'https://github.com/o/vigie/pull/3',
	severity: 'attention',
	needsYou: true
});

const BUSY = row({ source: 'kelos task', name: 'busy', state: 'implementing', busy: true });
const HIDDEN = row({ source: 'vikunja', name: '7', hidden: true });

describe('decodeSnapshot', () => {
	test('decodes what the server sends', () => {
		const want = snapshot([PR, BUSY, HIDDEN], {
			refreshing: true,
			error: 'hidden rows: denied',
			counts: { needYou: 1, busy: 2, idle: 3, hidden: 4 }
		});
		expect(decodeSnapshot(snapshotJSON(want))).toEqual(want);
	});

	test('decodes the server test fixture byte for byte', () => {
		// internal/server/handlers_test.go, TestSnapshotRowJSON.
		const json = `{"fetched_at":"2026-11-15T12:00:00Z","refreshing":false,"error":"","counts":{"need_you":1,"busy":0,"idle":0,"hidden":0},"rows":[{"source":"pr","name":"vigie#3","origin":"github","depth":0,"guides":"","mark":"●","state":"review","since":"2026-11-15T11:00:00Z","age":"1h","detail":"Serve the board","target":"https://github.com/o/vigie/pull/3","severity":"attention","needs_you":true,"busy":false,"hidden":false}]}`;
		expect(decodeSnapshot(JSON.parse(json))).toEqual(
			snapshot(
				[
					row({
						source: 'pr',
						name: 'vigie#3',
						origin: 'github',
						mark: '●',
						state: 'review',
						since: TEST_NOW - HOUR_MS,
						age: '1h',
						detail: 'Serve the board',
						target: 'https://github.com/o/vigie/pull/3',
						severity: 'attention',
						needsYou: true
					})
				],
				{ counts: { needYou: 1, busy: 0, idle: 0, hidden: 0 } }
			)
		);
	});

	test.each([
		['javascript:alert(document.domain)'],
		['data:text/html,<script>alert(1)</script>'],
		['//evil.example/x'],
		['/api/v1/snapshot'],
		['https://'],
		['http:///x'],
		['https:/x'],
		['https:x'],
		['https:\\\\x/y'],
		[' https://x'],
		['https://ex\tample.com/'],
		['https://exam\nple.com/x'],
		['https://x.example/a\tb'],
		['https://x.example/\x7f'],
		['https://evil.example\\@good.example'],
		['https://x.example:80\\/'],
		['https://ex ample.com/'],
		['https://x%41.example/'],
		['https://x{y}.example/'],
		['https://a[b@x.example/'],
		['https://x.example/%zz'],
		['https://x.example/#%2'],
		['not a url']
	])('drops a target that is no http(s) URL both parsers take: %s', (target) => {
		const got = decodeSnapshot(snapshotJSON(snapshot([row({ source: 'pr', name: 'x', target })])));
		expect(got.rows[0]?.target).toBe('');
	});

	test.each([
		['https://github.com/o/vigie/pull/3'],
		['http://kelos.example/api/x'],
		['HTTPS://github.com/o'],
		['https://user:pw@x.example:8080/a b?q=1#f'],
		['https://[::1]:8080/'],
		['https://bücher.example/%C3%BC'],
		['https://x.example/a\\b']
	])('keeps an http(s) target: %s', (target) => {
		const got = decodeSnapshot(snapshotJSON(snapshot([row({ source: 'pr', name: 'x', target })])));
		expect(got.rows[0]?.target).toBe(target);
	});

	test('maps null times to undefined', () => {
		const got = decodeSnapshot(
			snapshotJSON(snapshot([row({ source: 'pr', name: 'x' })], { fetchedAt: undefined }))
		);
		expect(got.fetchedAt).toBeUndefined();
		expect(got.rows[0]?.since).toBeUndefined();
	});

	const valid = () => snapshotJSON(snapshot([PR]));
	const firstRow = (s: Record<string, unknown>) => (s['rows'] as Record<string, unknown>[])[0]!;

	test.each([
		['not an object', () => [] as unknown, 'snapshot is not an object'],
		['null', () => null, 'snapshot is not an object'],
		['rows missing', () => ({ ...valid(), rows: undefined }), 'snapshot.rows is not an array'],
		[
			'refreshing a string',
			() => ({ ...valid(), refreshing: 'yes' }),
			'snapshot.refreshing is not a boolean'
		],
		['error missing', () => ({ ...valid(), error: undefined }), 'snapshot.error is not a string'],
		[
			'bad time',
			() => ({ ...valid(), fetched_at: 'yesterday' }),
			'snapshot.fetched_at is not a time or null'
		],
		[
			'counts missing',
			() => ({ ...valid(), counts: undefined }),
			'snapshot.counts is not an object'
		],
		[
			'negative count',
			() => ({ ...valid(), counts: { need_you: -1, busy: 0, idle: 0, hidden: 0 } }),
			'snapshot.counts.need_you is not a count'
		],
		[
			'fractional count',
			() => ({ ...valid(), counts: { need_you: 0, busy: 1.5, idle: 0, hidden: 0 } }),
			'snapshot.counts.busy is not a count'
		],
		[
			'row not an object',
			() => ({ ...valid(), rows: ['pr'] }),
			'snapshot.rows[0] is not an object'
		],
		[
			'unknown severity',
			() => {
				const s = valid();
				firstRow(s)['severity'] = 'scary';
				return s;
			},
			'snapshot.rows[0].severity is not a severity: scary'
		],
		[
			'needs_you missing',
			() => {
				const s = valid();
				delete firstRow(s)['needs_you'];
				return s;
			},
			'snapshot.rows[0].needs_you is not a boolean'
		],
		[
			'since a number',
			() => {
				const s = valid();
				firstRow(s)['since'] = 0;
				return s;
			},
			'snapshot.rows[0].since is not a time or null'
		]
	])('refuses %s', (_name, input, message) => {
		expect(() => decodeSnapshot(input())).toThrowError(new DecodeError(message));
	});
});
