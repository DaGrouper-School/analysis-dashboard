<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import ReadingChart from '#lib/components/ReadingChart.svelte';
	import { thresholdStatus } from '#lib/domain/alerts.ts';
	import { TIME_RANGES, withinRange } from '#lib/domains.ts';
	import { describeParameter, formatValue } from '#lib/key-values.ts';
	import { DEVICE_TYPE_LABELS } from '#lib/labels.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const device = $derived(data.device);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const latest = $derived(data.readings[0] ?? null);
	const shown = $derived(withinRange(data.readings, data.range));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const BADGE = {
		normal: 'bg-lime/15 text-lime ring-lime/40',
		caution: 'bg-orange/15 text-orange ring-orange/40',
		warning: 'bg-danger/15 text-danger ring-danger/40'
	};

	function address(change: { device?: string; domain?: string; range?: string }) {
		let query = sensorQuery(data.classroom!.id, change.device ?? device?.id);
		const domain = change.device ? undefined : (change.domain ?? data.tab?.id);
		if (domain) query += `&domain=${encodeURIComponent(domain)}`;
		query += `&range=${encodeURIComponent(change.range ?? data.range)}`;
		return resolve('/(app)/dashboard/domains') + query;
	}

	function go(change: { device?: string; domain?: string; range?: string }) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(address(change), { replace: true, reset: false });
	}

	const back = $derived(
		resolve('/(app)/dashboard') + (data.classroom ? sensorQuery(data.classroom.id, device?.id) : '')
	);
</script>

<svelte:head>
	<title>{data.tab ? `${data.tab.label} · ` : ''}Domains · DaGrouper Analysis</title>
</svelte:head>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
<a href={back} class="text-sm text-muted hover:text-ink">← Dashboard</a>

<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-3xl sm:text-4xl">Environmental domains</h1>
		{#if device && data.classroom}
			<p class="mt-1 text-muted">
				{device.name} · {DEVICE_TYPE_LABELS[device.type]} · {data.classroom.name}
			</p>
		{/if}
	</div>
	{#if data.classroom && data.devices.length > 1}
		<div class="w-full sm:w-72">
			<label class="label" for="domain-device">Sensor</label>
			<select
				id="domain-device"
				class="input"
				value={device?.id ?? ''}
				onchange={(e) => go({ device: e.currentTarget.value })}
			>
				{#each data.devices as option (option.id)}
					<option value={option.id}>{option.name}</option>
				{/each}
			</select>
		</div>
	{/if}
</div>

{#if !device || !data.tab}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">
			{device ? `No readings from ${device.name} yet` : 'No sensor to show'}
		</h2>
		<p class="mt-2 text-sm text-muted">
			{device
				? 'Domains show here once the sensor sends data.'
				: 'Domain views open for sensors in your own classrooms. Join a classroom to see its sensors.'}
		</p>
	</div>
{:else}
	<div class="mt-6 flex flex-wrap items-center justify-between gap-3">
		<div
			class="flex max-w-full gap-1 overflow-x-auto border-b border-line"
			role="tablist"
			aria-label="Environmental domains"
		>
			{#each data.tabs as tab (tab.id)}
				{@const active = tab.id === data.tab.id}
				<button
					type="button"
					role="tab"
					id="tab-{tab.id}"
					aria-selected={active}
					aria-controls="domain-panel"
					class={[
						'-mb-px shrink-0 border-b-2 px-3 py-2 text-sm whitespace-nowrap transition',
						active
							? 'border-ink font-medium text-ink'
							: 'border-transparent text-muted hover:text-ink'
					]}
					onclick={() => go({ domain: tab.id })}
				>
					<span aria-hidden="true">{tab.icon}</span>
					{tab.label}
					<span class="text-xs text-muted">{tab.parameterIds.length}</span>
				</button>
			{/each}
		</div>
		<div
			class="flex rounded-full border border-line p-0.5 text-sm"
			role="group"
			aria-label="Time range"
		>
			{#each TIME_RANGES as option (option.id)}
				<button
					type="button"
					class={[
						'rounded-full px-3 py-1 whitespace-nowrap transition',
						option.id === data.range ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'
					]}
					aria-pressed={option.id === data.range}
					onclick={() => go({ range: option.id })}>{option.label}</button
				>
			{/each}
		</div>
	</div>

	<p class="mt-3 text-xs text-muted">
		{shown.length} reading{shown.length === 1 ? '' : 's'} in the last {TIME_RANGES.find(
			(r) => r.id === data.range
		)!.label} of data{#if latest}, up to {dateTime.format(new Date(latest.observedAt))}{/if}.
	</p>

	<div
		id="domain-panel"
		role="tabpanel"
		aria-labelledby="tab-{data.tab.id}"
		class="mt-4 grid gap-4 lg:grid-cols-2"
	>
		{#each data.tab.parameterIds as id (id)}
			{@const param = describeParameter(id, paramsById)}
			{@const value = latest?.values[id]}
			{@const status = thresholdStatus(id, value)}
			<section class="card" aria-labelledby="param-{id}">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div class="min-w-0">
						<h2 id="param-{id}" class="font-medium">{param.label}</h2>
						{#if paramsById.get(id)?.definition}
							<p class="text-xs text-muted">{paramsById.get(id)?.definition}</p>
						{/if}
					</div>
					{#if status}
						<span
							class={['rounded-full px-2 py-0.5 text-xs font-medium ring-1', BADGE[status.level]]}
						>
							{#if status.level !== 'normal'}<span aria-hidden="true">▲</span>{/if}
							{status.title}
						</span>
					{:else}
						<span class="rounded-full px-2 py-0.5 text-xs text-muted ring-1 ring-line">
							No threshold
						</span>
					{/if}
				</div>
				<p class="mt-3 text-3xl text-ink">{formatValue(value, param.unit)}</p>
				<div class="mt-3">
					<ReadingChart
						readings={shown}
						parameterId={id}
						label={param.label}
						unit={param.unit}
						height={160}
					/>
				</div>
			</section>
		{/each}
	</div>
{/if}
