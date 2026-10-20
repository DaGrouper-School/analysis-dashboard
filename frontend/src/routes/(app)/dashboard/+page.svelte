<script lang="ts">
	import { resolve } from '$app/paths';
	import type { EnvironmentalDomain } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { DEVICE_TYPE_LABELS } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const firstName = $derived(data.user.fullName.split(' ')[0]);

	const DOMAINS: { id: EnvironmentalDomain; label: string; icon: string }[] = [
		{ id: 'AIR', label: 'Air', icon: '🌳' },
		{ id: 'RAIN', label: 'Rain', icon: '🌧️' },
		{ id: 'WIND', label: 'Wind', icon: '💨' },
		{ id: 'SUN', label: 'Sun', icon: '☀️' },
		{ id: 'WAVES', label: 'Waves', icon: '🌊' },
		{ id: 'LIGHTNING', label: 'Lightning', icon: '⚡️' }
	];

	// AP-06: current temperature, wind, rain and UV.
	const headline = [
		{ id: 'air_temperature', label: 'Temperature' },
		{ id: 'wind_avg', label: 'Wind' },
		{ id: 'precip_accum_local_day', label: 'Rain today' },
		{ id: 'uv', label: 'UV index' }
	];

	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const countByDomain = $derived(
		Object.fromEntries(
			DOMAINS.map((d) => [d.id, data.parameters.filter((p) => p.domain === d.id).length])
		)
	);

	function format(value: number | string | undefined, unit = '') {
		if (value === undefined) return '—';
		if (typeof value === 'string') return value;
		const rounded = Math.abs(value) >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
		return `${rounded}${unit && !unit.startsWith('°') ? ' ' : ''}${unit}`;
	}

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const reading = $derived(data.featured?.reading ?? null);
</script>

<svelte:head><title>Dashboard · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<p class="text-sm text-muted">Welcome back</p>
		<h1 class="font-display text-5xl">Hello, {firstName}</h1>
	</div>
	{#if data.featured}
		<p class="rounded-full border border-line bg-white px-4 py-2 text-sm">
			<span class="text-muted">{data.featured.classroomName}</span>
			<strong class="ml-1">{data.featured.device.labName}</strong>
		</p>
	{/if}
</div>

<section class="mt-8 grid gap-4 lg:grid-cols-3">
	{#if data.featured && reading}
		<div class="card bg-forest text-white lg:col-span-2">
			<div class="flex flex-wrap items-start justify-between gap-4">
				<div>
					<p class="text-sm text-white/60">Current conditions</p>
					<h2 class="text-xl font-semibold">{data.featured.device.name}</h2>
					<p class="mt-1 text-sm text-white/60">
						{formatPoint(reading.location)}
						{#if reading.locationSource === 'lab'}
							<span
								class="ml-1 rounded-full bg-white/10 px-2 py-0.5 text-xs text-white ring-1 ring-white/20"
								title="The device sent no GPS with this reading, so the lab's location is shown."
								>Lab location, not from the device</span
							>
						{:else}
							· device GPS
						{/if}
					</p>
				</div>
				<p class="text-right text-xs text-white/50">
					Observed {dateTime.format(new Date(reading.observedAt))}
				</p>
			</div>
			<dl class="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
				{#each headline as item (item.id)}
					<div class="rounded-2xl bg-white/5 p-4 ring-1 ring-white/10">
						<dt class="text-xs text-white/60">{item.label}</dt>
						<dd class="mt-2 text-3xl font-semibold text-lime">
							{format(reading.values[item.id], paramsById.get(item.id)?.unit)}
						</dd>
					</div>
				{/each}
			</dl>
		</div>
	{:else}
		<div class="card lg:col-span-2">
			<p class="text-sm text-muted">Current conditions</p>
			<h2 class="mt-1 text-xl font-semibold">No weather readings yet</h2>
			<p class="mt-2 text-sm text-muted">
				Conditions come only from devices in your classrooms' labs. Join a classroom with an invite
				link, or ask your facilitator to connect a weather station.
			</p>
			<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
		</div>
	{/if}

	<div class="card">
		<h2 class="font-semibold">Active alerts</h2>
		<p class="mt-2 text-sm text-muted">
			No alerts right now. Local threshold alerts arrive in a later sprint.
		</p>
	</div>
</section>

{#if data.others.length}
	<section class="mt-8">
		<h2 class="text-lg font-semibold">Other devices in your classrooms</h2>
		<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each data.others as item (item.device.id)}
				<li class="card p-4">
					<div class="flex items-start justify-between gap-2">
						<a
							class="font-medium hover:underline"
							href={resolve('/(app)/labs/[labId]', { labId: item.device.labId })}
							>{item.device.name}</a
						>
						<span class="rounded-full bg-lime px-2 py-0.5 text-xs font-semibold">
							{DEVICE_TYPE_LABELS[item.device.type]}
						</span>
					</div>
					<p class="mt-1 text-xs text-muted">
						{item.classroomName} · {dateTime.format(new Date(item.reading.observedAt))}
					</p>
					<p class="mt-2 text-xs">
						{formatPoint(item.reading.location)}
						{#if item.reading.locationSource === 'lab'}
							<span class="text-muted">· lab location, not from the device</span>
						{/if}
					</p>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<section class="mt-8">
	<h2 class="text-lg font-semibold">Environmental domains</h2>
	<p class="text-sm text-muted">
		Weather station parameters, grouped by domain. Detailed views come in sprint 3.
	</p>
	<ul class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
		{#each DOMAINS as domain (domain.id)}
			<li class="card p-4">
				<span class="text-2xl" aria-hidden="true">{domain.icon}</span>
				<p class="mt-3 font-semibold">{domain.label}</p>
				<p class="text-xs text-muted">{countByDomain[domain.id]} parameters</p>
			</li>
		{/each}
	</ul>
</section>
