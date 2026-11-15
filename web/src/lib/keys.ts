// The TUI's keys (internal/ui/keys.go), but q: a page is closed, not quit.

export type Action =
	| { readonly kind: 'move'; readonly to: Move }
	| { readonly kind: 'open' }
	| { readonly kind: 'refresh' }
	| { readonly kind: 'hide' }
	| { readonly kind: 'showHidden' };

export type Move = 'up' | 'down' | 'pageUp' | 'pageDown' | 'home' | 'end';

/** The key help in the footer. */
export const KEY_HELP =
	'↑/↓ j/k move · pgup/pgdn home/end · enter open · h hide · H show hidden · r refresh';

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
	H: { kind: 'showHidden' }
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
