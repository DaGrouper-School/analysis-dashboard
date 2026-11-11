<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import { domainsOf } from '#lib/domains.ts';
	import CalculationPanel from '#lib/components/CalculationPanel.svelte';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import ClassroomAbout from '#lib/components/ClassroomAbout.svelte';
	import ClassroomCompleteness from '#lib/components/ClassroomCompleteness.svelte';
	import ClassroomOverview, { type ClassroomTab } from '#lib/components/ClassroomOverview.svelte';
	import ClassroomLabs from '#lib/components/ClassroomLabs.svelte';
	import ClassroomPeople from '#lib/components/ClassroomPeople.svelte';
	import ClassroomSettings from '#lib/components/ClassroomSettings.svelte';
	import DomainBrowser from '#lib/components/DomainBrowser.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import RegisterSensorModal from '#lib/components/RegisterSensorModal.svelte';
	import Select from '#lib/components/Select.svelte';
	import SyncModal from '#lib/components/SyncModal.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { formatValue, keySummary, keyValues } from '#lib/key-values.ts';
	import { deviceSummary, pickDevice, readingTiles } from '#lib/station-now.ts';
	import {
		CLASSROOM_ROLE_LABELS,
		withArticle,
		deviceTypeColor,
		deviceTypeIcon,
		deviceTypeLabel,
		deviceTypeOf,
		isManualType
	} from '#lib/labels.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import { valueHref } from '#lib/site.ts';
	import type { PageData } from './$types';

	/**
	 * The dashboard, in two views of the same data (AP-45). The overview is the classroom at a
	 * glance: its header, devices, what's happening now, the map and the institution. The details
	 * page holds the classroom's sections as tabs (domains, alerts, measurements and the rest),
	 * with the classroom and the device picked from dropdowns.
	 */
	let { data, view }: { data: PageData; view: 'overview' | 'details' } = $props();
	const details = $derived(view === 'details');

	// Skip a title, so "Dr. Marcus L." is greeted as Marcus.
	const firstName = $derived(
		data.user.fullName.split(' ').find((part) => !/^(dr|prof|mr|mrs|ms)\.?$/i.test(part)) ??
			data.user.fullName
	);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	// Another classroom opened in the app shows its public data only, with no member sections.
	const visiting = $derived(data.visiting);
	const station = $derived(visiting ?? data.station);
	const classroom = $derived(visiting ? null : data.classroom);
	const device = $derived(data.device);
	// A manual measurement has no sensor to sync: it is only ever entered by hand.
	const manual = $derived(!!device && isManualType(data.deviceTypes, device.type));

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const today = new Intl.DateTimeFormat(undefined, {
		weekday: 'long',
		day: 'numeric',
		month: 'long'
	});

	/** The overview on a classroom (and device). */
	const dashboardHref = (classroomId: string, deviceId?: string) =>
		resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId);
	/** The details page on a classroom (and device), open on a section. */
	const detailsHref = (classroomId: string, deviceId?: string, tab: string = active ?? 'domains') =>
		resolve('/(app)/dashboard/details') + sensorQuery(classroomId, deviceId) + `&tab=${tab}`;
	/** Moves to another classroom or device, staying on this view. */
	function choose(classroomId: string, deviceId?: string, tab = active) {
		const href = details
			? detailsHref(classroomId, deviceId, tab)
			: dashboardHref(classroomId, deviceId);
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		return goto(href, { replace: true, reset: false });
	}

	// ---- Devices: the one picked sets the tiles, the cards, the domains and the colour ----
	// A member's own classroom: the picked device's raw readings, so a sync shows at once.
	const domainIcons = $derived(new Map(data.domains.map((d) => [d.id, d.icon])));
	/** The device's tile or card values, as set for its type in the Registry. */
	const registryValues = (which: 'tileParameterIds' | 'cardParameterIds') =>
		device && data.readings[0]
			? keyValues(
					deviceTypeOf(data.deviceTypes, device.type)?.[which],
					data.readings[0].values,
					paramsById,
					{ domainIcons, fallback: which === 'tileParameterIds' }
				)
			: [];
	const own = $derived.by(() => {
		if (visiting || !device || !data.readings.length) return undefined;
		const icon = deviceTypeIcon(data.deviceTypes, device.type);
		return {
			tiles: readingTiles(data.readings, registryValues('tileParameterIds'), icon),
			summary: deviceSummary(data.readings, registryValues('cardParameterIds')),
			color: deviceTypeColor(data.deviceTypes, device.type)
		};
	});
	// Another classroom: its released devices, picked on the page as on its public page.
	let visitingDeviceId = $state<string>();
	const visitingDevice = $derived(
		visiting
			? pickDevice(visiting.devices, visitingDeviceId ?? page.url.searchParams.get('device'))
			: null
	);
	const shownDevice = $derived(visiting ? visitingDevice : own);
	const shownDeviceId = $derived(visiting ? visitingDevice?.id : device?.id);
	// The domains of the device shown.
	const domains = $derived(domainsOf(station?.domains ?? [], shownDeviceId));
	const pickerDevices = $derived(
		visiting
			? visiting.devices
			: classroom
				? data.devices.map((d) => ({
						id: d.id,
						name: d.name,
						color: deviceTypeColor(data.deviceTypes, d.type),
						icon: deviceTypeIcon(data.deviceTypes, d.type)
					}))
				: []
	);
	function pickDeviceShown(id: string) {
		if (visiting) visitingDeviceId = id;
		else if (classroom) choose(classroom.id, id);
	}

	// ---- The section tabs (AP-45) ----------------------------------------------------------
	type Section =
		| 'alerts'
		| 'domains'
		| 'devices'
		| 'labs'
		| 'records'
		| 'complete'
		| 'about'
		| 'people'
		| 'settings';
	const role = $derived(data.classrooms.find((c) => c.id === pendingId)?.myRole ?? null);
	const permissions = $derived(data.detail?.permissions);
	// People for whoever manages members, invites or consent; Settings for whoever edits.
	const managesPeople = $derived(
		!!permissions &&
			(permissions.manageMembers || permissions.manageConsent || permissions.invite.length > 0)
	);
	const managesSettings = $derived(!!permissions && (permissions.edit || permissions.viewCode));
	let active = $state<Section>();
	const alertCount = $derived(data.alerts.length + data.waitingForConsent.length);
	const sections = $derived<TabItem<Section>[]>(
		visiting
			? [
					{ id: 'domains', label: 'Domains', icon: 'layers', count: domains.length },
					{ id: 'complete', label: 'Completeness', icon: 'check' },
					{ id: 'about', label: 'About', icon: 'info' }
				]
			: [
					// Domains first; Labs for the institution head only.
					{ id: 'domains', label: 'Domains', icon: 'layers', count: domains.length },
					{
						id: 'alerts',
						label: 'Alerts',
						icon: 'alert',
						...(alertCount
							? { badge: { text: String(alertCount), title: `${alertCount} need you` } }
							: { count: 0 })
					},
					{ id: 'devices', label: 'Measurements', icon: 'station', count: data.devices.length },
					...(role === 'institution_head'
						? [{ id: 'labs' as const, label: 'Labs', icon: 'flask', count: data.labs.length }]
						: []),
					{ id: 'records', label: 'Records', icon: 'clipboard', count: data.records.length },
					{ id: 'complete', label: 'Completeness', icon: 'check' },
					{ id: 'about', label: 'About', icon: 'info' },
					...(managesPeople
						? [
								{
									id: 'people' as const,
									label: 'People',
									icon: 'users',
									count: data.detail?.members.length ?? 0
								}
							]
						: []),
					...(managesSettings
						? [{ id: 'settings' as const, label: 'Settings', icon: 'sliders' }]
						: [])
				]
	);
	// ---- Adding a measurement (a device) to one of the classroom's labs ----
	// Heads and facilitators add devices, as on a lab page.
	const canAddDevice = $derived(!!permissions?.invite.length && data.labs.length > 0);
	let adding = $state(false);
	let addLabId = $state('');
	const addLab = $derived(data.labs.find((l) => l.id === addLabId) ?? data.labs[0]);
	const labOptions = $derived(data.labs.map((l) => ({ value: l.id, label: l.name })));

	/** Opens a section: on the details page, its tab; from the overview, the details page on it. */
	function openSection(id: Section) {
		if (details) active = id;
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		else goto(detailsHref(station!.classroom.id, shownDeviceId, id));
	}
	const deviceOptions = $derived(
		pickerDevices.map((d) => ({ value: d.id, label: d.name, icon: d.icon, color: d.color }))
	);

	// ---- The shared overview ---------------------------------------------------------------
	const mine = $derived(new Set(data.classrooms.map((c) => c.id)));
	const markerLabel = (p: { id: string; name: string }) =>
		`${p.name} · ${mine.has(p.id) ? 'yours' : 'public data only'}`;
	const tabs = $derived<ClassroomTab[]>([
		...data.classrooms.map((c) => ({ id: c.id, label: c.name, href: dashboardHref(c.id) })),
		...(visiting
			? [
					{
						id: visiting.classroom.id,
						label: `${visiting.classroom.name} · visiting`,
						title: visiting.classroom.name,
						href: dashboardHref(visiting.classroom.id)
					}
				]
			: [])
	]);
	// The details page is for the user's own classrooms only.
	const classroomOptions = $derived(data.classrooms.map((c) => ({ value: c.id, label: c.name })));
	// The classroom being opened: the overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		((navigating.to?.route.id === '/(app)/dashboard' ||
			navigating.to?.route.id === '/(app)/dashboard/details') &&
			navigating.to.url.searchParams.get('classroom')) ||
			station?.classroom.id ||
			''
	);

	// ---- What's happening now: the picked device's values, as it used to be (AP-45) --------
	// "How is this calculated?" (AP-07) behind each value's info icon.
	let calculating = $state(false);
	let calculationId = $state('');
	function explain(id: string) {
		calculationId = id;
		calculating = true;
	}

	// ---- Devices ---------------------------------------------------------------------------
	let syncing = $state(false);
	/** Picks the device, then opens its sync once its lab and readings have loaded. */
	async function sync(deviceId: string) {
		if (deviceId !== device?.id) await choose(data.classroom!.id, deviceId, 'devices');
		syncing = true;
	}
	function lastGps() {
		return data.readings.find((r) => r.locationSource === 'device')?.location ?? null;
	}
	// The sensor shown can be synced here: a recorder, a lab to fall back on, not entered by hand.
	const canSync = $derived(
		!!classroom && !!device && data.canRecord && !!data.labLocation && !manual
	);
	const conditionOf = (id: string) => data.classroomConditions.find((c) => c.device.id === id);
</script>

<svelte:head
	><title>{details ? 'Classroom details' : 'Dashboard'} · DaGrouper Analysis</title></svelte:head
>

{#snippet classroomSections(station: NonNullable<typeof data.station>)}
	<section class="mt-8" aria-label="Classroom sections">
		<VerticalTabs tabs={sections} bind:active label="Classroom sections">
			{#if active === 'alerts'}
				{@render panelHead(
					'Alerts',
					alertCount
						? `What needs you in ${station.classroom.name}, from each device's latest reading.`
						: `Nothing needs you in ${station.classroom.name} right now.`
				)}
				<ul class="mt-5 space-y-3">
					{#each data.alerts as alert (`${alert.device.id}|${alert.parameterId}`)}
						{@const label = paramsById.get(alert.parameterId)?.label ?? alert.parameterId}
						<li class="card flex flex-wrap items-center gap-4 p-5">
							<div class="min-w-0 flex-1">
								<p class="flex flex-wrap items-center gap-3 text-xs text-muted">
									<span
										class={[
											'chip',
											alert.level === 'warning'
												? 'border-danger/60 text-danger'
												: 'border-orange/60 text-orange'
										]}
										><Icon name="alert" class="size-3.5" />
										{alert.level === 'warning' ? 'Warning' : 'Caution'}</span
									>
									{dateTime.format(new Date(alert.observedAt))}
								</p>
								<p class="mt-2 font-medium text-ink">{alert.title} at {alert.device.name}</p>
								<p class="mt-1 text-sm">{alert.message}</p>
							</div>
							<!-- Opens the value's page on this device, with its alert levels. valueHref resolves. -->
							<!-- eslint-disable svelte/no-navigation-without-resolve -->
							<a
								class="btn btn-ghost"
								href={valueHref(station.classroom.id, alert.parameterId, alert.device.id)}
								>Open {label}</a
							>
							<!-- eslint-enable svelte/no-navigation-without-resolve -->
						</li>
					{/each}
					{#each data.waitingForConsent as member (member.userId)}
						<li class="card flex flex-wrap items-center gap-4 p-5">
							<div class="min-w-0 flex-1">
								<p class="text-xs text-muted">
									<span class="chip"><Icon name="user" class="size-3.5" /> Consent</span>
								</p>
								<p class="mt-2 font-medium text-ink">
									{member.fullName} is waiting for guardian consent
								</p>
								<p class="mt-1 text-sm">
									They can't send measurements until a parent or guardian consents{member.guardianName
										? `. The link went to ${member.guardianName.replace(/\.$/, '')}.`
										: '.'}
								</p>
							</div>
							<button type="button" class="btn btn-ghost" onclick={() => openSection('people')}
								>Record paper consent</button
							>
						</li>
					{/each}
				</ul>
			{:else if active === 'domains'}
				<DomainBrowser
					{domains}
					parameters={station.parameters}
					classroomId={station.classroom.id}
				/>
			{:else if active === 'devices'}
				<div class="flex flex-wrap items-end justify-between gap-3">
					<div>
						{@render panelHead(
							'Measurements',
							`The devices in ${station.classroom.name}'s labs, each with its latest reading.`
						)}
					</div>
					{#if canAddDevice}
						<div class="flex flex-wrap items-center gap-2">
							{#if data.labs.length > 1}
								<Select
									options={labOptions}
									value={addLab?.id}
									label="Lab to add it to"
									size="compact"
									onchange={(id) => (addLabId = id)}
								/>
							{/if}
							<button
								type="button"
								class="btn btn-primary"
								aria-haspopup="dialog"
								onclick={() => (adding = true)}
							>
								<Icon name="plus" class="size-4" /> Add measurement
							</button>
						</div>
					{/if}
				</div>
				<ul class="mt-5 space-y-2.5">
					{#each data.devices as option (option.id)}
						{@const item = conditionOf(option.id)}
						<li class="card flex flex-wrap items-center gap-x-5 gap-y-3 p-4 sm:px-5">
							<span
								class="grid size-11 shrink-0 place-items-center rounded-md border border-line bg-raised"
								style:color={deviceTypeColor(data.deviceTypes, option.type)}
							>
								<Icon name={deviceTypeIcon(data.deviceTypes, option.type)} class="size-5" />
							</span>
							<div class="min-w-0 flex-[2_1_14rem]">
								<p class="font-medium text-ink">{option.name}</p>
								<p class="text-xs text-muted">
									{deviceTypeLabel(data.deviceTypes, option.type)} · {option.labName}
								</p>
							</div>
							<div class="min-w-0 flex-[2_1_12rem] text-sm">
								<p class="text-ink">
									{item
										? keySummary(
												deviceTypeOf(data.deviceTypes, option.type)?.tileParameterIds,
												item.reading.values,
												paramsById
											)
										: 'No readings yet'}
								</p>
								{#if item}
									<p class="mt-0.5 flex items-center gap-1.5 text-xs">
										<span class="size-1.5 rounded-full bg-lime" aria-hidden="true"></span>
										Seen {dateTime.format(new Date(item.reading.observedAt))}
									</p>
								{/if}
							</div>
							<div class="flex flex-wrap gap-2">
								{#if data.canRecord && !isManualType(data.deviceTypes, option.type)}
									<button
										type="button"
										class="btn btn-ghost"
										aria-haspopup="dialog"
										onclick={() => sync(option.id)}><Icon name="sync" class="size-4" /> Sync</button
									>
								{:else if data.canRecord}
									<!-- eslint-disable svelte/no-navigation-without-resolve -->
									<a
										class="btn btn-ghost"
										href={`${resolve('/(app)/observe')}?lab=${option.labId}&device=${option.id}`}
										>Enter a reading</a
									>
									<!-- eslint-enable svelte/no-navigation-without-resolve -->
								{/if}
								<!-- eslint-disable svelte/no-navigation-without-resolve -->
								<a
									class="btn btn-ghost"
									href={resolve('/(app)/dashboard/readings') +
										sensorQuery(station.classroom.id, option.id)}>Raw data</a
								>
								<!-- eslint-enable svelte/no-navigation-without-resolve -->
							</div>
						</li>
					{:else}
						<li class="text-muted">
							No devices yet. Ask your facilitator to add one in one of the labs.
						</li>
					{/each}
				</ul>
			{:else if active === 'labs'}
				{@render panelHead(
					'Labs',
					`${station.classroom.name}'s labs, with their devices and site profile.`
				)}
				<div class="mt-5">
					<ClassroomLabs
						classroomId={station.classroom.id}
						labs={data.labs}
						devices={data.devices}
						canAdd={!!permissions?.invite.length}
					/>
				</div>
			{:else if active === 'records'}
				<div class="flex flex-wrap items-end justify-between gap-3">
					<div>
						{@render panelHead(
							'Records',
							'Readings entered by hand and field observations in this classroom, newest first.'
						)}
					</div>
					{#if data.canRecord}
						<a class="btn btn-accent" href={resolve('/(app)/observe')}
							><Icon name="plus" class="size-4" /> Record observation</a
						>
					{/if}
				</div>
				{#if data.records.length}
					<div class="card mt-5 overflow-x-auto px-5 py-1">
						<table class="w-full min-w-[36rem] text-sm">
							<thead>
								<tr class="border-b border-line text-left text-xs text-muted">
									<th class="py-2.5 pr-3 font-medium">When</th>
									<th class="py-2.5 pr-3 font-medium">What</th>
									<th class="py-2.5 pr-3 font-medium">Value</th>
									<th class="py-2.5 font-medium">From</th>
								</tr>
							</thead>
							<tbody class="divide-y divide-line">
								{#each data.records as row (row.id)}
									{@const param = paramsById.get(row.parameterId)}
									<tr>
										<td class="py-3 pr-3 whitespace-nowrap text-muted tabular-nums"
											>{dateTime.format(new Date(row.observedAt))}</td
										>
										<td class="py-3 pr-3 text-ink">{param?.label ?? row.parameterId}</td>
										<td class="py-3 pr-3 font-mono whitespace-nowrap text-ink"
											>{formatValue(row.value, param?.unit, param?.valueType)}</td
										>
										<td class="py-3">
											{row.sourceKind === 'field'
												? 'Field observation'
												: row.sourceName}{row.contributorId
												? ` · participant ${row.contributorId}`
												: ''}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{:else}
					<p class="mt-4 text-muted">Nothing has been recorded by hand yet.</p>
				{/if}
			{:else if active === 'complete'}
				<ClassroomCompleteness completeness={station.completeness} />
			{:else if active === 'about' && data.about}
				<ClassroomAbout about={data.about} />
			{:else if active === 'people' && data.detail}
				{@render panelHead(
					'People',
					`Who is in ${station.classroom.name}, who is invited, and guardian consent for members under 18.`
				)}
				<div class="mt-5">
					<ClassroomPeople detail={data.detail} consent={data.consent} userId={data.user.id} />
				</div>
			{:else if active === 'settings' && data.detail}
				{@render panelHead('Settings', `${station.classroom.name}'s name, institution and code.`)}
				<div class="mt-5">
					<ClassroomSettings detail={data.detail} institutionInfo={station.institution} />
				</div>
			{/if}
		</VerticalTabs>
	</section>
{/snippet}

{#snippet panelHead(title: string, sub: string)}
	<h2 class="font-display text-2xl">{title}</h2>
	<p class="mt-1 max-w-2xl text-sm text-muted">{sub}</p>
{/snippet}

{#if data.invitations.length}
	<div class="mb-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

{#if !station}
	<p class="text-sm font-medium text-muted">Hello, {firstName}</p>
	<div class="card mt-6">
		<p class="text-sm text-muted">Current conditions</p>
		<h2 class="mt-1 text-xl font-semibold">You're not in a classroom yet</h2>
		<p class="mt-2 text-sm text-muted">
			Conditions come only from devices in your classrooms' labs. Join a classroom with an invite
			link to see its measurements here.
		</p>
		<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms/new')}>Start a classroom</a>
	</div>
{:else if details}
	<Breadcrumbs
		items={[
			{ label: 'Dashboard', href: dashboardHref(station.classroom.id, shownDeviceId) },
			{ label: 'Classroom details' }
		]}
	/>
	<header class={['mt-4', pendingId !== station.classroom.id && 'is-loading']}>
		<h1 class="font-display text-4xl">Classroom details</h1>
		<div class="mt-5 grid gap-3 sm:grid-cols-2 lg:w-2/3">
			<Select
				options={classroomOptions}
				value={pendingId}
				label="Classroom"
				onchange={(id) => choose(id, undefined, active)}
			/>
			{#if deviceOptions.length}
				<Select
					options={deviceOptions}
					value={shownDeviceId}
					label="Device"
					onchange={pickDeviceShown}
				/>
			{/if}
		</div>
	</header>
	{@render classroomSections(station)}
{:else}
	<ClassroomOverview
		{station}
		{tabs}
		tabsLabel="Your classrooms"
		{pendingId}
		{markerLabel}
		now={shownDevice?.tiles}
		summary={shownDevice?.summary}
		accent={shownDevice?.color}
		released={!own}
		devices={pickerDevices}
		deviceId={shownDeviceId}
		onpickdevice={pickDeviceShown}
		onexplain={explain}
		onpick={(id) => choose(id, undefined, active)}
	>
		{#snippet eyebrow()}
			<p class="text-sm font-medium text-muted">
				Hello, {firstName} · {today.format(new Date())}
			</p>
		{/snippet}
		{#snippet badges()}
			{#if role}
				<span class="chip"
					><Icon name="user" class="size-3" /> You're {withArticle(
						CLASSROOM_ROLE_LABELS[role].toLowerCase()
					)}</span
				>
			{:else if visiting}
				<span class="chip">Public data only</span>
			{/if}
		{/snippet}
		{#snippet nowActions()}
			{#if canSync && device}
				{@const id = device.id}
				<button
					type="button"
					class="btn btn-primary w-full rounded-full"
					aria-haspopup="dialog"
					onclick={() => sync(id)}><Icon name="sync" class="size-4" /> Sync this device</button
				>
			{/if}
			<!-- A classroom's details are for its members; a visitor sees its overview only. -->
			{#if classroom}
				<button
					type="button"
					class="btn btn-ghost flex-1 rounded-full"
					onclick={() => openSection('domains')}>View domains</button
				>
			{/if}
		{/snippet}
		{#snippet institutionActions()}
			{#if classroom && role === 'institution_head'}
				<button
					type="button"
					class="btn btn-ghost flex-1 rounded-full"
					onclick={() => openSection('settings')}>Edit overview</button
				>
			{/if}
		{/snippet}
	</ClassroomOverview>

	{#if canSync && classroom && device && data.labLocation}
		<!-- Keyed by sensor, so a new pick never syncs to the sensor the user moved away from. -->
		{#key device.id}
			<SyncModal
				bind:open={syncing}
				devices={[device]}
				deviceTypes={data.deviceTypes}
				classroomName={classroom.name}
				labLocation={data.labLocation}
				{lastGps}
				parameters={data.parameters}
				onsynced={() => invalidateAll()}
				onclose={() => invalidateAll()}
			/>
		{/key}
	{/if}
{/if}

{#if canAddDevice && addLab}
	<RegisterSensorModal
		bind:open={adding}
		labId={addLab.id}
		labName={addLab.name}
		deviceTypes={data.deviceTypes}
		domains={data.domains}
		parameters={data.parameters}
		onclose={() => invalidateAll()}
	/>
{/if}

<CalculationPanel
	bind:open={calculating}
	parameterId={calculationId}
	onpick={(id) => (calculationId = id)}
/>
