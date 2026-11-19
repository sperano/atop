// The board the App tests draw: a Vikunja task, its PR, the Kelos Task on
// that PR and a source's error row, as internal/server's tests send them.
import { HOUR_MS, row, snapshot, TEST_NOW } from './builders';

export const TASK = row({
	source: 'vikunja',
	name: '#7',
	origin: 'vikunja',
	state: 'idle',
	since: TEST_NOW - 2 * HOUR_MS,
	age: '2h',
	detail: '[claude] serve',
	target: 'https://vikunja.example/tasks/7'
});
export const PR = row({
	source: 'pr',
	name: 'vigie#3',
	depth: 1,
	guides: '└─ ',
	mark: '●',
	state: 'review',
	since: TEST_NOW - HOUR_MS,
	age: '1h',
	detail: 'Serve the board',
	target: 'https://github.com/o/vigie/pull/3',
	severity: 'attention',
	needsYou: true
});
export const AGENT = row({
	source: 'kelos task',
	name: 'fix',
	origin: 'unknown',
	depth: 2,
	guides: '   └─ ',
	state: 'busy',
	since: undefined,
	age: '-',
	severity: 'busy',
	busy: true
});
export const ERROR = row({
	source: 'mirror',
	name: '-',
	state: 'error',
	detail: 'down',
	mark: '●',
	severity: 'failed',
	needsYou: true,
	hideable: false
});
export const BOARD = snapshot([TASK, PR, AGENT, ERROR], {
	counts: { needYou: 2, busy: 1, idle: 1, hidden: 0, done: 0 }
});
