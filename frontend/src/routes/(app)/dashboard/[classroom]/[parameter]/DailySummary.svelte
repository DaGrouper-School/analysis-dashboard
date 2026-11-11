<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { Chart, Parameter, PublicParameterPage } from '#lib/domain/types.ts';
	import { dailyStats, parameterCsv, rainDays } from '#lib/features/measurements/parameter-page.ts';
	import { UTF8_BOM } from '#lib/features/measurements/raw-data.ts';
	import { dayText, seriesLabel, showValue } from './value-format.ts';

	/** The daily summary (AP-23): one row a day in the chosen range, and the released data as CSV. */
	let {
		page,
		parameter,
		chart,
		rainInRange,
		statsInRange
	}: {
		page: Pick<PublicParameterPage, 'classroom' | 'series' | 'rain' | 'terms'>;
		parameter: Parameter;
		chart: Chart | null;
		/** Rain days in range, oldest first; null when the value is not rain. */
		rainInRange: ReturnType<typeof rainDays> | null;
		statsInRange: ReturnType<typeof dailyStats>;
	} = $props();

	const show = (value: number | string | null | undefined) => showValue(value, parameter);
	const referenceId = $derived(page.rain?.referenceSourceId ?? null);

	function download() {
		const units = Object.fromEntries(page.series.map((s) => [s.parameterId, parameter.unit]));
		const csv = parameterCsv(page.classroom.id, page.series, units, page.terms);
		const url = URL.createObjectURL(new Blob([UTF8_BOM, csv], { type: 'text/csv;charset=utf-8' }));
		const link = Object.assign(document.createElement('a'), {
			href: url,
			download: `dagrouper-${page.classroom.id}-${parameter.id}-released.csv`
		});
		link.click();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	}
</script>

<section class="mt-8" aria-labelledby="daily-heading">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 id="daily-heading" class="font-display text-2xl">Daily summary</h2>
		<button type="button" class="btn btn-ghost" onclick={download}>
			<Icon name="download" class="size-4" /> Download CSV
		</button>
	</div>
	{#if chart}
		<div class="card mt-3 overflow-x-auto p-0">
			<table class="w-full min-w-[32rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						<th class="px-4 py-3 font-medium">Day</th>
						{#if rainInRange}
							{#each page.series as s (s.source.id)}
								<th class="px-4 py-3 text-right font-medium">{seriesLabel(s, referenceId)}</th>
							{/each}
							<th class="px-4 py-3 font-medium">Check</th>
						{:else}
							<th class="px-4 py-3 text-right font-medium">Low</th>
							<th class="px-4 py-3 text-right font-medium">Average</th>
							<th class="px-4 py-3 text-right font-medium">High</th>
							<th class="px-4 py-3 text-right font-medium">Readings</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#if rainInRange}
						{#each rainInRange.toReversed() as day (day.day)}
							<tr>
								<td class="px-4 py-2.5 whitespace-nowrap">{dayText(day.day)}</td>
								{#each page.series as s (s.source.id)}
									<td class="px-4 py-2.5 text-right font-mono tabular-nums"
										>{show(day.totals[s.source.id])}</td
									>
								{/each}
								<td class="px-4 py-2.5">
									{#if day.review}
										<span class="text-orange">Flagged for review, {show(day.spread)} apart</span>
									{:else if day.spread !== null}
										<span class="text-muted">Agree</span>
									{:else}
										<span class="text-muted">One source</span>
									{/if}
								</td>
							</tr>
						{:else}
							<tr
								><td class="px-4 py-3 text-muted" colspan="9"
									>No released readings in this range.</td
								></tr
							>
						{/each}
					{:else}
						{#each statsInRange.toReversed() as day (day.day)}
							<tr>
								<td class="px-4 py-2.5 whitespace-nowrap">{dayText(day.day)}</td>
								<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.low)}</td>
								<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.mean)}</td>
								<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.high)}</td>
								<td class="px-4 py-2.5 text-right tabular-nums">{day.count}</td>
							</tr>
						{:else}
							<tr
								><td class="px-4 py-3 text-muted" colspan="5"
									>No released readings in this range.</td
								></tr
							>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	{/if}
	<p class="mt-2 text-xs text-muted">
		The CSV carries the licence ({page.terms.licence}) and how to credit {page.classroom.name}.
	</p>
</section>
