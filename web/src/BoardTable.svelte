<script lang="ts">
	import { formatAge, formatTime } from './lib/format';
	import { MARK_NEEDS_YOU, rowKey, type Row } from './lib/model';
	import { pageRows } from './lib/selection';

	interface Props {
		rows: readonly Row[];
		selected: number;
		/** Milliseconds since the epoch, which the ages are computed against. */
		now: number;
		/** Says "no rows" when there are none. */
		empty: boolean;
		onselect: (index: number) => void;
		/** Hides the row at index, or unhides it when it is hidden. */
		onhide: (index: number) => void;
	}

	let { rows, selected, now, empty, onselect, onhide }: Props = $props();

	let viewport = $state<HTMLElement>();
	let head = $state<HTMLElement>();
	const rowElements: HTMLElement[] = $state([]);

	/** The rows the visible part of the table shows, for pgup/pgdn. */
	export function pageSize(): number {
		const first = rowElements[0];
		if (!viewport || !first) return 1;
		return pageRows(viewport.clientHeight - (head?.offsetHeight ?? 0), first.offsetHeight);
	}

	$effect(() => {
		rowElements[selected]?.scrollIntoView({ block: 'nearest' });
	});
</script>

<div class="viewport" bind:this={viewport}>
	<table>
		<colgroup>
			<col class="mark" />
			<col />
			<col class="type" />
			<col class="state" />
			<col class="age" />
			<col class="actions" />
		</colgroup>
		<thead bind:this={head}>
			<tr>
				<th scope="col"><span class="visually-hidden">needs you</span></th>
				<th scope="col">Name</th>
				<th scope="col" class="type">Type</th>
				<th scope="col">State</th>
				<th scope="col" class="age">Age</th>
				<th scope="col"><span class="visually-hidden">actions</span></th>
			</tr>
		</thead>
		<tbody>
			{#each rows as row, i (rowKey(row))}
				<tr
					bind:this={rowElements[i]}
					class={row.severity}
					class:selected={i === selected}
					aria-current={i === selected ? 'true' : undefined}
					onclick={() => onselect(i)}
					onfocusin={() => onselect(i)}
				>
					<td class="mark">
						{#if row.mark === MARK_NEEDS_YOU}<span class="dot" title="needs you"
								><span class="visually-hidden">needs you</span></span
							>{/if}
					</td>
					<td class="item" class:nested={row.depth > 0} style:--depth={row.depth}>
						<div class="name" title={row.name}>
							{#if row.target}<a href={row.target} target="_blank" rel="noopener noreferrer"
									>{row.name}</a
								>{:else}{row.name}{/if}{#if row.hidden}<span class="tag">hidden</span>{/if}
						</div>
						{#if row.detail}<div class="detail" title={row.detail}>{row.detail}</div>{/if}
					</td>
					<td class="type">
						<div class="source">{row.source}</div>
						<div class="origin">{row.origin}</div>
					</td>
					<td><span class="pill" title={row.state}>{row.state}</span></td>
					<td class="age" title={row.since === undefined ? undefined : formatTime(row.since)}
						>{row.since === undefined ? row.age : formatAge(row.since, now)}</td
					>
					<td class="actions">
						{#if row.hideable}<button
								type="button"
								onclick={(e) => {
									e.stopPropagation();
									onhide(i);
								}}>{row.hidden ? 'Unhide' : 'Hide'}</button
							>{/if}
					</td>
				</tr>
			{/each}
			{#if empty && rows.length === 0}
				<tr class="empty"><td colspan="6">Nothing to show.</td></tr>
			{/if}
		</tbody>
	</table>
</div>

<style>
	.viewport {
		overflow: auto;
		min-height: 0;
		max-width: var(--page-wide);
		margin: 0 auto;
		width: 100%;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		box-shadow: 0 1px 3px var(--color-shadow);
	}

	table {
		width: 100%;
		table-layout: fixed;
		border-collapse: collapse;
	}

	col.mark {
		width: 2rem;
	}
	col.type {
		width: 9rem;
	}
	col.state {
		width: 10rem;
	}
	col.age {
		width: 4.5rem;
	}
	col.actions {
		width: 5.5rem;
	}

	th,
	td {
		padding: var(--space-2) var(--space-2);
		text-align: left;
		vertical-align: middle;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	th {
		position: sticky;
		top: 0;
		z-index: 1;
		background: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		color: var(--color-muted);
		font-size: var(--font-size-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	tbody tr {
		border-bottom: 1px solid var(--color-border);
		cursor: default;
	}

	tbody tr:last-child {
		border-bottom: none;
	}

	tbody tr:hover {
		background: var(--color-hover);
	}

	tbody tr.selected {
		background: var(--color-selected);
	}

	tbody tr.selected td:first-child {
		box-shadow: inset 3px 0 0 var(--color-accent);
	}

	.mark {
		text-align: center;
	}

	.dot {
		display: inline-block;
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--color-mark);
		vertical-align: middle;
	}

	.item {
		position: relative;
		padding-left: calc(var(--space-2) + var(--depth) * var(--tree-indent));
	}

	/* An elbow from the parent row down to this one. */
	.item.nested::before {
		content: '';
		position: absolute;
		top: 0;
		left: calc(var(--space-2) + (var(--depth) - 0.6) * var(--tree-indent));
		width: calc(var(--tree-indent) * 0.45);
		height: 50%;
		border-left: 1px solid var(--color-control-border);
		border-bottom: 1px solid var(--color-control-border);
		border-bottom-left-radius: var(--radius-sm);
	}

	.name,
	.detail,
	.source,
	.origin {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.name {
		font-weight: 600;
	}

	.detail,
	.origin {
		color: var(--color-muted);
		font-size: var(--font-size-xs);
	}

	tr.quiet .name {
		font-weight: 500;
	}

	a {
		text-decoration: none;
	}

	a:hover {
		text-decoration: underline;
	}

	.tag {
		margin-left: var(--space-2);
		padding: 0 var(--space-2);
		border: 1px solid var(--color-control-border);
		border-radius: var(--radius-pill);
		color: var(--color-muted);
		font-size: var(--font-size-xs);
		font-weight: 400;
	}

	.pill {
		display: inline-block;
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		vertical-align: middle;
		padding: 0 var(--space-2);
		border-radius: var(--radius-pill);
		font-size: var(--font-size-xs);
		font-weight: 600;
	}

	tr.failed .pill {
		color: var(--color-failed);
		background: var(--color-failed-bg);
	}
	tr.blocked .pill {
		color: var(--color-blocked);
		background: var(--color-blocked-bg);
	}
	tr.busy .pill {
		color: var(--color-busy);
		background: var(--color-busy-bg);
	}
	tr.attention .pill {
		color: var(--color-attention);
		background: var(--color-attention-bg);
	}
	tr.quiet .pill {
		color: var(--color-quiet);
		background: var(--color-quiet-bg);
	}

	.age {
		text-align: right;
		font-variant-numeric: tabular-nums;
		color: var(--color-muted);
	}

	.actions {
		text-align: right;
	}

	.actions button {
		padding: 0 var(--space-2);
		border: 1px solid var(--color-control-border);
		border-radius: var(--radius-sm);
		background: var(--color-surface);
		font-size: var(--font-size-xs);
		cursor: pointer;
	}

	.actions button:hover {
		background: var(--color-hover);
	}

	/* With a pointer, the button shows on the row under it, the selected one
	   and the one holding focus (so Tab still reaches it); a touch screen
	   always shows it. */
	@media (hover: hover) {
		tbody tr:not(:hover, .selected, :focus-within) .actions button {
			opacity: 0;
		}
	}

	tr.empty td {
		padding: var(--space-6);
		text-align: center;
		color: var(--color-muted);
	}

	@media (width < 40rem) {
		col.type,
		.type {
			display: none;
		}
	}
</style>
