<script module lang="ts">
	/** How often the header's time and the ages move on. */
	export const TICK_MS = 1000;

	/**
	 * Opens url in a new tab without giving it this page. Not through the
	 * noopener feature: window.open then returns null even when the tab opened.
	 */
	function openInNewTab(url: string): boolean {
		const tab = window.open(url, '_blank');
		if (tab) tab.opener = null;
		return tab !== null;
	}
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import BoardTable from './BoardTable.svelte';
	import { isRefusal, TooSoonError, type Api } from './lib/api';
	import { titleLine } from './lib/format';
	import { actionOf, KEY_HELP, type Action } from './lib/keys';
	import { LiveBoard, type Connection, type OpenStream, type Timer } from './lib/live';
	import type { Snapshot } from './lib/model';
	import { moveTo, reselect } from './lib/selection';
	import * as status from './lib/status';

	const NO_COUNTS = { needYou: 0, busy: 0, idle: 0, hidden: 0 };

	interface Props {
		api: Api;
		/** Opens url in a new tab and says whether the browser let it. */
		openUrl?: (url: string) => boolean;
		now?: () => number;
		openStream?: OpenStream;
		timer?: Timer;
	}

	let { api, openUrl = openInNewTab, now = Date.now, openStream, timer }: Props = $props();

	let snapshot = $state<Snapshot>();
	let showHidden = $state(false);
	let selected = $state(0);
	let connection = $state<Connection>({ state: 'connecting' });
	let message = $state('');
	let time = $state(0);
	let table = $state<ReturnType<typeof BoardTable>>();

	const rows = $derived(snapshot?.rows ?? []);
	const statusText = $derived(
		status.statusLine({
			connection,
			refreshing: snapshot?.refreshing ?? false,
			showHidden,
			serverError: snapshot?.error ?? '',
			message
		})
	);

	function adopt(next: Snapshot): void {
		selected = reselect(rows[selected], selected, next.rows);
		snapshot = next;
	}

	/** Follows the server from mount to unmount. */
	let live: LiveBoard | undefined;

	onMount(() => {
		live = new LiveBoard({
			api,
			openStream,
			timer,
			onsnapshot: adopt,
			onconnection: (c) => (connection = c)
		});
		live.start(showHidden);
		time = now();
		const ticker = setInterval(() => (time = now()), TICK_MS);
		return () => {
			clearInterval(ticker);
			live?.stop();
		};
	});

	function open(): void {
		const row = rows[selected];
		if (!row) return;
		if (!row.target) {
			message = status.STATUS_NOTHING;
			return;
		}
		message = openUrl(row.target)
			? `opened ${row.target}`
			: `error: the browser did not open ${row.target}`;
	}

	async function refresh(): Promise<void> {
		const layout = showHidden;
		try {
			const next = await api.refresh(layout);
			live?.offer(next, layout);
		} catch (err) {
			message =
				err instanceof TooSoonError
					? status.tooSoonMessage(err.retryAfter)
					: `error: refresh: ${err instanceof Error ? err.message : String(err)}`;
		}
	}

	async function toggleHidden(): Promise<void> {
		const row = rows[selected];
		if (!row) return;
		const hide = !row.hidden;
		try {
			await api.setHidden(row, hide);
			message = hide ? status.hidMessage(row) : status.unhidMessage(row);
		} catch (err) {
			const text = err instanceof Error ? err.message : String(err);
			message = isRefusal(err) ? text : `error: ${text}`;
		}
	}

	function toggleShowHidden(): void {
		showHidden = !showHidden;
		message = showHidden ? '' : status.STATUS_HIDES_HIDDEN;
		live?.setShowHidden(showHidden);
	}

	function run(action: Action): void {
		switch (action.kind) {
			case 'move':
				selected = moveTo(selected, action.to, rows.length, table?.pageSize() ?? 1);
				return;
			case 'open':
				return open();
			case 'refresh':
				void refresh();
				return;
			case 'hide':
				void toggleHidden();
				return;
			case 'showHidden':
				return toggleShowHidden();
		}
	}

	function onkeydown(event: KeyboardEvent): void {
		const action = actionOf(event);
		if (!action) return;
		event.preventDefault();
		run(action);
	}
</script>

<svelte:window {onkeydown} />

<div class="app">
	<header>{titleLine(snapshot?.counts ?? NO_COUNTS, time)}</header>
	<BoardTable
		bind:this={table}
		{rows}
		{selected}
		now={time}
		empty={snapshot !== undefined && !snapshot.refreshing}
		onselect={(i) => (selected = i)}
	/>
	<footer>
		<div class="status" role="status">{statusText}</div>
		<div class="help">{KEY_HELP}</div>
	</footer>
</div>

<style>
	.app {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr) auto;
		height: 100dvh;
	}

	header {
		background: var(--color-header);
		color: var(--color-on-header);
		font-weight: bold;
		padding: var(--space-2) var(--space-3);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	footer {
		border-top: 1px solid var(--color-border);
		padding: var(--space-1) var(--space-3);
	}

	.status,
	.help {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-height: 1.5em;
	}

	.help {
		color: var(--color-muted);
	}
</style>
