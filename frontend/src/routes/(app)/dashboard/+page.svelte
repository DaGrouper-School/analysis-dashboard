<script lang="ts">
	import { tick } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { domainTabs } from '#lib/domains.ts';
	import { formatPoint } from '#lib/geo.ts';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import ReadingChart from '#lib/components/ReadingChart.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import SyncModal from '#lib/components/SyncModal.svelte';
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import { formatValue, keySummary, keyValues } from '#lib/key-values.ts';
	import { reveal } from '#lib/reveal.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import {
		CLASSROOM_ROLE_LABELS,
		deviceTypeColor,
		deviceTypeIcon,
		deviceTypeLabel
	} from '#lib/labels.ts';
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
	// AP-16: a manual reading for this sensor, on the guided form.
	const observeHref = $derived(
		resolve('/(app)/observe') +
			(data.device ? `?lab=${data.device.labId}&device=${data.device.id}` : '')
	);
	const latest = $derived(data.readings[0] ?? null);
	const tiles = $derived(device && latest ? keyValues(device.type, latest.values, paramsById) : []);

	/** Shows a classroom and, optionally, one of its sensors. The pick lives in the address. */
	async function choose(classroomId: string, deviceId?: string) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		await goto(resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId), {
			replace: true,
			reset: false
		});
		// Then show the picked sensor's values, wherever on the page it was picked from.
		await tick();
		reveal(conditions);
	}

	let conditions = $state<HTMLElement>();

	let syncing = $state(false);

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
				<!-- Tinted with the sensor's own color, the same as its dot on the globe. -->
				<div
					bind:this={conditions}
					class="accent-panel card flex-1 scroll-mt-24 text-ink"
					style:--accent={deviceTypeColor(data.deviceTypes, device.type)}
				>
					<div class="flex flex-wrap items-start justify-between gap-4">
						<div>
							<p class="text-sm text-muted">Current conditions</p>
							<h2 class="flex items-center gap-2 text-xl font-semibold">
								<Icon
									name={deviceTypeIcon(data.deviceTypes, device.type)}
									class="size-6 text-(--accent)"
								/>
								{device.name}
							</h2>
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
					<!-- As many columns as values, up to four: a lone value fills the row, no empty cells. -->
					<dl
						class={[
							'accent-tiles mt-8 grid gap-px border',
							tiles.length > 1 && 'grid-cols-2 [&>:last-child:nth-child(odd)]:col-span-2',
							tiles.length === 3 && 'sm:grid-cols-3 sm:[&>:last-child:nth-child(odd)]:col-span-1',
							tiles.length > 3 && 'sm:grid-cols-4'
						]}
					>
						<!-- Keyed by position, so switching sensors rolls the numbers in place. -->
						{#each tiles as item, i (i)}
							<div class="accent-tile p-4">
								<dt class="text-xs text-muted">{item.label}</dt>
								<dd class="mt-2 text-3xl text-ink">
									<RollingNumber value={formatValue(item.value, item.unit)} />
								</dd>
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
							aria-haspopup="dialog"
							onclick={() => (syncing = true)}
						>
							<Icon name="sync" class="size-4" /> Sync
						</button>
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a class="btn btn-ghost" href={observeHref}
							><Icon name="plus" class="size-4" /> Enter a reading by hand</a
						>
					{/if}
					<button type="button" class="btn btn-ghost" onclick={showReadings}>
						<Icon name="readings" class="size-4" /> Show raw sensor readings
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
						tint={device ? deviceTypeColor(data.deviceTypes, device.type) : undefined}
					/>
				{/key}
			</div>
		</a>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	</section>

	{#if device && data.canRecord && data.labLocation}
		<!-- Keyed by sensor, so a new pick never syncs to the sensor the user moved away from. -->
		{#key device.id}
			<SyncModal
				bind:open={syncing}
				devices={[device]}
				deviceTypes={data.deviceTypes}
				classroomName={data.classroom.name}
				labLocation={data.labLocation}
				{lastGps}
				parameters={data.parameters}
				onclose={() => invalidateAll()}
			/>
		{/key}
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
					color={deviceTypeColor(data.deviceTypes, device.type)}
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
									><span style:color={deviceTypeColor(data.deviceTypes, other.type)}
										><Icon
											name={deviceTypeIcon(data.deviceTypes, other.type)}
											class="size-3.5"
										/></span
									>
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
						<Icon name={domain.icon} fallback="tag" class="size-7 text-ink" />
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
