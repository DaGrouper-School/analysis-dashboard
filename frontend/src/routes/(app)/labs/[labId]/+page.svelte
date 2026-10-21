<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteMap } from 'svelte/reactivity';
	import Icon from '#lib/components/Icon.svelte';
	import RegisterSensorModal from '#lib/components/RegisterSensorModal.svelte';
	import SetupSteps from '#lib/components/SetupSteps.svelte';
	import SyncModal from '#lib/components/SyncModal.svelte';
	import type { Reading } from '#lib/domain/types.ts';
	import { distanceKm } from '#lib/domain/validation.ts';
	import { formatPoint } from '#lib/geo.ts';
	import {
		deviceTypeColor,
		deviceTypeIcon,
		deviceTypeLabel,
		SITE_SETTING_LABELS
	} from '#lib/labels.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const lab = $derived(data.detail.lab);
	const devices = $derived(data.detail.devices);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	/** Each device's readings, newest first. Position belongs to the reading, not the device. */
	const readingsByDevice = $derived.by(() => {
		const byDevice = new SvelteMap<string, Reading[]>();
		for (const r of data.detail.readings)
			byDevice.set(r.deviceId, [...(byDevice.get(r.deviceId) ?? []), r]);
		return byDevice;
	});

	let registering = $state(false);
	let justAdded = $state('');
	let actionError = $state('');

	let syncing = $state(false);
	let syncId = $state('');

	function sync(deviceId: string) {
		syncId = deviceId;
		syncing = true;
	}

	function distanceLabel(km: number) {
		if (km < 1) return 'at the lab';
		return `${km < 10 ? km.toFixed(1) : Math.round(km)} km from the lab`;
	}

	function lastGps(deviceId: string) {
		return (
			readingsByDevice.get(deviceId)?.find((r) => r.locationSource === 'device')?.location ?? null
		);
	}

	async function remove(deviceId: string, deviceName: string) {
		if (!confirm(`Remove ${deviceName} from ${lab.name}? It will stop sending data to SAM.`))
			return;
		actionError = '';
		try {
			await services.labs.removeDevice(deviceId);
			await invalidate('app:lab');
		} catch (e) {
			actionError = errorMessage(e);
		}
	}
</script>

<svelte:head><title>{lab.name} · DaGrouper Analysis</title></svelte:head>

<a
	href={resolve('/(app)/classrooms/[id]', { id: lab.classroomId })}
	class="text-sm text-muted hover:text-ink">← {data.detail.classroomName}</a
>
<div class="mt-4">
	{#if data.onboarding}<SetupSteps current={3} />{/if}
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="font-display text-3xl sm:text-4xl">{lab.name}</h1>
			<p class="mt-1 text-muted">{lab.placeName}, {lab.country} · {formatPoint(lab.location)}</p>
		</div>
		{#if data.detail.canManage}
			<a class="btn btn-ghost" href={resolve('/(app)/labs/[labId]/edit', { labId: lab.id })}
				>Edit lab</a
			>
		{/if}
	</div>
</div>

<div class="mt-8 grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
	<div class="space-y-6">
		<section class="card">
			<h2 class="text-lg font-semibold">
				Devices <span class="text-muted">{devices.length}</span>
			</h2>
			<p class="mt-1 text-sm text-muted">
				Each device is registered in SAM as a source for this lab. Location comes with each reading,
				so a device can report from different places over time.
			</p>
			{#if actionError}<p class="field-error" role="alert">{actionError}</p>{/if}
			<ul class="mt-4 divide-y divide-line">
				{#each devices as device (device.id)}
					{@const readings = readingsByDevice.get(device.id) ?? []}
					<li class="py-4">
						<div class="flex flex-wrap items-start gap-3">
							<div class="min-w-0 flex-1">
								<p class="font-medium">{device.name}</p>
								<p class="text-sm text-muted">
									{readings.length
										? `Last reading ${dateTime.format(new Date(readings[0].observedAt))}`
										: 'No readings yet'}
								</p>
							</div>
							<span class="chip shrink-0"
								><span style:color={deviceTypeColor(data.deviceTypes, device.type)}
									><Icon
										name={deviceTypeIcon(data.deviceTypes, device.type)}
										class="size-3.5"
									/></span
								>
								{deviceTypeLabel(data.deviceTypes, device.type)}</span
							>
						</div>
						<dl class="mt-3 grid gap-x-6 gap-y-1 text-xs text-muted sm:grid-cols-2">
							<div>
								<dt class="inline">ID</dt>
								<dd class="inline font-mono text-ink">{device.identifier}</dd>
							</div>
							{#if device.samSourceId}
								<div>
									<dt class="inline">SAM source</dt>
									<dd class="inline font-mono break-all text-ink">{device.samSourceId}</dd>
								</div>
							{/if}
						</dl>
						{#if readings.length}
							<ul class="mt-3 space-y-1 text-xs" aria-label="Where recent readings were taken">
								{#each readings.slice(0, 3) as reading (reading.id)}
									<li class="flex flex-wrap items-center gap-x-2">
										<span class="text-muted">{dateTime.format(new Date(reading.observedAt))}</span>
										<span>{formatPoint(reading.location)}</span>
										{#if reading.locationSource === 'device'}
											<span class="text-muted">
												device GPS · {distanceLabel(distanceKm(lab.location, reading.location))}
											</span>
										{:else}
											<span
												class="rounded-full bg-raised px-2 py-0.5 font-medium text-ink ring-1 ring-line"
												title="The device sent no GPS with this reading."
												>Lab location, not from the device</span
											>
										{/if}
										{#if reading.creditedTo}
											<span class="text-muted">· recorded by {reading.creditedTo}</span>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
						{#if data.detail.canRecord || data.detail.canManage}
							<div class="mt-3 flex flex-wrap items-center gap-3">
								{#if data.detail.canRecord}
									<button
										type="button"
										class="btn btn-ghost px-3 py-1.5"
										aria-haspopup="dialog"
										onclick={() => sync(device.id)}
									>
										<Icon name="sync" class="size-4" /> Sync
									</button>
								{/if}
								{#if data.detail.canManage}
									<button
										type="button"
										class="text-xs font-medium text-muted hover:text-danger"
										onclick={() => remove(device.id, device.name)}>Remove device</button
									>
								{/if}
							</div>
						{/if}
					</li>
				{:else}
					<li class="py-3 text-sm text-muted">No devices registered yet.</li>
				{/each}
			</ul>
		</section>

		<section class="card">
			<h2 class="text-lg font-semibold">Site profile</h2>
			<p class="mt-1 text-sm text-muted">{SITE_SETTING_LABELS[lab.siteProfile.setting]}</p>
			<p class="mt-3">{lab.siteProfile.description}</p>
			<h3 class="mt-5 text-sm font-semibold">Local research questions</h3>
			<ul class="mt-2 list-disc space-y-1 pl-5 text-sm">
				{#each lab.siteProfile.researchQuestions as question (question)}
					<li>{question}</li>
				{/each}
			</ul>
		</section>
	</div>

	{#if data.detail.canManage}
		<div class="space-y-6 self-start">
			<section class="card">
				<h2 class="text-lg font-semibold">Register a sensor</h2>
				<p class="mt-1 text-sm text-muted">
					Pick its type, enter the serial on the sensor and pair it. It becomes a SAM source for
					this lab; its GPS position comes with each reading.
				</p>
				<button
					type="button"
					class="btn btn-primary mt-4"
					aria-haspopup="dialog"
					onclick={() => (registering = true)}
				>
					<Icon name="plus" class="size-4" /> Register a sensor
				</button>
				{#if justAdded}
					<p class="mt-3 text-sm text-muted" role="status">
						{justAdded} is registered. Add another, or
						<a class="link" href={resolve('/(app)/classrooms/[id]', { id: lab.classroomId })}
							>finish setup</a
						>.
					</p>
				{/if}
			</section>
		</div>
		<RegisterSensorModal
			bind:open={registering}
			labId={lab.id}
			labName={lab.name}
			deviceTypes={data.deviceTypes}
			domains={data.domains}
			parameters={data.parameters}
			onregistered={(device) => (justAdded = device.name)}
			onclose={() => invalidate('app:lab')}
		/>
	{/if}
</div>

{#if data.detail.canRecord && devices.length}
	<SyncModal
		bind:open={syncing}
		{devices}
		bind:deviceId={syncId}
		deviceTypes={data.deviceTypes}
		classroomName={data.detail.classroomName}
		labLocation={lab.location}
		{lastGps}
		parameters={data.parameters}
		onclose={() => invalidate('app:lab')}
	/>
{/if}
