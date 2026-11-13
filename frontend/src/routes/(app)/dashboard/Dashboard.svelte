<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import CalculationPanel from '#lib/features/measurements/CalculationPanel.svelte';
	import ClassroomOverview, { type ClassroomTab } from '#lib/views/ClassroomOverview.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PendingInvitations from '#lib/features/classroom/PendingInvitations.svelte';
	import AddDeviceButton from '#lib/features/devices/AddDeviceButton.svelte';
	import SyncModal from '#lib/features/devices/SyncModal.svelte';
	import { pickDevice } from '#lib/features/devices/station-now.ts';
	import { CLASSROOM_ROLE_LABELS, withArticle, isManualType } from '#lib/utils/labels.ts';
	import { givenName } from '#lib/utils/format.ts';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';
	import { classroomsHref, domainsHref } from '#lib/utils/site.ts';
	import ClassroomPageHeader from './ClassroomPageHeader.svelte';
	import ClassroomSections from './ClassroomSections.svelte';
	import NoClassroom from './NoClassroom.svelte';
	import { sectionTabs, type Section } from './sections.ts';
	import type { PageData } from './$types';

	/**
	 * The dashboard, in two views of the same data (AP-45). The overview is the classroom at a
	 * glance: its header, devices, what's happening now, the map and the institution. The
	 * Classrooms page holds the classroom's sections as tabs (measurements, labs, people and the
	 * rest), with the classroom picked from a dropdown. Domains have their own page, and alerts and
	 * messages are in the top bar on every page.
	 */
	let { data, view }: { data: PageData; view: 'overview' | 'details' } = $props();
	const details = $derived(view === 'details');

	const firstName = $derived(givenName(data.user.fullName));
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	// Another classroom opened in the app shows its public data only, with no member sections.
	const visiting = $derived(data.visiting);
	const station = $derived(visiting ?? data.station);
	const classroom = $derived(visiting ? null : data.classroom);
	const device = $derived(data.device);
	// A manual measurement has no sensor to sync: it is only ever entered by hand.
	const manual = $derived(!!device && isManualType(data.deviceTypes, device.type));

	const today = new Intl.DateTimeFormat(undefined, {
		weekday: 'long',
		day: 'numeric',
		month: 'long'
	});

	/** The overview on a classroom (and device). */
	const dashboardHref = (classroomId: string, deviceId?: string) =>
		resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId);
	/** The Classrooms page on a classroom (and device), open on a section. */
	const detailsHref = (classroomId: string, deviceId?: string, tab?: string) =>
		classroomsHref(classroomId, tab ?? active, deviceId);
	/** Moves to another classroom or device, staying on this view. */
	function choose(classroomId: string, deviceId?: string, tab = active) {
		const href = details
			? detailsHref(classroomId, deviceId, tab)
			: dashboardHref(classroomId, deviceId);
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		return goto(href, { replace: true, reset: false });
	}

	// ---- Devices: the one picked sets the tiles, the cards, the domains and the colour ----
	// The same devices as the classroom's public page, built by the same service from the
	// database: a member's own classroom with its readings still in review, so a sync shows at
	// once; another classroom with its released data only.
	let visitingDeviceId = $state<string>();
	const shownDevice = $derived(
		station
			? pickDevice(
					station.devices,
					visiting ? (visitingDeviceId ?? page.url.searchParams.get('device')) : device?.id
				)
			: null
	);
	const shownDeviceId = $derived(shownDevice?.id);
	const pickerDevices = $derived(station?.devices ?? []);
	function pickDeviceShown(id: string) {
		if (visiting) visitingDeviceId = id;
		else if (classroom) choose(classroom.id, id);
	}

	// ---- The section tabs (AP-45) ----------------------------------------------------------
	const role = $derived(data.classrooms.find((c) => c.id === pendingId)?.myRole ?? null);
	const permissions = $derived(data.detail?.permissions);
	// People for whoever manages members, invites or consent; Settings for whoever edits.
	const managesPeople = $derived(
		!!permissions &&
			(permissions.manageMembers || permissions.manageConsent || permissions.invite.length > 0)
	);
	const managesSettings = $derived(
		!!permissions && (permissions.edit || permissions.seeCode || permissions.viewSharing)
	);
	let active = $state<Section>();
	const sections = $derived(
		sectionTabs(
			{
				devices: data.devices.length,
				labs: data.labs.length,
				records: data.records.length,
				members: data.detail?.members.length ?? 0
			},
			{ head: role === 'institution_head', managesPeople, managesSettings }
		)
	);
	// ---- Adding a measurement (a device) to one of the classroom's labs ----
	// Whoever manages the classroom's devices adds them, as on a lab page.
	const canAddDevice = $derived(!!permissions?.manageDevices && data.labs.length > 0);

	/** Opens a section: on the Classrooms page, its tab; from the overview, that page on it. */
	function openSection(id: Section) {
		if (details) active = id;
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		else goto(detailsHref(station!.classroom.id, shownDeviceId, id));
	}

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
	// The Classrooms page is for the user's own classrooms only.
	const classroomOptions = $derived(data.classrooms.map((c) => ({ value: c.id, label: c.name })));
	// The classroom being opened: the overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		((navigating.to?.route.id === '/(app)/dashboard' ||
			navigating.to?.route.id === '/(app)/dashboard/classrooms') &&
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
	/** Recording by hand in the device's lab; a hand-entered measurement opens on its own form. */
	function recordHref(shown: { labId: string; id: string }) {
		const query = new URLSearchParams({ lab: shown.labId, ...(manual && { device: shown.id }) });
		return `${resolve('/(app)/observe')}?${query}`;
	}
	function lastGps() {
		return data.readings.find((r) => r.locationSource === 'device')?.location ?? null;
	}
	// The sensor shown can be synced here: a recorder, a lab to fall back on, not entered by hand.
	const canSync = $derived(
		!!classroom && !!device && data.canRecord && !!data.labLocation && !manual
	);
</script>

<svelte:head><title>{details ? 'Classrooms' : 'Dashboard'} · DaGrouper Analysis</title></svelte:head
>

<!-- Someone in no classroom yet sees their invitations here too; everyone has them in the top bar. -->
{#if !station && data.invitations.length}
	<div class="mb-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

{#if !station}
	<NoClassroom {firstName} />
{:else if details}
	<ClassroomPageHeader
		title="Classrooms"
		sub="Everything about {station.classroom
			.name}: its measurements, labs, records, people and settings."
		loading={pendingId !== station.classroom.id}
		{classroomOptions}
		classroomId={pendingId}
		onclassroom={(id) => choose(id, undefined, active)}
	/>
	<ClassroomSections
		{data}
		{station}
		{sections}
		bind:active
		parameters={paramsById}
		{canAddDevice}
		canAddLab={!!permissions?.manageDevices}
		onsync={sync}
	/>
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
		devices={pickerDevices}
		deviceId={shownDeviceId}
		onpickdevice={pickDeviceShown}
		onexplain={explain}
		onpick={(id) => choose(id, undefined, active)}
	>
		{#snippet headerActions()}
			<!-- The classroom's own page, with its sections; for its members only. -->
			{#if classroom}
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class="btn btn-ghost shrink-0 max-sm:px-2.5"
					href={detailsHref(classroom.id, shownDeviceId, 'devices')}
					aria-label="Classroom details"
					><Icon name="settings" class="size-4" /><span class="max-sm:sr-only"
						>Classroom details</span
					></a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{/if}
		{/snippet}
		{#snippet deviceActions()}
			<!-- Pairing, for whoever manages the classroom's devices. -->
			{#if classroom && canAddDevice}
				<AddDeviceButton
					labs={data.labs}
					deviceTypes={data.deviceTypes}
					domains={data.domains}
					parameters={data.parameters}
					label="Pair new device"
					iconOnPhone
				/>
			{/if}
		{/snippet}
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
			<!-- Syncing the device and recording by hand side by side, then its domains under them. -->
			{#if canSync && device}
				{@const id = device.id}
				<button
					type="button"
					class="btn btn-primary flex-[1_1_9rem] rounded-full px-3 whitespace-nowrap"
					aria-haspopup="dialog"
					onclick={() => sync(id)}><Icon name="sync" class="size-4" /> Sync device</button
				>
			{/if}
			{#if classroom && data.canRecord && device}
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class={[
						'btn flex-[1_1_9rem] rounded-full px-3 whitespace-nowrap',
						canSync ? 'btn-ghost' : 'btn-primary'
					]}
					href={recordHref(device)}><Icon name="plus" class="size-4" /> Record manually</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{/if}
			<!-- A classroom's details are for its members; a visitor sees its overview only. -->
			{#if classroom}
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class="btn btn-accent basis-full rounded-full"
					href={domainsHref(classroom.id, shownDeviceId)}
					><Icon name="layers" class="size-4" /> View domains</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{/if}
		{/snippet}
		{#snippet institutionActions()}
			{#if classroom && role === 'institution_head'}
				<button
					type="button"
					class="btn btn-ghost flex-1 rounded-full"
					onclick={() => openSection('settings')}
					><Icon name="pencil" class="size-4" /> Edit overview</button
				>
			{/if}
		{/snippet}
	</ClassroomOverview>
{/if}

<!-- On the dashboard and on the Classrooms page, where Measurements opens it. -->
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

<CalculationPanel
	bind:open={calculating}
	parameterId={calculationId}
	onpick={(id) => (calculationId = id)}
/>
