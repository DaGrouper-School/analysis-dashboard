<script lang="ts">
	import type { EnvironmentalDomain } from '#lib/domain/types.ts';
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

	const observed = $derived(
		data.snapshot
			? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(
					new Date(data.snapshot.observedAt)
				)
			: null
	);
</script>

<svelte:head><title>Dashboard · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<p class="text-sm text-muted">Welcome back</p>
		<h1 class="font-display text-5xl">Hello, {firstName}</h1>
	</div>
	{#if data.station}
		<p class="rounded-full border border-line bg-white px-4 py-2 text-sm">
			<span class="text-muted">Your station</span>
			<strong class="ml-1">{data.station.name}</strong>
		</p>
	{/if}
</div>

<section class="mt-8 grid gap-4 lg:grid-cols-3">
	<div class="card bg-forest text-white lg:col-span-2">
		<div class="flex items-start justify-between gap-4">
			<div>
				<p class="text-sm text-white/60">Current conditions</p>
				<h2 class="text-xl font-semibold">{data.station?.siteName}, {data.station?.country}</h2>
			</div>
			{#if observed}<p class="text-right text-xs text-white/50">Observed {observed}</p>{/if}
		</div>
		<dl class="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
			{#each headline as item (item.id)}
				<div class="rounded-2xl bg-white/5 p-4 ring-1 ring-white/10">
					<dt class="text-xs text-white/60">{item.label}</dt>
					<dd class="mt-2 text-3xl font-semibold text-lime">
						{format(data.snapshot?.values[item.id], paramsById.get(item.id)?.unit)}
					</dd>
				</div>
			{/each}
		</dl>
	</div>

	<div class="card">
		<h2 class="font-semibold">Active alerts</h2>
		<p class="mt-2 text-sm text-muted">
			No alerts for this station. Local threshold alerts arrive in a later sprint.
		</p>
	</div>
</section>

<section class="mt-8">
	<h2 class="text-lg font-semibold">Environmental domains</h2>
	<p class="text-sm text-muted">
		The station's parameters, grouped by domain. Detailed views come in sprint 3.
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
