<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { domainTabs } from '#lib/domains.ts';
	import { formatPoint } from '#lib/geo.ts';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import ReadingChart from '#lib/components/ReadingChart.svelte';
	import RecordData from '#lib/components/RecordData.svelte';
	import { formatValue, keySummary, keyValues } from '#lib/key-values.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import { CLASSROOM_ROLE_LABELS, deviceTypeColor, deviceTypeLabel } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Skip a title, so "Dr. Marcus L." is greeted as Marcus.
	const firstName = $derived(
		data.user.fullName.split(' ').find((part) => !/^(dr|prof|mr|mrs|ms)\.?$/i.test(part)) ??
			data.user.fullName
	);

	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	// The domains the picked sensor sends values for open the domain view (AP-10).
	const sensorTabs = $derived(domainTabs(data.readings, data.parameters, data.domains));
	// Only the domains this sensor has values for, in the registry's order.
	const domainCards = $derived(
		data.classroom && data.device
			? sensorTabs.map((tab) => ({
					...tab,
					// The path is resolved; only the query is added to it.
					href:
						resolve('/(app)/dashboard/domains') +
						sensorQuery(data.classroom!.id, data.device!.id) +
						`&domain=${tab.id}`
				}))
			: []
	);
	// The map opens on the same classroom and sensor.
	const mapHref = $derived(
		resolve('/(app)/map') + (data.classroom ? sensorQuery(data.classroom.id, data.device?.id) : '')
	);

	const device = $derived(data.device);
	const latest = $derived(data.readings[0] ?? null);
	const tiles = $derived(device && latest ? keyValues(device.type, latest.values, paramsById) : []);

	/** Shows a classroom and, optionally, one of its sensors. The pick lives in the address. */
	function choose(classroomId: string, deviceId?: string) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId), {
			replace: true,
			reset: false
		});
	}

	let recording = $state(false);
	// A new pick closes the recorder, so it never records to a sensor the user moved away from.
	const deviceId = $derived(device?.id);
	$effect(() => {
		void deviceId;
		recording = false;
	});

	function lastGps() {
		return data.readings.find((r) => r.locationSource === 'device')?.location ?? null;
	}

	function showReadings() {
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(resolve('/(app)/dashboard/readings') + sensorQuery(data.classroom!.id, device?.id));
	}

	// Chart the sensor's key values that are numbers; the first one is shown first.
	const chartOptions = $derived(
		device && latest
			? keyValues(device.type, latest.values, paramsById, 8).filter(
					(v) => typeof v.value === 'number'
				)
			: []
	);
	let chartParam = $state('');
	const charted = $derived(chartOptions.find((o) => o.id === chartParam) ?? chartOptions[0]);

	const markers = $derived<GlobeMarker[]>(
		data.classroomConditions.map((c) => ({
			id: c.device.id,
			lat: c.reading.location.lat,
			lng: c.reading.location.lng,
			color: deviceTypeColor(data.deviceTypes, c.device.type),
			hollow: c.reading.locationSource === 'lab',
			label: c.device.name
		}))
	);

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<svelte:head><title>Dashboard · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<p class="text-sm text-muted">Welcome back</p>
		<h1 class="font-display text-3xl sm:text-4xl">Hello, {firstName}</h1>
	</div>
	{#if data.classroom}
		<div
			class="flex w-full flex-wrap items-end gap-3 sm:w-auto"
			role="group"
			aria-label="Show data for"
		>
			<div class="w-full sm:w-72">
				<label class="label" for="pick-classroom">Classroom</label>
				<select
					id="pick-classroom"
					class="input"
					value={data.classroom.id}
					onchange={(e) => choose(e.currentTarget.value)}
				>
					{#each data.classrooms as classroom (classroom.id)}
						<option value={classroom.id}>{classroom.name}</option>
					{/each}
				</select>
			</div>
			<div class="w-full sm:w-72">
				<label class="label" for="pick-device">Sensor</label>
				<select
					id="pick-device"
					class="input"
					value={device?.id ?? ''}
					disabled={!data.devices.length}
					onchange={(e) => choose(data.classroom!.id, e.currentTarget.value)}
				>
					{#each data.devices as option (option.id)}
						<option value={option.id}>{option.name}</option>
					{:else}
						<option value="">No sensors yet</option>
					{/each}
				</select>
			</div>
		</div>
	{/if}
</div>

{#if data.invitations.length}
	<div class="mt-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

{#if !data.classroom}
	<div class="card mt-8">
		<p class="text-sm text-muted">Current conditions</p>
		<h2 class="mt-1 text-xl font-semibold">You're not in a classroom yet</h2>
		<p class="mt-2 text-sm text-muted">
			Conditions come only from devices in your classrooms' labs. Join a classroom with an invite
			link to see its sensors here.
		</p>
		<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
	</div>
{:else}
	<p class="mt-3 text-sm text-muted">
		{data.classroom.institutionName}
		{#if data.classroom.myRole}· You're {CLASSROOM_ROLE_LABELS[
				data.classroom.myRole
			].toLowerCase()}{/if}
	</p>

	<section class="mt-6 grid gap-4 lg:grid-cols-3">
		{#if device && latest}
			<div class="flex flex-col gap-3 lg:col-span-2">
				<div class="card flex-1 border-teal/25 bg-forest text-ink">
					<div class="flex flex-wrap items-start justify-between gap-4">
						<div>
							<p class="text-sm text-muted">Current conditions</p>
							<h2 class="text-xl font-semibold">{device.name}</h2>
							<p class="mt-1 text-sm text-muted">
								<a
									class="hover:text-ink hover:underline"
									href={resolve('/(app)/labs/[labId]', { labId: device.labId })}>{device.labName}</a
								>
								· {formatPoint(latest.location)}
								{#if latest.locationSource === 'lab'}
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
							Observed {dateTime.format(new Date(latest.observedAt))}
						</p>
					</div>
					<dl
						class={[
							'mt-8 grid grid-cols-2 gap-px border border-line bg-line',
							tiles.length > 2 && 'sm:grid-cols-4'
						]}
					>
						{#each tiles as item (item.id)}
							<div class="bg-forest p-4">
								<dt class="text-xs text-muted">{item.label}</dt>
								<dd class="mt-2 text-3xl text-ink">{formatValue(item.value, item.unit)}</dd>
							</div>
						{/each}
					</dl>
					<div class="mt-6">
						<h3 class="text-sm font-medium">Active alerts</h3>
						{#if data.alerts.length}
							<ul class="mt-2 space-y-2">
								{#each data.alerts as alert (alert.parameterId)}
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
							<p class="mt-1 text-sm text-muted">No active alerts for this sensor.</p>
						{/if}
					</div>
				</div>
				<div class="flex flex-wrap gap-2">
					{#if data.canRecord}
						<button
							type="button"
							class="btn btn-primary"
							aria-expanded={recording}
							aria-controls="record-panel"
							onclick={() => (recording = !recording)}
						>
							{recording ? 'Close recorder' : 'Record data'}
						</button>
					{/if}
					<button type="button" class="btn btn-ghost" onclick={showReadings}>
						Show raw sensor readings
					</button>
				</div>
			</div>
		{:else}
			<div class="card lg:col-span-2">
				<p class="text-sm text-muted">Current conditions</p>
				<h2 class="mt-1 text-xl font-semibold">
					{device ? `No readings from ${device.name} yet` : 'No sensors in this classroom yet'}
				</h2>
				<p class="mt-2 text-sm text-muted">
					{device
						? 'Readings show here once the device is connected and sends data.'
						: 'Ask your facilitator to register a device in one of its labs.'}
				</p>
			</div>
		{/if}

		<!-- The path is resolved; only the query is added to it. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a
			href={mapHref}
			class="card group relative flex min-h-80 flex-col overflow-hidden p-0 transition hover:border-line-strong"
		>
			<div class="flex items-baseline justify-between gap-3 p-5 pb-0">
				<h2 class="font-medium">Sensors in this classroom</h2>
				<span class="text-sm text-muted group-hover:text-ink">Open the map →</span>
			</div>
			<div class="pointer-events-none relative min-h-64 flex-1">
				{#key data.classroom.id}
					<Globe
						{markers}
						selectedId={device?.id ?? ''}
						ariaLabel="Globe of this classroom's sensors"
						interactive={false}
					/>
				{/key}
			</div>
		</a>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	</section>

	{#if recording && device && data.canRecord && data.labLocation && data.classroom}
		<div id="record-panel" class="mt-4">
			<RecordData
				devices={[device]}
				classroomName={data.classroom.name}
				labLocation={data.labLocation}
				{lastGps}
				parameters={data.parameters}
				onfinish={() => invalidateAll()}
			/>
		</div>
	{/if}

	{#if device && charted}
		<section class="card mt-4">
			<div class="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">{charted.label} over time</h2>
					<p class="text-sm text-muted">
						{device.name} · last {data.readings.length} reading{data.readings.length === 1
							? ''
							: 's'}
					</p>
				</div>
				{#if chartOptions.length > 1}
					<div
						class="flex max-w-full overflow-x-auto rounded-full border border-line p-0.5 text-sm"
						role="group"
						aria-label="Value to chart"
					>
						{#each chartOptions as option (option.id)}
							<button
								type="button"
								class={[
									'shrink-0 rounded-full px-3 py-1 whitespace-nowrap transition',
									option.id === charted.id ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'
								]}
								aria-pressed={option.id === charted.id}
								onclick={() => (chartParam = option.id)}>{option.label}</button
							>
						{/each}
					</div>
				{/if}
			</div>
			<div class="mt-4">
				<ReadingChart
					readings={data.readings}
					parameterId={charted.id}
					label={charted.label}
					unit={charted.unit}
				/>
			</div>
		</section>
	{/if}

	{#if data.devices.length > 1}
		<section class="mt-8">
			<h2 class="text-lg font-semibold">Other sensors in {data.classroom.name}</h2>
			<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each data.devices.filter((d) => d.id !== device?.id) as other (other.id)}
					{@const item = data.classroomConditions.find((c) => c.device.id === other.id)}
					<li>
						<button
							type="button"
							class="card block h-full w-full p-4 text-left transition hover:border-line-strong"
							onclick={() => choose(data.classroom!.id, other.id)}
						>
							<div class="flex items-start justify-between gap-2">
								<span class="font-medium">{other.name}</span>
								<span class="chip shrink-0"
									><span
										class="size-2 rounded-full"
										style:background={deviceTypeColor(data.deviceTypes, other.type)}
									></span>
									{deviceTypeLabel(data.deviceTypes, other.type)}</span
								>
							</div>
							<p class="mt-1 text-xs text-muted">
								{other.labName}{#if item}
									· {dateTime.format(new Date(item.reading.observedAt))}{/if}
							</p>
							<p class="mt-2 text-sm">
								{item ? keySummary(other.type, item.reading.values, paramsById) : 'No readings yet'}
							</p>
						</button>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
{/if}

{#if device && domainCards.length}
	<section class="mt-8">
		<h2 class="text-lg font-semibold">Environmental domains</h2>
		<p class="text-sm text-muted">
			Open a domain for each of {device.name}'s values, with thresholds and trends.
		</p>
		<ul class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
			{#each domainCards as domain (domain.id)}
				<li>
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a class="card block h-full p-4 transition hover:border-line-strong" href={domain.href}>
						<span class="text-2xl" aria-hidden="true">{domain.icon}</span>
						<p class="mt-3 font-semibold">{domain.label}</p>
						<p class="text-xs text-muted">
							{domain.parameterIds.length} value{domain.parameterIds.length === 1 ? '' : 's'} →
						</p>
					</a>
				</li>
			{/each}
		</ul>
	</section>
{/if}
