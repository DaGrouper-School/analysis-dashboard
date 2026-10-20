<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteMap } from 'svelte/reactivity';
	import RecordData from '#lib/components/RecordData.svelte';
	import SetupSteps from '#lib/components/SetupSteps.svelte';
	import type { DeviceType, Reading, RegisterDeviceInput } from '#lib/domain/types.ts';
	import { distanceKm, validateDevice } from '#lib/domain/validation.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { DEVICE_TYPE_COLORS, DEVICE_TYPE_LABELS, SITE_SETTING_LABELS } from '#lib/labels.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
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

	let name = $state('');
	let type = $state<DeviceType>('weather_station');
	let identifier = $state('');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let justAdded = $state('');
	let actionError = $state('');

	let recordId = $state('');

	function distanceLabel(km: number) {
		if (km < 1) return 'at the lab';
		return `${km < 10 ? km.toFixed(1) : Math.round(km)} km from the lab`;
	}

	async function register(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		justAdded = '';
		const input: RegisterDeviceInput = { name, type, identifier };
		errors = validateDevice(input);
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			const device = await services.labs.registerDevice(lab.id, input);
			justAdded = device.name;
			recordId = device.id;
			name = identifier = '';
			saving = false;
			await invalidate('app:lab');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
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
								><span
									class="size-2 rounded-full"
									style:background={DEVICE_TYPE_COLORS[device.type]}
								></span>
								{DEVICE_TYPE_LABELS[device.type]}</span
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
						{#if data.detail.canManage}
							<button
								class="mt-3 text-xs font-medium text-muted hover:text-danger"
								onclick={() => remove(device.id, device.name)}>Remove device</button
							>
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

	{#if data.detail.canManage || data.detail.canRecord}
		<div class="space-y-6 self-start">
			{#if data.detail.canManage}
				<section class="card">
					<h2 class="text-lg font-semibold">Register a device</h2>
					<p class="mt-1 text-sm text-muted">
						A device can sit far from the lab, like a weather station in another town feeding this
						lab. Its GPS position is sent with each reading, not set here.
					</p>
					<form class="mt-4 space-y-4" onsubmit={register} novalidate>
						<div>
							<label class="label" for="device-name">Device name</label>
							<input
								id="device-name"
								class={['input', errors.name && 'input-error']}
								placeholder="Shashamane Weather Station"
								bind:value={name}
							/>
							{#if errors.name}<p class="field-error">{errors.name}</p>{/if}
						</div>
						<div class="grid grid-cols-2 gap-3">
							<div>
								<label class="label" for="device-type">Type</label>
								<select id="device-type" class="input" bind:value={type}>
									{#each Object.entries(DEVICE_TYPE_LABELS) as [value, label] (value)}
										<option {value}>{label}</option>
									{/each}
								</select>
							</div>
							<div>
								<label class="label" for="identifier">Identifier</label>
								<input
									id="identifier"
									class={['input font-mono', errors.identifier && 'input-error']}
									placeholder="TWS-2041-SH"
									bind:value={identifier}
								/>
							</div>
							{#if errors.identifier}
								<p class="field-error col-span-2 mt-0">{errors.identifier}</p>
							{/if}
						</div>
						{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}
						<button class="btn btn-primary" disabled={saving}>
							{saving ? 'Registering…' : 'Register device'}
						</button>
						{#if justAdded}
							<p class="text-sm text-muted" role="status">
								{justAdded} is registered. Add another, or
								<a class="link" href={resolve('/(app)/classrooms/[id]', { id: lab.classroomId })}
									>finish setup</a
								>.
							</p>
						{/if}
					</form>
				</section>
			{/if}

			{#if data.detail.canRecord && devices.length}
				<RecordData
					{devices}
					classroomName={data.detail.classroomName}
					labLocation={lab.location}
					{lastGps}
					bind:deviceId={recordId}
					onfinish={() => invalidate('app:lab')}
				/>
			{/if}
		</div>
	{/if}
</div>
