<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import type { Parameter } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/features/measurements/key-values.ts';
	import PanelHead from './PanelHead.svelte';
	import type { PageData } from './$types';

	/** The Manual records section (AP-45): readings entered by hand and field observations, newest first. */
	let {
		records,
		canRecord,
		parameters
	}: {
		records: PageData['records'];
		canRecord: boolean;
		parameters: ReadonlyMap<string, Parameter>;
	} = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<div class="flex flex-wrap items-end justify-between gap-3">
	<div>
		<PanelHead
			title="Manual records"
			sub="Readings entered by hand and field observations in this classroom, newest first."
		/>
	</div>
	{#if canRecord}
		<a class="btn btn-accent" href={resolve('/(app)/observe')}
			><Icon name="plus" class="size-4" /> Record observation</a
		>
	{/if}
</div>
{#if records.length}
	<div class="card mt-5 overflow-x-auto px-5 py-1">
		<table class="w-full min-w-[36rem] text-sm">
			<thead>
				<tr class="border-b border-line text-left text-xs text-muted">
					<th class="py-2.5 pr-3 font-medium">When</th>
					<th class="py-2.5 pr-3 font-medium">What</th>
					<th class="py-2.5 pr-3 font-medium">Value</th>
					<th class="py-2.5 font-medium">From</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each records as row (row.id)}
					{@const param = parameters.get(row.parameterId)}
					<tr>
						<td class="py-3 pr-3 whitespace-nowrap text-muted tabular-nums"
							>{dateTime.format(new Date(row.observedAt))}</td
						>
						<td class="py-3 pr-3 text-ink">{param?.label ?? row.parameterId}</td>
						<td class="py-3 pr-3 font-mono whitespace-nowrap text-ink"
							>{formatValue(row.value, param?.unit, param?.valueType)}</td
						>
						<td class="py-3">
							{row.sourceKind === 'field' ? 'Field observation' : row.sourceName}{row.contributorId
								? ` · participant ${row.contributorId}`
								: ''}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else}
	<p class="mt-4 text-muted">Nothing has been recorded by hand yet.</p>
{/if}
