<script lang="ts">
	import { resolve } from '$app/paths';
	import DeviceMap from '#lib/components/DeviceMap.svelte';
	import { formatPoint } from '#lib/geo.ts';
	import { DEVICE_TYPE_LABELS } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// svelte-ignore state_referenced_locally
	let selectedId = $state(
		data.conditions.find((c) => c.device.id === data.selected)?.device.id ??
			data.conditions.find((c) => c.device.type === 'weather_station')?.device.id ??
			data.conditions[0]?.device.id ??
			''
	);
	const selected = $derived(data.conditions.find((c) => c.device.id === selectedId) ?? null);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	// AP-06: current temperature, wind, rain and UV.
	const headline = [
		{ id: 'air_temperature', label: 'Temperature' },
		{ id: 'wind_avg', label: 'Wind' },
		{ id: 'precip_accum_local_day', label: 'Rain today' },
		{ id: 'uv', label: 'UV index' }
	];
	const HEADLINE_IDS = new Set(headline.map((h) => h.id));

	function format(value: number | string | undefined, unit = '') {
		if (value === undefined) return '—';
		if (typeof value === 'string') return value;
		const rounded = Math.abs(value) >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
		return `${rounded}${unit && !unit.startsWith('°') ? ' ' : ''}${unit}`;
	}

	function label(id: string) {
		return (
			paramsById.get(id)?.label ?? id.replaceAll('_', ' ').replace(/^./, (c) => c.toUpperCase())
		);
	}

	/** Devices without weather parameters (soil, rain gauge) show their own values instead. */
	const otherValues = $derived(
		selected && !headline.some((h) => selected.reading.values[h.id] !== undefined)
			? Object.entries(selected.reading.values).filter(([id]) => !HEADLINE_IDS.has(id))
			: []
	);
</script>

<svelte:head><title>Map · DaGrouper Analysis</title></svelte:head>

<h1 class="font-display text-5xl">Device map</h1>
<p class="mt-1 max-w-2xl text-muted">
	Devices in your classrooms, placed where their latest reading was taken. Hollow, dashed markers
	have no GPS in that reading and sit at the lab instead.
</p>

{#if data.conditions.length}
	<div class="mt-6 grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
		<div>
			<DeviceMap items={data.conditions} bind:selectedId />
			<ul class="mt-3 flex flex-wrap gap-4 text-xs text-muted" aria-label="Map legend">
				<li class="flex items-center gap-1.5">
					<span class="size-3 rounded-full border-2 border-forest bg-lime"></span> Device GPS
				</li>
				<li class="flex items-center gap-1.5">
					<span class="size-3 rounded-full border-2 border-forest bg-[#f2a33a]"></span> Active alert
				</li>
				<li class="flex items-center gap-1.5">
					<span class="size-3 rounded-full border-2 border-dashed border-forest bg-white"></span>
					Lab location, not from the device
				</li>
			</ul>
		</div>

		<div class="space-y-4">
			{#if selected}
				<section class="card bg-forest text-white" aria-live="polite">
					<p class="text-sm text-white/60">Current conditions</p>
					<h2 class="text-xl font-semibold">{selected.device.name}</h2>
					<p class="mt-1 text-sm text-white/60">
						{selected.classroomName} ·
						<a
							class="underline-offset-2 hover:underline"
							href={resolve('/(app)/labs/[labId]', { labId: selected.device.labId })}
							>{selected.device.labName}</a
						>
					</p>
					<p class="mt-3 text-xs text-white/70">
						Observed {dateTime.format(new Date(selected.reading.observedAt))}
					</p>
					<p class="mt-1 text-xs">
						{formatPoint(selected.reading.location)}
						{#if selected.reading.locationSource === 'lab'}
							<span class="ml-1 rounded-full bg-white/10 px-2 py-0.5 ring-1 ring-white/20"
								>Lab location, not from the device</span
							>
						{:else}
							<span class="text-white/60">· device GPS</span>
						{/if}
					</p>
					<dl class="mt-5 grid grid-cols-2 gap-3">
						{#each headline as item (item.id)}
							<div class="rounded-2xl bg-white/5 p-3 ring-1 ring-white/10">
								<dt class="text-xs text-white/60">{item.label}</dt>
								<dd class="mt-1 text-2xl font-semibold text-lime">
									{format(selected.reading.values[item.id], paramsById.get(item.id)?.unit)}
								</dd>
							</div>
						{/each}
					</dl>
					{#if otherValues.length}
						<dl class="mt-4 space-y-1 text-sm">
							{#each otherValues as [id, value] (id)}
								<div class="flex justify-between gap-3">
									<dt class="text-white/60">{label(id)}</dt>
									<dd>{format(value, paramsById.get(id)?.unit)}</dd>
								</div>
							{/each}
						</dl>
					{/if}
				</section>

				<section class="card">
					<h2 class="font-semibold">Active alerts</h2>
					{#if selected.alerts.length}
						<ul class="mt-3 space-y-2">
							{#each selected.alerts as alert (alert.parameterId)}
								<li
									class={[
										'rounded-2xl p-3 text-sm ring-1',
										alert.level === 'warning'
											? 'bg-danger/10 ring-danger/30'
											: 'bg-[#f2a33a]/15 ring-[#f2a33a]/40'
									]}
								>
									<p class="font-semibold">{alert.title}</p>
									<p class="text-muted">{alert.message}</p>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="mt-2 text-sm text-muted">No active alerts for this device.</p>
					{/if}
				</section>
			{/if}

			<section class="card p-4">
				<h2 class="px-1 text-sm font-semibold">Devices {data.conditions.length}</h2>
				<ul class="mt-2 space-y-1">
					{#each data.conditions as item (item.device.id)}
						<li>
							<button
								type="button"
								class={[
									'flex w-full items-center gap-3 rounded-2xl px-3 py-2 text-left text-sm transition',
									item.device.id === selectedId ? 'bg-lime/40' : 'hover:bg-canvas'
								]}
								aria-pressed={item.device.id === selectedId}
								onclick={() => (selectedId = item.device.id)}
							>
								<span class="min-w-0 flex-1">
									<span class="block truncate font-medium">{item.device.name}</span>
									<span class="block text-xs text-muted">
										{DEVICE_TYPE_LABELS[item.device.type]}
										{#if item.reading.locationSource === 'lab'}· lab location{/if}
									</span>
								</span>
								{#if item.alerts.length}
									<span class="rounded-full bg-[#f2a33a] px-2 py-0.5 text-xs font-semibold">
										{item.alerts.length} alert{item.alerts.length > 1 ? 's' : ''}
									</span>
								{/if}
							</button>
						</li>
					{/each}
				</ul>
			</section>
		</div>
	</div>
{:else}
	<div class="card mt-6 max-w-2xl">
		<h2 class="text-xl font-semibold">No devices to show</h2>
		<p class="mt-2 text-sm text-muted">
			The map shows devices in your classrooms' labs once they have sent a reading. Join a classroom
			with an invite link, or ask your facilitator to connect a device.
		</p>
		<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
	</div>
{/if}
