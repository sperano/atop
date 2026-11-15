<script lang="ts">
	import { formatAge } from './lib/format';
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
	}

	let { rows, selected, now, empty, onselect }: Props = $props();

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
			<col class="type" />
			<col class="origin" />
			<col class="name" />
			<col class="state" />
			<col class="age" />
			<col />
		</colgroup>
		<thead bind:this={head}>
			<tr>
				<th scope="col"><span class="visually-hidden">mark</span></th>
				<th scope="col">TYPE</th>
				<th scope="col">ORIGIN</th>
				<th scope="col">NAME</th>
				<th scope="col">STATE</th>
				<th scope="col" class="age">AGE</th>
				<th scope="col">DETAIL</th>
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
					<td class="mark" class:needs-you={row.mark === MARK_NEEDS_YOU}>{row.mark}</td>
					<td>{row.source}</td>
					<td>{row.origin}</td>
					<td title={row.name}>
						<span class="guides">{row.guides}</span>{#if row.target}<a
								href={row.target}
								target="_blank"
								rel="noopener noreferrer">{row.name}</a
							>{:else}{row.name}{/if}
					</td>
					<td title={row.state}>{row.state}</td>
					<td class="age">{row.since === undefined ? row.age : formatAge(row.since, now)}</td>
					<td title={row.detail}>{row.detail}</td>
				</tr>
			{/each}
			{#if empty && rows.length === 0}
				<tr class="quiet"><td></td><td colspan="6">no rows</td></tr>
			{/if}
		</tbody>
	</table>
</div>

<style>
	.viewport {
		overflow: auto;
		min-height: 0;
	}

	table {
		width: 100%;
		min-width: 110ch;
		table-layout: fixed;
		border-collapse: collapse;
	}

	col.mark {
		width: 3ch;
	}
	col.type {
		width: 15ch;
	}
	col.origin {
		width: 10ch;
	}
	col.name {
		width: 36ch;
	}
	col.state {
		width: 22ch;
	}
	col.age {
		width: 6ch;
	}

	th,
	td {
		padding: 0 var(--space-1);
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.guides {
		white-space: pre;
	}

	th {
		position: sticky;
		top: 0;
		background: var(--color-page);
		border-bottom: 1px solid var(--color-border);
		font-weight: bold;
	}

	.age {
		text-align: right;
	}

	tr {
		cursor: default;
	}

	tr.failed {
		color: var(--color-failed);
	}
	tr.blocked {
		color: var(--color-blocked);
	}
	tr.busy {
		color: var(--color-busy);
	}
	tr.quiet {
		color: var(--color-muted);
	}

	tr.selected {
		background: var(--color-selected);
	}

	.mark.needs-you {
		color: var(--color-mark);
	}

	a {
		text-decoration: underline;
		text-decoration-color: var(--color-border);
	}

	a:hover {
		text-decoration-color: currentColor;
	}

	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
	}
</style>
