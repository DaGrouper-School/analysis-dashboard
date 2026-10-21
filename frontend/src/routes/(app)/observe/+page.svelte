<script lang="ts">
	import { resolve } from '$app/paths';
	import type { GeoPoint, Observation, ObservationTemplate, Parameter } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { formatValue } from '#lib/key-values.ts';
	import { deviceTypeColor, deviceTypeLabel } from '#lib/labels.ts';
	import { rangeText, readEntries, unitsFor, type FieldEntry } from '#lib/observations.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

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

	/** Templates by domain, in the registry's domain order; ones with no domain go last. */
	const groups = $derived.by(() => {
		const byDomain = [
			...data.domains.map((d) => ({ id: d.id, label: d.label, icon: d.icon })),
			{ id: null, label: 'Other', icon: '📋' }
		].map((domain) => ({
			...domain,
			templates: data.templates.filter((t) =>
				domain.id === null ? !data.domains.some((d) => d.id === t.domain) : t.domain === domain.id
			)
		}));
		return byDomain.filter((g) => g.templates.length);
	});

	const devicesOfType = (t: ObservationTemplate) =>
		t.deviceType ? (target?.devices.filter((d) => d.type === t.deviceType).length ?? 0) : null;

	/** Now, as a `datetime-local` value in the browser's time zone. */
	function localNow() {
		const offset = new Date().getTimezoneOffset() * 60_000;
		return new Date(Date.now() - offset).toISOString().slice(0, 16);
	}

	let entries = $state<Record<string, FieldEntry>>({});
	let observedAt = $state(localNow());
	let credit = $state(false);
	let location = $state<GeoPoint | null>(null);
	let gps = $state<'idle' | 'locating' | 'fix' | 'none'>('idle');
	let touched = $state<Record<string, boolean>>({});
	let tried = $state(false);
	let sending = $state(false);
	let serverErrors = $state<Record<string, string>>({});
	let formError = $state('');
	let sent = $state<Observation | null>(null);

	/** A fresh form for the picked template: empty values, each in its own unit. */
	function reset(next: ObservationTemplate | null) {
		entries = Object.fromEntries(
			(next?.fields ?? []).map((f) => [
				f.parameterId,
				{ value: '', unit: paramsById.get(f.parameterId)?.unit ?? '' }
			])
		);
		touched = {};
		tried = false;
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
		template?.deviceType && !device
			? fitting.length
				? 'Choose the device you read this from.'
				: `No ${deviceTypeLabel(data.deviceTypes, template.deviceType).toLowerCase()} is registered to this lab.`
			: ''
	);
	const timeError = $derived(
		!observedAt
			? 'Enter when you observed it.'
			: new Date(observedAt).getTime() > Date.now() + 5 * 60_000
				? 'That time is in the future.'
				: ''
	);
	/** Shown once a field was left, or after a send was tried. The server's word wins. */
	const shownError = (id: string) =>
		serverErrors[id] ?? (tried || touched[id] ? checked.errors[id] : undefined);
	const problems = $derived(
		[
			...Object.entries(checked.errors).map(([id, message]) => ({ id: `field-${id}`, message })),
			deviceError && { id: 'observe-device', message: deviceError },
			timeError && { id: 'observe-time', message: timeError }
		].filter((p): p is { id: string; message: string } => !!p)
	);

	function locate() {
		if (!('geolocation' in navigator)) {
			gps = 'none';
			return;
		}
		gps = 'locating';
		navigator.geolocation.getCurrentPosition(
			(position) => {
				location = {
					lat: Math.round(position.coords.latitude * 1e5) / 1e5,
					lng: Math.round(position.coords.longitude * 1e5) / 1e5
				};
				gps = 'fix';
			},
			() => {
				location = null;
				gps = 'none';
			},
			{ enableHighAccuracy: true, timeout: 10_000 }
		);
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!template || !target) return;
		tried = true;
		serverErrors = {};
		formError = '';
		if (problems.length) {
			document.getElementById(problems[0].id)?.focus();
			return;
		}
		sending = true;
		try {
			sent = await services.labs.submitObservation(
				{
					templateId: template.id,
					labId: target.lab.id,
					deviceId: device?.id ?? null,
					observedAt: new Date(observedAt).toISOString(),
					values: checked.values,
					location
				},
				{ credit }
			);
		} catch (e) {
			if (e instanceof ServiceError && Object.keys(e.fields).length) serverErrors = e.fields;
			formError = errorMessage(e);
		} finally {
			sending = false;
		}
	}

	function another() {
		sent = null;
		observedAt = localNow();
		reset(template);
	}

	const sentDevice = $derived(target?.devices.find((d) => d.id === sent?.deviceId) ?? null);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const labelFor = (p: Parameter | undefined, id: string) => p?.label ?? id;
</script>

<svelte:head><title>Submit an observation · DaGrouper</title></svelte:head>

<div class="mx-auto max-w-3xl">
	<p class="text-sm text-muted">Contribute</p>
	<h1 class="font-display text-3xl sm:text-4xl">Submit an observation</h1>
	<p class="mt-2 text-text">
		Pick what you are recording and fill in the form. It checks your values before anything is sent,
		then tags them with your classroom and sends them on to SAM.
	</p>

	{#if !data.targets.length}
		<div class="card mt-8">
			<h2 class="text-xl font-semibold">You can't record observations yet</h2>
			<p class="mt-2 text-sm text-muted">
				Community scientists, facilitators and institution heads record observations in their
				classrooms' labs. Researchers read the data only.
			</p>
			<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
		</div>
	{:else if sent && template && target}
		<section class="card mt-8" aria-labelledby="sent-heading">
			<p class="text-sm text-lime" role="status">Sent</p>
			<h2 id="sent-heading" class="mt-1 text-xl font-semibold">{template.label} recorded</h2>
			<p class="mt-1 text-sm text-muted">
				{dateTime.format(new Date(sent.observedAt))} · {target.classroomName} · {target.lab.name}
				{#if sentDevice}· {sentDevice.name}{/if}
			</p>
			<dl class="mt-5 grid gap-px border border-line bg-line sm:grid-cols-2">
				{#each Object.entries(sent.values) as [id, value] (id)}
					<div class="bg-surface p-3">
						<dt class="text-xs text-muted">{labelFor(paramsById.get(id), id)}</dt>
						<dd class="mt-1 text-lg text-ink">{formatValue(value, paramsById.get(id)?.unit)}</dd>
					</div>
				{/each}
			</dl>
			<ul class="mt-4 space-y-1 text-sm">
				<li>
					Tagged with {target.classroomName}{sentDevice
						? `, as a reading from ${sentDevice.name}`
						: ', as a field observation'}.
				</li>
				<li>
					{formatPoint(sent.location)}
					{#if sent.locationSource === 'device'}
						<span class="text-muted">· your GPS</span>
					{:else}
						<span class="rounded-full bg-raised px-2 py-0.5 text-xs font-medium ring-1 ring-line"
							>No GPS: lab location, not from the device</span
						>
					{/if}
				</li>
				<li class="text-muted">
					{sent.creditedTo
						? `Credited to ${sent.creditedTo}`
						: 'Not credited to anyone: public outputs show it without a name.'}
				</li>
			</ul>
			<div class="mt-6 flex flex-wrap gap-2">
				<button type="button" class="btn btn-primary" onclick={another}>Record another</button>
				<a class="btn btn-ghost" href={resolve('/(app)/labs/[labId]', { labId: target.lab.id })}
					>Open {target.lab.name}</a
				>
			</div>
		</section>
	{:else}
		<form class="mt-8 space-y-8" novalidate onsubmit={submit}>
			{#if data.targets.length > 1}
				<div class="max-w-sm">
					<label class="label" for="observe-lab">Lab</label>
					<select
						id="observe-lab"
						class="input"
						bind:value={labId}
						onchange={() => (deviceId = '')}
					>
						{#each data.targets as t (t.lab.id)}
							<option value={t.lab.id}>{t.lab.name} · {t.classroomName}</option>
						{/each}
					</select>
				</div>
			{:else if target}
				<p class="text-sm text-muted">
					Recording in <span class="text-ink">{target.lab.name}</span> · {target.classroomName}
				</p>
			{/if}

			<fieldset>
				<legend class="text-lg font-semibold text-ink">1. What are you recording?</legend>
				<div class="mt-3 space-y-5">
					{#each groups as group (group.id)}
						<div>
							<p class="eyebrow"><span aria-hidden="true">{group.icon}</span> {group.label}</p>
							<div class="mt-2 grid gap-2 sm:grid-cols-2">
								{#each group.templates as t (t.id)}
									{@const count = devicesOfType(t)}
									<label
										class={[
											'flex cursor-pointer gap-3 rounded-md border p-4 transition',
											t.id === templateId
												? 'border-ink bg-ink/[0.05]'
												: 'border-line hover:border-line-strong'
										]}
									>
										<input
											type="radio"
											name="template"
											class="mt-1 border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
											value={t.id}
											checked={t.id === templateId}
											onchange={() => pick(t.id)}
										/>
										<span class="min-w-0">
											<span class="block font-medium text-ink">{t.label}</span>
											<span class="block text-sm text-muted">{t.description}</span>
											<span class="mt-1 block text-xs text-muted">
												{#if t.deviceType}
													<span
														class="mr-1 inline-block size-2 rounded-full"
														style:background={deviceTypeColor(data.deviceTypes, t.deviceType)}
														aria-hidden="true"
													></span>
													{deviceTypeLabel(data.deviceTypes, t.deviceType)} reading{count === 0
														? ' · none registered in this lab'
														: ''}
												{:else}
													Field observation, no device
												{/if}
											</span>
										</span>
									</label>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			</fieldset>

			{#if template && target}
				{#if template.deviceType}
					<div>
						<h2 class="text-lg font-semibold">2. Which device did you read?</h2>
						{#if fitting.length > 1}
							<div class="mt-3 max-w-sm">
								<label class="label" for="observe-device">Device</label>
								<select
									id="observe-device"
									class={['input', tried && deviceError && 'input-error']}
									bind:value={deviceId}
									aria-invalid={tried && !!deviceError}
									aria-describedby="observe-device-help"
								>
									<option value="" disabled>Choose a device</option>
									{#each fitting as d (d.id)}
										<option value={d.id}>{d.name} · {d.identifier}</option>
									{/each}
								</select>
							</div>
						{:else if device}
							<p id="observe-device" tabindex="-1" class="mt-3 text-ink">
								{device.name} <span class="text-muted">· {device.identifier}</span>
							</p>
						{:else}
							<p id="observe-device" tabindex="-1" class="field-error">{deviceError}</p>
						{/if}
						{#if tried && deviceError && fitting.length > 1}
							<p class="field-error">{deviceError}</p>
						{/if}
						<p id="observe-device-help" class="mt-2 text-sm text-muted">
							Only devices registered to {target.lab.name} are listed. A facilitator registers new ones
							on the lab page.
						</p>
					</div>
				{/if}

				<fieldset>
					<legend class="text-lg font-semibold text-ink">
						{template.deviceType ? '3' : '2'}. Your values
					</legend>
					<div class="mt-3 grid gap-5 sm:grid-cols-2">
						{#each template.fields as field (field.parameterId)}
							{@const param = paramsById.get(field.parameterId)}
							{@const error = shownError(field.parameterId)}
							{@const entry = entries[field.parameterId]}
							{#if param && entry}
								{@const units = unitsFor(param)}
								{@const range = rangeText(param)}
								<div>
									<label class="label" for="field-{param.id}">
										{param.label}
										{#if !field.required}<span class="font-normal text-muted">(optional)</span>{/if}
									</label>
									<div class="flex gap-2">
										{#if param.options}
											<select
												id="field-{param.id}"
												class={['input', error && 'input-error']}
												bind:value={entry.value}
												onblur={() => (touched[param.id] = true)}
												aria-invalid={!!error}
												aria-describedby="help-{param.id}"
											>
												<option value="">Choose</option>
												{#each param.options as option (option)}
													<option value={option}>{option}</option>
												{/each}
											</select>
										{:else}
											<input
												id="field-{param.id}"
												class={['input min-w-0', error && 'input-error']}
												inputmode={(param.valueType ?? 'number') === 'number' ? 'decimal' : 'text'}
												autocomplete="off"
												bind:value={entry.value}
												onblur={() => (touched[param.id] = true)}
												aria-invalid={!!error}
												aria-describedby="help-{param.id}"
											/>
										{/if}
										{#if units.length > 1}
											<select
												class="input w-24 shrink-0"
												aria-label="{param.label} unit"
												bind:value={entry.unit}
											>
												{#each units as unit (unit)}
													<option value={unit}>{unit}</option>
												{/each}
											</select>
										{:else if param.unit}
											<span class="self-center text-sm text-muted">{param.unit}</span>
										{/if}
									</div>
									<p id="help-{param.id}" class="mt-1.5 text-xs text-muted">
										{param.definition}{range ? ` Expected ${range}.` : ''}
									</p>
									{#if error}<p class="field-error">{error}</p>{/if}
								</div>
							{/if}
						{/each}
					</div>
				</fieldset>

				<div class="grid gap-5 sm:grid-cols-2">
					<div>
						<label class="label" for="observe-time">When</label>
						<input
							id="observe-time"
							type="datetime-local"
							class={['input', tried && timeError && 'input-error']}
							max={localNow()}
							bind:value={observedAt}
						/>
						{#if (tried && timeError) || serverErrors.observedAt}
							<p class="field-error">{serverErrors.observedAt ?? timeError}</p>
						{/if}
					</div>
					<div>
						<p class="label">Where</p>
						<div class="flex flex-wrap items-center gap-2">
							<button
								type="button"
								class="btn btn-ghost"
								onclick={locate}
								disabled={gps === 'locating'}
							>
								{gps === 'locating' ? 'Finding you…' : gps === 'fix' ? 'Update GPS' : 'Use my GPS'}
							</button>
							{#if gps === 'fix' && location}
								<span class="text-sm text-ink">{formatPoint(location)}</span>
							{/if}
						</div>
						<p class="mt-1.5 text-xs text-muted" aria-live="polite">
							{gps === 'fix'
								? 'Sent with the observation.'
								: `${gps === 'none' ? 'No GPS fix. ' : ''}Without GPS, ${target.lab.name}'s location is used and flagged as not from the device.`}
						</p>
					</div>
				</div>

				<label class="flex items-start gap-2 text-sm">
					<input
						id="observe-credit"
						type="checkbox"
						class="mt-0.5 rounded-sm border-line-strong bg-transparent text-teal focus:ring-teal"
						bind:checked={credit}
					/>
					<span>
						Credit this observation to me
						<span class="block text-xs text-muted"
							>Your name is shown on it to people in {target.classroomName} and in public outputs. Leave
							it off and it is stored without your name.</span
						>
					</span>
				</label>

				{#if tried && problems.length}
					<div class="rounded-md border border-danger/50 p-4" role="alert">
						<p class="text-sm font-medium text-ink">
							Fix {problems.length === 1 ? 'this' : `these ${problems.length}`} before sending:
						</p>
						<ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-danger">
							{#each problems as problem (problem.id)}
								<li><a class="underline" href="#{problem.id}">{problem.message}</a></li>
							{/each}
						</ul>
					</div>
				{/if}
				{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

				<button class="btn btn-primary" disabled={sending}>
					{sending ? 'Sending…' : 'Send observation'}
				</button>
			{/if}
		</form>
	{/if}
</div>
