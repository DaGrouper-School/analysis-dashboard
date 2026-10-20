<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import LocationInput from '#lib/components/LocationInput.svelte';
	import SetupSteps from '#lib/components/SetupSteps.svelte';
	import type { DeviceType, RegisterDeviceInput } from '#lib/domain/types.ts';
	import { distanceKm, validateDevice } from '#lib/domain/validation.ts';
	import { formatPoint, toOptionalGeoPoint } from '#lib/geo.ts';
	import { DEVICE_TYPE_LABELS, SITE_SETTING_LABELS } from '#lib/labels.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const lab = $derived(data.detail.lab);
	const devices = $derived(data.detail.devices);

	let name = $state('');
	let type = $state<DeviceType>('weather_station');
	let identifier = $state('');
	let placeName = $state('');
	// svelte-ignore state_referenced_locally
	let country = $state(data.detail.lab.country);
	let lat = $state('');
	let lng = $state('');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let justAdded = $state('');
	let actionError = $state('');

	function distanceLabel(km: number) {
		if (km < 1) return 'at the lab';
		return `${km < 10 ? km.toFixed(1) : Math.round(km)} km from the lab`;
	}

	async function register(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		justAdded = '';
		const input: RegisterDeviceInput = {
			name,
			type,
			identifier,
			placeName,
			country,
			location: toOptionalGeoPoint(lat, lng)
		};
		errors = validateDevice(input);
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			const device = await services.labs.registerDevice(lab.id, input);
			justAdded = device.name;
			name = identifier = placeName = lat = lng = '';
			saving = false;
			await invalidate('app:lab');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
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
	<h1 class="font-display text-5xl">{lab.name}</h1>
	<p class="mt-1 text-muted">{lab.placeName}, {lab.country} · {formatPoint(lab.location)}</p>
</div>

<div class="mt-8 grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
	<div class="space-y-6">
		<section class="card">
			<h2 class="text-lg font-semibold">
				Devices <span class="text-muted">{devices.length}</span>
			</h2>
			<p class="mt-1 text-sm text-muted">
				Each device reports from its own location and is registered in SAM as a source for this lab.
			</p>
			{#if actionError}<p class="field-error" role="alert">{actionError}</p>{/if}
			<ul class="mt-4 divide-y divide-line">
				{#each devices as device (device.id)}
					<li class="py-4">
						<div class="flex flex-wrap items-start gap-3">
							<div class="min-w-0 flex-1">
								<p class="font-medium">{device.name}</p>
								<p class="text-sm text-muted">
									{device.placeName}, {device.country} ·
									{device.ownLocation
										? distanceLabel(distanceKm(lab.location, device.location))
										: "uses the lab's location"}
								</p>
							</div>
							<span class="rounded-full bg-lime px-2.5 py-0.5 text-xs font-semibold">
								{DEVICE_TYPE_LABELS[device.type]}
							</span>
						</div>
						<dl class="mt-3 grid gap-x-6 gap-y-1 text-xs text-muted sm:grid-cols-3">
							<div>
								<dt class="inline">ID</dt>
								<dd class="inline font-mono text-ink">{device.identifier}</dd>
							</div>
							<div>
								<dt class="inline">GPS</dt>
								<dd class="inline text-ink">
									{device.ownLocation ? formatPoint(device.location) : 'Not set'}
								</dd>
							</div>
							{#if device.samSourceId}
								<div>
									<dt class="inline">SAM source</dt>
									<dd class="inline font-mono break-all text-ink">{device.samSourceId}</dd>
								</div>
							{/if}
						</dl>
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

	{#if data.detail.canManage}
		<section class="card self-start">
			<h2 class="text-lg font-semibold">Register a device</h2>
			<p class="mt-1 text-sm text-muted">
				A device can sit far from the lab, like a weather station in another town feeding this lab.
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
				<div class="grid grid-cols-2 gap-3">
					<div>
						<label class="label" for="device-place">Where it is</label>
						<input
							id="device-place"
							class={['input', errors.placeName && 'input-error']}
							placeholder="Shashamane"
							bind:value={placeName}
						/>
					</div>
					<div>
						<label class="label" for="device-country">Country</label>
						<input
							id="device-country"
							class={['input', errors.country && 'input-error']}
							bind:value={country}
						/>
					</div>
					{#if errors.placeName || errors.country}
						<p class="field-error col-span-2 mt-0">{errors.placeName || errors.country}</p>
					{/if}
				</div>
				<div>
					<p class="label">Device GPS location <span class="text-muted">(optional)</span></p>
					<p class="-mt-1 mb-2 text-xs text-muted">Leave blank to use the lab's location.</p>
					<LocationInput
						id="device-location"
						bind:lat
						bind:lng
						error={errors.location}
						copyFrom={lab.location}
					/>
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
</div>
