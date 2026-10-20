<script lang="ts">
	import { resolve } from '$app/paths';
	import type { EnvironmentalDomain } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import { DEVICE_TYPE_COLORS, DEVICE_TYPE_LABELS } from '#lib/labels.ts';
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

	const SUMMARY_FIRST = [
		'air_temperature',
		'relative_humidity',
		'wind_avg',
		'precip_accum_local_day',
		'uv'
	];

	/** A few values with units, for compact lists. Unknown parameters show their ID as a label. */
	function summary(values: Record<string, number | string>, count = 3) {
		// Headline parameters lead, so a weather station shows temperature before air density.
		const rank = (id: string) => {
			const i = SUMMARY_FIRST.indexOf(id);
			return i === -1 ? SUMMARY_FIRST.length : i;
		};
		return Object.entries(values)
			.sort(([a], [b]) => rank(a) - rank(b))
			.slice(0, count)
			.map(([id, value]) => {
				const param = paramsById.get(id);
				const label = param?.label ?? id.replaceAll('_', ' ').replace(/^./, (c) => c.toUpperCase());
				return `${label} ${format(value, param?.unit)}`;
			})
			.join(' · ');
	}

	const markers = $derived<GlobeMarker[]>(
		[data.featured, ...data.others]
			.filter((c) => c !== null)
			.map((c) => ({
				id: c.device.id,
				lat: c.reading.location.lat,
				lng: c.reading.location.lng,
				color: DEVICE_TYPE_COLORS[c.device.type],
				hollow: c.reading.locationSource === 'lab',
				label: c.device.name
			}))
	);

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const reading = $derived(data.featured?.reading ?? null);
</script>

<svelte:head><title>Dashboard · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<p class="text-sm text-muted">Welcome back</p>
		<h1 class="font-display text-3xl sm:text-4xl">Hello, {firstName}</h1>
	</div>
	{#if data.featured}
		<p class="rounded-full border border-line-strong px-4 py-2 text-sm">
			<span class="text-muted">{data.featured.classroomName}</span>
			<strong class="ml-1">{data.featured.device.labName}</strong>
		</p>
	{/if}
</div>

{#if data.invitations.length}
	<div class="mt-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

<section class="mt-8 grid gap-4 lg:grid-cols-3">
	{#if data.featured && reading}
		<div class="card border-teal/25 bg-forest text-ink lg:col-span-2">
			<div class="flex flex-wrap items-start justify-between gap-4">
				<div>
					<p class="text-sm text-muted">Current conditions</p>
					<h2 class="text-xl font-semibold">{data.featured.device.name}</h2>
					<p class="mt-1 text-sm text-muted">
						{formatPoint(reading.location)}
						{#if reading.locationSource === 'lab'}
							<span
								class="ml-1 rounded-full bg-ink/10 px-2 py-0.5 text-xs text-ink ring-1 ring-line-strong"
								title="The device sent no GPS with this reading, so the lab's location is shown."
								>Lab location, not from the device</span
							>
						{:else}
							· device GPS
						{/if}
					</p>
				</div>
				<p class="text-right text-xs text-muted">
					Observed {dateTime.format(new Date(reading.observedAt))}
				</p>
			</div>
			<dl class="mt-8 grid grid-cols-2 gap-px border border-line bg-line sm:grid-cols-4">
				{#each headline as item (item.id)}
					<div class="bg-forest p-4">
						<dt class="text-xs text-muted">{item.label}</dt>
						<dd class="mt-2 text-3xl text-ink">
							{format(reading.values[item.id], paramsById.get(item.id)?.unit)}
						</dd>
					</div>
				{/each}
			</dl>
			<div class="mt-6">
				<h3 class="text-sm font-medium">Active alerts</h3>
				{#if data.featured.alerts.length}
					<ul class="mt-2 space-y-2">
						{#each data.featured.alerts as alert (alert.parameterId)}
							<li
								class={[
									'border-l-2 bg-ink/[0.04] py-2 pr-2 pl-3 text-sm',
									alert.level === 'warning' ? 'border-danger' : 'border-orange'
								]}
							>
								<span class="text-ink">
									<span aria-hidden="true">{alert.level === 'warning' ? '▲▲' : '▲'}</span>
									{alert.title}</span
								>
								<span class="text-muted">· {alert.message}</span>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-1 text-sm text-muted">No active alerts for this station.</p>
				{/if}
			</div>
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

	<a
		href={resolve('/(app)/map')}
		class="card group relative flex min-h-80 flex-col overflow-hidden p-0 transition hover:border-line-strong"
	>
		<div class="flex items-baseline justify-between gap-3 p-5 pb-0">
			<h2 class="font-medium">Your devices</h2>
			<span class="text-sm text-muted group-hover:text-ink">Open the map →</span>
		</div>
		<div class="pointer-events-none relative min-h-64 flex-1">
			<Globe {markers} ariaLabel="Globe of your classrooms' devices" />
		</div>
	</a>
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
						<span class="chip shrink-0"
							><span
								class="size-2 rounded-full"
								style:background={DEVICE_TYPE_COLORS[item.device.type]}
							></span>
							{DEVICE_TYPE_LABELS[item.device.type]}</span
						>
					</div>
					<p class="mt-1 text-xs text-muted">
						{item.classroomName} · {dateTime.format(new Date(item.reading.observedAt))}
					</p>
					<p class="mt-2 text-sm">{summary(item.reading.values)}</p>
					<p class="mt-1 text-xs">
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

{#if data.recent.length}
	<section class="mt-8">
		<h2 class="text-lg font-semibold">Recent readings</h2>
		<p class="text-sm text-muted">The latest data recorded by devices in your classrooms.</p>
		<div class="card mt-4 overflow-x-auto p-0">
			<table class="w-full min-w-[40rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						<th class="px-4 py-3 font-medium">Observed</th>
						<th class="px-4 py-3 font-medium">Device</th>
						<th class="px-4 py-3 font-medium">Values</th>
						<th class="px-4 py-3 font-medium">Location</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each data.recent as item (item.reading.id)}
						<tr class="align-top">
							<td class="px-4 py-3 whitespace-nowrap">
								{dateTime.format(new Date(item.reading.observedAt))}
							</td>
							<td class="px-4 py-3">
								<a
									class="font-medium hover:underline"
									href={resolve('/(app)/labs/[labId]', { labId: item.device.labId })}
									>{item.device.name}</a
								>
								<p class="text-xs text-muted">{item.classroomName}</p>
							</td>
							<td class="px-4 py-3">{summary(item.reading.values)}</td>
							<td class="px-4 py-3 text-xs">
								{formatPoint(item.reading.location)}
								<p class="text-muted">
									{item.reading.locationSource === 'device'
										? 'Device GPS'
										: 'Lab location, not from the device'}
								</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
{/if}

<section class="mt-8">
	<h2 class="text-lg font-semibold">Environmental domains</h2>
	<p class="text-sm text-muted">Weather station parameters, grouped by domain.</p>
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
