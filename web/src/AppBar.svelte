<script lang="ts">
	import { APP_NAME, formatTime, updatedText } from './lib/format';
	import type { Counts } from './lib/model';

	interface Props {
		counts: Counts;
		/** When the server last finished a refresh, undefined before. */
		fetchedAt: number | undefined;
		/** Milliseconds since the epoch, which "updated … ago" is computed against. */
		now: number;
		refreshing: boolean;
		showHidden: boolean;
		onrefresh: () => void;
		ontogglehidden: () => void;
	}

	let { counts, fetchedAt, now, refreshing, showHidden, onrefresh, ontogglehidden }: Props =
		$props();

	const badges = $derived([
		{ kind: 'need-you', count: counts.needYou, label: 'need you' },
		{ kind: 'busy', count: counts.busy, label: 'busy' },
		{ kind: 'idle', count: counts.idle, label: 'idle' },
		...(counts.hidden > 0 ? [{ kind: 'hidden', count: counts.hidden, label: 'hidden' }] : [])
	]);
</script>

<header>
	<h1>{APP_NAME}</h1>
	<ul class="badges" aria-label="counts">
		{#each badges as badge (badge.kind)}
			<li class="badge {badge.kind}" class:active={badge.count > 0}>
				<strong>{badge.count}</strong>
				{badge.label}
			</li>
		{/each}
	</ul>
	<div class="actions">
		<time
			class="updated"
			datetime={fetchedAt === undefined ? undefined : new Date(fetchedAt).toISOString()}
			title={fetchedAt === undefined ? undefined : formatTime(fetchedAt)}
			>{updatedText(fetchedAt, now)}</time
		>
		<button type="button" aria-pressed={showHidden} onclick={ontogglehidden}>Show hidden</button>
		<button type="button" class="refresh" disabled={refreshing} onclick={onrefresh}>
			{refreshing ? 'Refreshing…' : 'Refresh'}
		</button>
	</div>
</header>

<style>
	header {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2) var(--space-4);
		padding: var(--space-3) var(--space-6);
		background: var(--color-header);
		color: var(--color-on-header);
	}

	h1 {
		margin: 0;
		font-size: var(--font-size-lg);
		font-weight: 700;
		letter-spacing: 0.02em;
	}

	.badges {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.badge {
		padding: 0 var(--space-3);
		border-radius: var(--radius-pill);
		background: var(--color-badge);
		white-space: nowrap;
	}

	.badge.need-you.active {
		background: var(--color-badge-need-you);
	}

	.badge.busy.active {
		background: var(--color-badge-busy);
	}

	.actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-left: auto;
	}

	.updated {
		color: var(--color-header-muted);
		margin-right: var(--space-2);
		white-space: nowrap;
	}

	button {
		padding: var(--space-1) var(--space-3);
		border: 1px solid var(--color-header-muted);
		border-radius: var(--radius-sm);
		background: var(--color-header);
		color: var(--color-on-header);
		cursor: pointer;
		white-space: nowrap;
	}

	button:hover:not(:disabled) {
		background: var(--color-badge);
	}

	button[aria-pressed='true'] {
		background: var(--color-badge);
		border-color: var(--color-on-header);
	}

	button:focus-visible {
		outline-color: var(--color-focus-on-header);
	}

	button:disabled {
		cursor: progress;
		color: var(--color-header-muted);
	}
</style>
