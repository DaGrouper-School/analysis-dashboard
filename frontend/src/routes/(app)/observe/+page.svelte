<script lang="ts">
	import { resolve } from '$app/paths';
	import type { GeoPoint, ObservationTemplate } from '#lib/domain/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import SyncStatus from '#lib/features/devices/SyncStatus.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import { deviceTypeColor, deviceTypeIcon } from '#lib/utils/labels.ts';
	import { readEntries, type FieldEntry } from '#lib/features/measurements/observations.ts';
	import { outbox } from '#lib/features/devices/outbox.svelte.ts';
	import type { OutboxEntry } from '#lib/features/devices/outbox.ts';
	import { contributionBlock } from '#lib/domain/consent.ts';
	import { errorMessage } from '#lib/services/index.ts';
	import ObserveDevice from './ObserveDevice.svelte';
	import ObserveReview from './ObserveReview.svelte';
	import ObserveSent from './ObserveSent.svelte';
	import ObserveValues from './ObserveValues.svelte';
	import ObserveWhat from './ObserveWhat.svelte';
	import {
		deviceProblem,
		localNow,
		observeSteps,
		STEP_INFO,
		templateGroups,
		timeProblem,
		type GpsState
	} from './observe-form.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// A minor without a guardian's consent cannot send anything (AP-04); the service refuses too.
	const blocked = $derived(contributionBlock(data.user));

	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));

	// Start from what the dashboard or lab page was showing, when it was passed along.
	// svelte-ignore state_referenced_locally
	const askedDevice = data.targets
		.flatMap((t) => t.devices)
		.find((d) => d.id === data.asked.device);
	// svelte-ignore state_referenced_locally
	let labId = $state(
		data.targets.find((t) => t.lab.id === (data.asked.lab ?? askedDevice?.labId))?.lab.id ??
			data.targets[0]?.lab.id ??
			''
	);
	// svelte-ignore state_referenced_locally
	let templateId = $state(
		data.templates.find((t) => t.id === data.asked.template)?.id ??
			data.templates.find((t) => askedDevice && t.deviceType === askedDevice.type)?.id ??
			''
	);
	// svelte-ignore state_referenced_locally
	let deviceId = $state(askedDevice?.labId === labId ? askedDevice.id : '');

	// Opened for one sensor ("Enter measurement"): its form only, with nothing else to pick.
	// svelte-ignore state_referenced_locally
	const locked =
		!!askedDevice &&
		askedDevice.labId === labId &&
		data.templates.some((t) => t.id === templateId && t.deviceType === askedDevice.type);

	const target = $derived(data.targets.find((t) => t.lab.id === labId) ?? null);
	const template = $derived(data.templates.find((t) => t.id === templateId) ?? null);
	/** The lab's registered devices this template is read from. Nothing else can be picked. */
	const fitting = $derived(
		template?.deviceType && target
			? target.devices.filter((d) => d.type === template.deviceType)
			: []
	);
	const device = $derived(
		fitting.length === 1 ? fitting[0] : (fitting.find((d) => d.id === deviceId) ?? null)
	);

	const groups = $derived(templateGroups(data.domains, data.templates));

	let entries = $state<Record<string, FieldEntry>>({});
	let observedAt = $state(localNow());
	let location = $state<GeoPoint | null>(null);
	let gps = $state<GpsState>('idle');
	let touched = $state<Record<string, boolean>>({});
	let tried = $state(false);
	let triedDevice = $state(false);
	let triedWhat = $state(false);
	let step = $state(0);
	let sending = $state(false);
	let serverErrors = $state<Record<string, string>>({});
	let formError = $state('');
	/** The observation just saved: synced, or waiting on this device. */
	let sent = $state<OutboxEntry | null>(null);

	/** A fresh form for the picked template: empty values, each in its own unit. */
	function reset(next: ObservationTemplate | null) {
		entries = Object.fromEntries(
			(next?.fields ?? []).map((f) => [
				f.parameterId,
				{ value: '', unit: paramsById.get(f.parameterId)?.unit ?? '' }
			])
		);
		touched = {};
		tried = triedDevice = false;
		serverErrors = {};
		formError = '';
	}
	// svelte-ignore state_referenced_locally
	reset(template);

	function pick(id: string) {
		templateId = id;
		deviceId = '';
		sent = null;
		reset(data.templates.find((t) => t.id === id) ?? null);
	}

	const checked = $derived(
		template ? readEntries(template, data.parameters, entries) : { values: {}, errors: {} }
	);
	const deviceError = $derived(
		template ? deviceProblem(template, fitting, device, data.deviceTypes) : ''
	);
	const timeError = $derived(timeProblem(observedAt));
	/** Shown once a field was left, or after Next was tried. The server's word wins. */
	const shownError = (id: string) =>
		serverErrors[id] ?? (tried || touched[id] ? checked.errors[id] : undefined);

	const stepIds = $derived(observeSteps(locked, !!template?.deviceType));
	const steps = $derived(stepIds.map((id) => STEP_INFO[id]));
	const stepId = $derived(stepIds[step]);

	/** Whether a step is ready to leave; shows its errors and focuses the first one if not. */
	function check(at: number) {
		const id = stepIds[at];
		let first = '';
		if (id === 'what') {
			triedWhat = true;
			if (!template || !target) first = 'observe-what';
		} else if (id === 'device') {
			triedDevice = true;
			if (deviceError) first = 'observe-device';
		} else if (id === 'values') {
			tried = true;
			const field = Object.keys(checked.errors)[0];
			if (field) first = `field-${field}`;
			else if (timeError) first = 'observe-time';
		} else {
			// Review: send the person back to the first step that still has a problem.
			const back = stepIds.findIndex((s, i) => i < at && !check(i));
			if (back >= 0) {
				step = back;
				return false;
			}
		}
		if (first) document.getElementById(first)?.focus();
		return !first;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!template || !target) return;
		serverErrors = {};
		formError = '';
		sending = true;
		try {
			// Saved on the device first, then sent; offline it waits there and syncs later.
			const entry = await outbox.save(
				{
					templateId: template.id,
					labId: target.lab.id,
					deviceId: device?.id ?? null,
					observedAt: new Date(observedAt).toISOString(),
					values: checked.values,
					location
				},
				// Credited by participant ID, never by name: a public profile is set in Settings.
				false
			);
			if (entry.status === 'failed') {
				// Refused while the person is still here: fix it in the form, not in the outbox.
				outbox.discard(entry.clientId);
				if (Object.keys(entry.fields).length) {
					serverErrors = entry.fields;
					step = stepIds.indexOf('values');
				}
				formError = entry.error ?? 'Not accepted.';
			} else {
				sent = entry;
			}
		} catch (e) {
			formError = errorMessage(e);
		} finally {
			sending = false;
		}
	}

	function another() {
		sent = null;
		observedAt = localNow();
		reset(template);
		// Same thing again: skip straight past picking it.
		step = locked ? 0 : 1;
	}

	/** The live entry, so a saved one flips to synced when the connection comes back. */
	const sentEntry = $derived(
		sent ? (outbox.mine.find((e) => e.clientId === sent!.clientId) ?? sent) : null
	);
</script>

<svelte:head><title>Submit an observation · DaGrouper</title></svelte:head>

<div class="mx-auto max-w-3xl">
	<p class="text-sm text-muted">Contribute</p>
	<h1 class="font-display text-3xl sm:text-4xl">Submit an observation</h1>
	<p class="mt-2 text-text">
		Pick what you are recording and fill in the form. It checks your values before anything is sent,
		then tags them with your classroom and sends them on to SAM.
	</p>

	{#if !outbox.online && data.targets.length}
		<p class="mt-6 rounded-md border border-orange/50 px-4 py-3 text-sm text-ink" role="status">
			You are offline. You can keep recording: observations are saved on this device and sync when
			you are back online.
		</p>
	{/if}

	{#if blocked && data.targets.length}
		<div class="card mt-8">
			<h2 class="text-xl font-semibold">Your guardian's consent is needed</h2>
			<p class="mt-2 text-sm">{blocked}</p>
			<p class="mt-1 text-sm text-muted">
				You can still explore your classroom's data. Ask your facilitator for a paper consent sheet,
				or open Settings to see where consent stands and send your guardian a link.
			</p>
			<a class="btn btn-ghost mt-4" href={resolve('/(app)/settings')}>Open settings</a>
		</div>
	{:else if !data.targets.length}
		<div class="card mt-8">
			<h2 class="text-xl font-semibold">You can't record observations yet</h2>
			<p class="mt-2 text-sm text-muted">
				Community scientists, facilitators and institution heads record observations in their
				classrooms' labs. Researchers read the data only.
			</p>
			<a class="btn btn-ghost mt-4" href={resolve('/(app)/dashboard')}>Your dashboard</a>
		</div>
	{:else if sentEntry && template && target}
		<ObserveSent
			entry={sentEntry}
			{template}
			{target}
			parameters={paramsById}
			onanother={another}
		/>
	{:else}
		{#if locked && device && target}
			<p class="mt-6 flex flex-wrap items-center gap-2 text-sm text-muted">
				<span class="inline-flex" style:color={deviceTypeColor(data.deviceTypes, device.type)}
					><Icon name={deviceTypeIcon(data.deviceTypes, device.type)} class="size-5" /></span
				>
				Entering a measurement for <span class="text-ink">{device.name}</span>
				<span class="font-mono text-xs">{device.identifier}</span> · {target.lab.name}
			</p>
		{/if}
		<div class={['card', locked ? 'mt-3' : 'mt-8']}>
			<Wizard
				{steps}
				bind:current={step}
				{check}
				onsubmit={submit}
				submitLabel="Send observation"
				busy={sending}
				busyLabel="Sending…"
				label="Submit an observation"
			>
				{#if stepId === 'what'}
					<ObserveWhat
						targets={data.targets}
						{target}
						bind:labId
						{templateId}
						{groups}
						deviceTypes={data.deviceTypes}
						tried={triedWhat}
						onlab={() => (deviceId = '')}
						onpick={pick}
					/>
				{:else if !template || !target}
					<p class="text-sm text-muted">Pick what you are recording first.</p>
				{:else if stepId === 'device'}
					<ObserveDevice
						{target}
						{fitting}
						{device}
						bind:deviceId
						deviceTypes={data.deviceTypes}
						error={deviceError}
						tried={triedDevice}
					/>
				{:else if stepId === 'values'}
					<ObserveValues
						{template}
						{target}
						parameters={paramsById}
						bind:entries
						bind:touched
						bind:observedAt
						bind:location
						bind:gps
						errorOf={shownError}
						{timeError}
						{serverErrors}
						{tried}
					/>
				{:else}
					<ObserveReview
						{template}
						{target}
						{device}
						values={checked.values}
						parameters={paramsById}
						{observedAt}
						location={gps === 'fix' ? location : null}
						error={formError}
					/>
				{/if}
			</Wizard>
		</div>
	{/if}
	{#if data.targets.length}
		<SyncStatus templates={data.templates} targets={data.targets} />
	{/if}
</div>
