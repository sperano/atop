import { describe, expect, test } from 'vitest';
import { row, snapshot, snapshotJSON, TEST_NOW } from '../testing/builders';
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

describe('decodeSnapshot', () => {
	test('decodes what the server sends', () => {
		const want = snapshot([PR], {
			refreshing: true,
			error: 'hidden rows: denied',
			counts: { needYou: 1, busy: 2, idle: 3, hidden: 4 }
		});
		expect(decodeSnapshot(snapshotJSON(want))).toEqual(want);
	});

	test('decodes the server test fixture byte for byte', () => {
		// internal/server/handlers_test.go, TestSnapshotRowJSON.
		const json = `{"fetched_at":"2026-11-15T12:00:00Z","refreshing":false,"error":"","counts":{"need_you":1,"busy":0,"idle":0,"hidden":0},"rows":[{"source":"pr","name":"vigie#3","origin":"github","depth":0,"guides":"","mark":"●","state":"review","since":"2026-11-15T11:00:00Z","age":"1h","detail":"Serve the board","target":"https://github.com/o/vigie/pull/3","severity":"attention","needs_you":true,"busy":false,"hidden":false}]}`;
		const got = decodeSnapshot(JSON.parse(json));
		expect(got.fetchedAt).toBe(TEST_NOW);
		expect(got.rows[0]?.since).toBe(TEST_NOW - 60 * 60 * 1000);
		expect(got.rows[0]?.needsYou).toBe(true);
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
