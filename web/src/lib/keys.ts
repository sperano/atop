// The TUI's keys (internal/ui/keys.go), but q: a page is closed, not quit.

export type Action =
	| { readonly kind: 'move'; readonly to: Move }
	| { readonly kind: 'open' }
	| { readonly kind: 'refresh' }
	| { readonly kind: 'hide' }
	| { readonly kind: 'showHidden' }
	| { readonly kind: 'hideDone' };

export type Move = 'up' | 'down' | 'pageUp' | 'pageDown' | 'home' | 'end';

/** A line of the key help in the footer: the keys, then what they do. */
export interface KeyHint {
	readonly keys: readonly string[];
	readonly does: string;
}

export const KEY_HELP: readonly KeyHint[] = [
	{ keys: ['↑', '↓', 'j', 'k'], does: 'move' },
	{ keys: ['PgUp', 'PgDn', 'Home', 'End'], does: 'jump' },
	{ keys: ['Enter'], does: 'open' },
	{ keys: ['h'], does: 'hide' },
	{ keys: ['H'], does: 'show hidden' },
	{ keys: ['d'], does: 'hide done' },
	{ keys: ['r'], does: 'refresh' }
];

const MOVES: Readonly<Record<string, Move>> = {
	ArrowUp: 'up',
	k: 'up',
	ArrowDown: 'down',
	j: 'down',
	PageUp: 'pageUp',
	PageDown: 'pageDown',
	Home: 'home',
	End: 'end'
};

const COMMANDS: Readonly<Record<string, Action>> = {
	Enter: { kind: 'open' },
	r: { kind: 'refresh' },
	h: { kind: 'hide' },
	H: { kind: 'showHidden' },
	d: { kind: 'hideDone' }
};

export type KeyInput = Pick<
	KeyboardEvent,
	'key' | 'ctrlKey' | 'metaKey' | 'altKey' | 'isComposing'
>;

/**
 * The action of a key press, undefined for any other key. A press with
 * ctrl, meta or alt is the browser's (ctrl+r reloads, alt+← goes back).
 */
export function actionOf(e: KeyInput): Action | undefined {
	if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return undefined;
	const move = MOVES[e.key];
	if (move) return { kind: 'move', to: move };
	return COMMANDS[e.key];
}
