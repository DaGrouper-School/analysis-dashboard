<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { formatPoint } from '#lib/geo.ts';
	import { describeParameter, keySummary } from '#lib/key-values.ts';
	import { deviceTypeLabel } from '#lib/labels.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const device = $derived(data.device);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const back = $derived(
		resolve('/(app)/dashboard') + (data.classroom ? sensorQuery(data.classroom.id, device?.id) : '')
	);

	function chooseDevice(deviceId: string) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(resolve('/(app)/dashboard/readings') + sensorQuery(data.classroom!.id, deviceId), {
			replace: true,
			reset: false
		});
	}
</script>

<svelte:head><title>Raw sensor readings · DaGrouper Analysis</title></svelte:head>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
<a href={back} class="text-sm text-muted hover:text-ink">← Dashboard</a>

<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-3xl sm:text-4xl">Raw sensor readings</h1>
		{#if device && data.classroom}
			<p class="mt-1 text-muted">
				{device.name} · {deviceTypeLabel(data.deviceTypes, device.type)} · {data.classroom.name}
			</p>
		{/if}
	</div>
	{#if data.classroom && data.devices.length > 1}
		<div class="w-full sm:w-72">
			<label class="label" for="raw-device">Sensor</label>
			<select
				id="raw-device"
				class="input"
				value={device?.id ?? ''}
				onchange={(e) => chooseDevice(e.currentTarget.value)}
			>
				{#each data.devices as option (option.id)}
					<option value={option.id}>{option.name}</option>
				{/each}
			</select>
		</div>
	{/if}
</div>

{#if !device}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">No sensor to show</h2>
		<p class="mt-2 text-sm text-muted">
			Readings come only from devices in your classrooms' labs. Join a classroom, or ask your
			facilitator to register a device.
		</p>
	</div>
{:else if !data.readings.length}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">No readings from {device.name} yet</h2>
		<p class="mt-2 text-sm text-muted">Readings show here once the device sends data.</p>
	</div>
{:else}
	<p class="mt-6 text-sm text-muted">
		The latest {data.readings.length} reading{data.readings.length === 1 ? '' : 's'}, newest first,
		exactly as the device sent them. Open a reading for every value it carries.
	</p>
	<div class="card mt-4 overflow-x-auto p-0">
		<table class="w-full min-w-[44rem] text-left text-sm">
			<thead class="text-xs text-muted">
				<tr class="border-b border-line">
					<th class="px-4 py-3 font-medium">Observed</th>
					<th class="px-4 py-3 font-medium">Values</th>
					<th class="px-4 py-3 font-medium">Location</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each data.readings as reading (reading.id)}
					{@const values = Object.entries(reading.values)}
					<tr class="align-top">
						<td class="px-4 py-3 whitespace-nowrap">
							{dateTime.format(new Date(reading.observedAt))}
							<p class="text-xs text-muted">
								Received {dateTime.format(new Date(reading.receivedAt))}
							</p>
							{#if reading.creditedTo}
								<p class="text-xs text-muted">Recorded by {reading.creditedTo}</p>
							{/if}
						</td>
						<td class="px-4 py-3">
							<details>
								<summary class="cursor-pointer">
									{keySummary(device.type, reading.values, paramsById, 4)}
									<span class="ml-1 text-xs text-muted"
										>· all {values.length} value{values.length === 1 ? '' : 's'}</span
									>
								</summary>
								<table class="mt-3 w-full text-xs">
									<thead class="text-muted">
										<tr>
											<th class="py-1 pr-4 font-medium">Parameter</th>
											<th class="py-1 pr-4 font-medium">ID</th>
											<th class="py-1 text-right font-medium">Value</th>
										</tr>
									</thead>
									<tbody>
										{#each values as [id, value] (id)}
											{@const param = describeParameter(id, paramsById)}
											<tr class="border-t border-line">
												<td class="py-1 pr-4">{param.label}</td>
												<td class="py-1 pr-4 font-mono text-muted">{id}</td>
												<td class="py-1 text-right font-mono tabular-nums">
													{value}{param.unit ? ` ${param.unit}` : ''}
												</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</details>
						</td>
						<td class="px-4 py-3 text-xs">
							{formatPoint(reading.location)}
							<p class="text-muted">
								{reading.locationSource === 'device'
									? 'Device GPS'
									: 'Lab location, not from the device'}
							</p>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
