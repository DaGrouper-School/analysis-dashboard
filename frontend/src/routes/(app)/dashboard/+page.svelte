<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating } from '$app/state';
	import { domainOf } from '#lib/domains.ts';
	import ClassroomOverview, { type ClassroomTab } from '#lib/components/ClassroomOverview.svelte';
	import CompletenessCard from '#lib/components/CompletenessCard.svelte';
	import DomainBrowser from '#lib/components/DomainBrowser.svelte';
	import type { GlobeMarker } from '#lib/components/Globe.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import SyncModal from '#lib/components/SyncModal.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { formatValue, keySummary } from '#lib/key-values.ts';
	import {
		CLASSROOM_ROLE_LABELS,
		deviceTypeColor,
		deviceTypeIcon,
		deviceTypeLabel,
		isManualType
	} from '#lib/labels.ts';
	import { domainQuery, sensorQuery } from '#lib/sensor-selection.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

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

	/** The dashboard on a classroom (and sensor), keeping the open section tab. */
	const dashboardHref = (classroomId: string, deviceId?: string, tab = active) =>
		resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId) + `&tab=${tab ?? 'alerts'}`;
	function choose(classroomId: string, deviceId?: string, tab = active) {
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		return goto(dashboardHref(classroomId, deviceId, tab), { replace: true, reset: false });
	}

	// ---- The section tabs (AP-45) ----------------------------------------------------------
	type Section = 'alerts' | 'domains' | 'devices' | 'labs' | 'records' | 'complete';
	let active = $state<Section>();
	const alertCount = $derived(data.alerts.length + data.waitingForConsent.length);
	const sections = $derived<TabItem<Section>[]>(
		visiting
			? [
					{ id: 'domains', label: 'Domains', icon: 'layers', count: visiting.domains.length },
					{ id: 'complete', label: 'Completeness', icon: 'check' }
				]
			: [
					{
						id: 'alerts',
						label: 'Alerts',
						icon: 'alert',
						...(alertCount
							? { badge: { text: String(alertCount), title: `${alertCount} need you` } }
							: { count: 0 })
					},
					{ id: 'domains', label: 'Domains', icon: 'layers', count: station?.domains.length ?? 0 },
					{ id: 'devices', label: 'Devices', icon: 'station', count: data.devices.length },
					{ id: 'labs', label: 'Labs', icon: 'flask', count: data.labs.length },
					{ id: 'records', label: 'Records', icon: 'clipboard', count: data.records.length },
					{ id: 'complete', label: 'Completeness', icon: 'check' }
				]
	);
	let sectionsEl = $state<HTMLElement>();
	function openSection(id: Section) {
		active = id;
		const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
		sectionsEl?.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
	}

	// ---- The shared overview ---------------------------------------------------------------
	const mine = $derived(new Set(data.classrooms.map((c) => c.id)));
	const markers = $derived<GlobeMarker[]>(
		(station?.places ?? []).map((p) => ({
			id: p.id,
			lat: p.location!.lat,
			lng: p.location!.lng,
			color: mine.has(p.id) ? '#3cc04b' : '#8e98a3',
			label: mine.has(p.id) ? `${p.name} · yours` : `${p.name} · public data only`
		}))
	);
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
	// The classroom being opened: the overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		(navigating.to?.route.id === '/(app)/dashboard' &&
			navigating.to.url.searchParams.get('classroom')) ||
			station?.classroom.id ||
			''
	);
	const role = $derived(data.classrooms.find((c) => c.id === pendingId)?.myRole ?? null);

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
	const conditionOf = (id: string) => data.classroomConditions.find((c) => c.device.id === id);
</script>

<svelte:head><title>Dashboard · DaGrouper Analysis</title></svelte:head>

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
{:else}
	<ClassroomOverview
		{station}
		{tabs}
		tabsLabel="Your classrooms"
		{pendingId}
		{markers}
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
					><Icon name="user" class="size-3" /> You're {CLASSROOM_ROLE_LABELS[
						role
					].toLowerCase()}</span
				>
			{:else if visiting}
				<span class="chip">Public data only</span>
			{/if}
		{/snippet}
		{#snippet tabsEnd()}
			<a
				href={resolve('/(app)/classrooms/new')}
				class="flex items-center gap-1.5 pb-2 text-muted hover:text-ink"
				><Icon name="plus" class="size-4" /> Join or start one</a
			>
		{/snippet}
		{#snippet globeLegend()}
			<span class="flex items-center gap-1.5"
				><span class="size-2.5 rounded-full bg-lime"></span>Your classrooms</span
			>
			<span class="flex items-center gap-1.5"
				><span class="size-2 rounded-full bg-muted"></span>Public data only</span
			>
		{/snippet}
		{#snippet nowActions()}
			{#if data.canRecord && !visiting}
				<a class="btn btn-accent flex-1 rounded-full" href={resolve('/(app)/observe')}
					><Icon name="plus" class="size-4" /> Record</a
				>
			{/if}
			<button
				type="button"
				class="btn btn-ghost flex-1 rounded-full"
				onclick={() => openSection('domains')}>View domains</button
			>
		{/snippet}
		{#snippet institutionActions()}
			{#if classroom && role === 'institution_head'}
				<a
					class="btn btn-ghost flex-1 rounded-full"
					href={resolve('/(app)/classrooms/[id]', { id: classroom.id })}>Edit overview</a
				>
			{/if}
		{/snippet}

		<section bind:this={sectionsEl} class="mt-14 scroll-mt-28" aria-label="Classroom sections">
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
								<!-- Opens the alert's value in its domain; the search can be cleared there. -->
								<!-- eslint-disable svelte/no-navigation-without-resolve -->
								<a
									class="btn btn-ghost"
									href={resolve('/(app)/dashboard/domains') +
										domainQuery(station.classroom.id, alert.device.id, {
											domain: domainOf(alert.parameterId, paramsById, data.domains),
											search: label
										})}>Open {label}</a
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
								<!-- eslint-disable svelte/no-navigation-without-resolve -->
								<a
									class="btn btn-ghost"
									href={`${resolve('/(app)/classrooms/[id]', { id: station.classroom.id })}?tab=consent`}
									>Record paper consent</a
								>
								<!-- eslint-enable svelte/no-navigation-without-resolve -->
							</li>
						{/each}
					</ul>
				{:else if active === 'domains'}
					<DomainBrowser
						domains={station.domains}
						parameters={station.parameters}
						classroomId={station.classroom.id}
					/>
				{:else if active === 'devices'}
					<div class="flex flex-wrap items-end justify-between gap-3">
						<div>
							{@render panelHead(
								'Devices',
								`The devices in ${station.classroom.name}'s labs, each with its latest reading.`
							)}
						</div>
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
											? keySummary(option.type, item.reading.values, paramsById)
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
											onclick={() => sync(option.id)}
											><Icon name="sync" class="size-4" /> Sync</button
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
					<ul class="mt-5 grid gap-3 sm:grid-cols-2">
						{#each data.labs as lab (lab.id)}
							{@const count = data.devices.filter((d) => d.labId === lab.id).length}
							<li>
								<a
									class="card block h-full p-5 transition hover:border-line-strong"
									href={resolve('/(app)/labs/[labId]', { labId: lab.id })}
								>
									<Icon name="flask" class="size-6 text-teal" />
									<p class="mt-3 font-medium text-ink">{lab.name}</p>
									<p class="text-sm text-muted">
										{lab.placeName}, {lab.country} · {count} device{count === 1 ? '' : 's'}
									</p>
									{#if lab.siteProfile?.description}
										<p class="mt-2 text-sm">{lab.siteProfile.description}</p>
									{/if}
								</a>
							</li>
						{:else}
							<li class="text-muted">No labs yet.</li>
						{/each}
					</ul>
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
					{@render panelHead(
						'How complete is this month?',
						"Readings received this month, as a share of what the classroom's research protocol expects. Missing days stay empty; they are never filled in."
					)}
					{#if station.completeness}
						<CompletenessCard class="mt-5" domains={station.completeness.domains} />
					{:else}
						<p class="mt-4 text-muted">This classroom has no research protocol yet.</p>
					{/if}
				{/if}
			</VerticalTabs>
		</section>
	</ClassroomOverview>

	{#if classroom && device && data.canRecord && data.labLocation && !manual}
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
				onclose={() => invalidateAll()}
			/>
		{/key}
	{/if}
{/if}
