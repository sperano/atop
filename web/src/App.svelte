<script module lang="ts">
	/** How often the "updated … ago" text and the ages move on. */
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
	import AppBar from './AppBar.svelte';
	import BoardTable from './BoardTable.svelte';
	import { isRefusal, TooSoonError, type Api, type Layout } from './lib/api';
	import { documentTitle } from './lib/format';
	import { actionOf, KEY_HELP, type Action } from './lib/keys';
	import { LiveBoard, type Connection, type OpenStream, type Timer } from './lib/live';
	import type { Snapshot } from './lib/model';
	import { moveTo, reselect } from './lib/selection';
	import * as status from './lib/status';

	const NO_COUNTS = { needYou: 0, busy: 0, idle: 0, hidden: 0, done: 0 };

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
	let hideDone = $state(false);
	let selected = $state(0);
	let connection = $state<Connection>({ state: 'connecting' });
	let message = $state('');
	let time = $state(0);
	let table = $state<ReturnType<typeof BoardTable>>();

	const rows = $derived(snapshot?.rows ?? []);
	const counts = $derived(snapshot?.counts ?? NO_COUNTS);
	const statusText = $derived(
		status.statusLine({
			connection,
			refreshing: snapshot?.refreshing ?? false,
			showHidden,
			hideDone,
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
		live.start({ showHidden, hideDone });
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
			: `${status.ERROR_PREFIX}the browser did not open ${row.target}`;
	}

	async function refresh(): Promise<void> {
		const layout: Layout = { showHidden, hideDone };
		try {
			const next = await api.refresh(layout);
			live?.offer(next, layout);
		} catch (err) {
			message =
				err instanceof TooSoonError
					? status.tooSoonMessage(err.retryAfter)
					: `${status.ERROR_PREFIX}refresh: ${err instanceof Error ? err.message : String(err)}`;
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
			message = isRefusal(err) ? text : `${status.ERROR_PREFIX}${text}`;
		}
	}

	function toggleShowHidden(): void {
		showHidden = !showHidden;
		message = showHidden ? '' : status.STATUS_HIDES_HIDDEN;
		live?.setLayout({ showHidden, hideDone });
	}

	function toggleHideDone(): void {
		hideDone = !hideDone;
		message = hideDone ? '' : status.STATUS_SHOWS_DONE_AGAIN;
		live?.setLayout({ showHidden, hideDone });
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
			case 'hideDone':
				return toggleHideDone();
		}
	}

	function hideRow(index: number): void {
		selected = index;
		void toggleHidden();
	}

	function onkeydown(event: KeyboardEvent): void {
		const action = actionOf(event);
		if (!action) return;
		// Enter on a button presses it, rather than opening the selected row.
		if (action.kind === 'open' && event.target instanceof HTMLButtonElement) return;
		event.preventDefault();
		run(action);
	}
</script>

<svelte:window {onkeydown} />

<svelte:head>
	<title>{documentTitle(counts)}</title>
</svelte:head>

<div class="app">
	<AppBar
		{counts}
		fetchedAt={snapshot?.fetchedAt}
		now={time}
		refreshing={snapshot?.refreshing ?? false}
		{showHidden}
		{hideDone}
		onrefresh={() => run({ kind: 'refresh' })}
		ontogglehidden={() => run({ kind: 'showHidden' })}
		ontoggledone={() => run({ kind: 'hideDone' })}
	/>
	<main>
		<BoardTable
			bind:this={table}
			{rows}
			{selected}
			now={time}
			empty={snapshot !== undefined && !snapshot.refreshing}
			onselect={(i) => (selected = i)}
			onhide={hideRow}
		/>
	</main>
	<footer>
		<div class="status" class:error={status.hasError(statusText)} role="status">
			{statusText}
		</div>
		<ul class="help" aria-label="keyboard shortcuts">
			{#each KEY_HELP as hint (hint.does)}
				<li>
					{#each hint.keys as key (key)}<kbd>{key}</kbd>{/each}
					<span class="does">{hint.does}</span>
				</li>
			{/each}
		</ul>
	</footer>
</div>

<style>
	.app {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr) auto;
		height: 100dvh;
	}

	main {
		display: flex;
		flex-direction: column;
		min-height: 0;
		padding: var(--space-4) var(--space-6);
	}

	footer {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-1) var(--space-4);
		padding: var(--space-2) var(--space-6);
		border-top: 1px solid var(--color-border);
		background: var(--color-surface);
		font-size: var(--font-size-xs);
	}

	.status {
		min-height: 1.5em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.status.error {
		color: var(--color-failed);
	}

	.help {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-3);
		margin: 0 0 0 auto;
		padding: 0;
		list-style: none;
		color: var(--color-muted);
	}

	kbd {
		display: inline-block;
		min-width: 1.5em;
		margin-right: var(--space-1);
		padding: 0 var(--space-1);
		border: 1px solid var(--color-control-border);
		border-radius: var(--radius-sm);
		background: var(--color-page);
		color: var(--color-fg);
		font-family: var(--font-mono);
		text-align: center;
	}

	@media (width < 40rem) {
		main {
			padding: var(--space-2);
		}

		.help {
			display: none;
		}
	}
</style>
