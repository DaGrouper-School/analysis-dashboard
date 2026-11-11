<script lang="ts">
	import { resolve } from '$app/paths';
	import type { PublicClassroom, ReviewState } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';

	/**
	 * The same few released values from each classroom (AP-20): rain today and over seven days,
	 * air temperature, last sync and review state. Each name opens the classroom's station.
	 */
	let {
		classrooms,
		units
	}: { classrooms: PublicClassroom[]; units: { rain: string; temperature: string } } = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const dayFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' });

	const REVIEW: Record<ReviewState, string> = {
		released: 'Released',
		in_review: 'Newer data in review',
		not_released: 'Not released yet'
	};
</script>

{#snippet reading(value: number | null, unit: string)}
	{#if value === null}
		<span class="text-muted italic">no data</span>
	{:else}
		<span class="font-mono whitespace-nowrap tabular-nums">{formatValue(value, unit)}</span>
	{/if}
{/snippet}

<div class="card overflow-x-auto p-0">
	<table class="w-full min-w-[46rem] text-left text-sm">
		<thead class="text-xs text-muted">
			<tr class="border-b border-line">
				<th class="px-4 py-3 font-medium">Classroom</th>
				<th class="px-4 py-3 text-right font-medium">Rain today</th>
				<th class="px-4 py-3 text-right font-medium">Rain, last 7 days</th>
				<th class="px-4 py-3 text-right font-medium">Air temperature now</th>
				<th class="px-4 py-3 font-medium">Last sync</th>
				<th class="px-4 py-3 font-medium">Review</th>
			</tr>
		</thead>
		<tbody class="divide-y divide-line">
			{#each classrooms as { id, name, core } (id)}
				<tr class="align-top">
					<th scope="row" class="px-4 py-3 font-medium text-ink">
						<a class="link" href={resolve('/classrooms/[classroom]', { classroom: id })}>{name}</a>
						{#if core.day}
							<span class="block text-xs font-normal text-muted"
								>{dayFormat.format(new Date(`${core.day}T00:00:00Z`))}</span
							>
						{/if}
					</th>
					<td class="px-4 py-3 text-right">{@render reading(core.rainToday, units.rain)}</td>
					<td class="px-4 py-3 text-right">
						{@render reading(core.rainLast7Days, units.rain)}
						{#if core.rainLast7Days !== null && core.rainDays < 7}
							<span class="block text-xs text-muted">{core.rainDays} of 7 days</span>
						{/if}
					</td>
					<td class="px-4 py-3 text-right"
						>{@render reading(core.airTemperature, units.temperature)}</td
					>
					<td class="px-4 py-3 whitespace-nowrap">
						{#if core.lastSync}
							{dateTime.format(new Date(core.lastSync))}
						{:else}
							<span class="text-muted italic">no data</span>
						{/if}
					</td>
					<td class="px-4 py-3">
						<span
							class={[
								'chip',
								core.review === 'released' && 'border-lime/50 text-lime',
								core.review === 'in_review' && 'border-orange/50 text-orange'
							]}>{REVIEW[core.review]}</span
						>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
