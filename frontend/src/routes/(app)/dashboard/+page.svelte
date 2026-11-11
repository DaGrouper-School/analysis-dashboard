<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { domainOf } from '#lib/domains.ts';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import CompletenessCard from '#lib/components/CompletenessCard.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import InstitutionOverview from '#lib/components/InstitutionOverview.svelte';
	import NowCard from '#lib/components/NowCard.svelte';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import Sparkline from '#lib/components/Sparkline.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import SyncModal from '#lib/components/SyncModal.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import type {
		InstitutionOverview as InstitutionInfo,
		PublicDomainCard,
		PublicValue,
		ReviewState,
		StationCoreItem
	} from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
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
	const station = $derived(data.station);
	const device = $derived(data.device);
	// A manual measurement has no sensor to sync: it is only ever entered by hand.
	const manual = $derived(!!device && isManualType(data.deviceTypes, device.type));

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const today = new Intl.DateTimeFormat(undefined, {
		weekday: 'long',
		day: 'numeric',
		month: 'long'
	});

	/** Opens another classroom or sensor; the pick lives in the address, with the open tab. */
	function choose(classroomId: string, deviceId?: string, tab = active) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		return goto(resolve('/(app)/dashboard') + sensorQuery(classroomId, deviceId) + `&tab=${tab}`, {
			replace: true,
			reset: false
		});
	}

	// ---- The section tabs (AP-45) ----------------------------------------------------------
	type Section = 'alerts' | 'domains' | 'devices' | 'labs' | 'records' | 'complete';
	let active = $state<Section>();
	const alertCount = $derived(data.alerts.length + data.waitingForConsent.length);
	const sections = $derived<TabItem<Section>[]>([
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
	]);
	let sectionsEl = $state<HTMLElement>();
	function openSection(id: Section) {
		active = id;
		sectionsEl?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	// ---- The three columns -----------------------------------------------------------------
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
	// The globe turns to a classroom picked on it; the tabs above choose the dashboard's.
	let globeSelected = $derived(data.classroom?.id ?? '');
	const institution = $derived<InstitutionInfo | null>(
		station
			? (station.institution ?? {
					name: station.classroom.name,
					location: station.classroom.location,
					deviceTypes: [],
					photoUrl: null,
					photoAlt: '',
					history: '',
					hub: null,
					website: null
				})
			: null
	);
	const stage = $derived(station?.content.stages.find((s) => s.id === station.classroom.stageId));

	// ---- Core readings ---------------------------------------------------------------------
	const REVIEW: Record<ReviewState, string> = {
		released: 'Released',
		in_review: 'Newer data in review',
		not_released: 'Not released yet'
	};
	const REVIEW_TONE: Record<ReviewState, string> = {
		released: 'bg-lime',
		in_review: 'bg-orange',
		not_released: 'bg-muted'
	};
	/** How the sources behind a core reading compare, in plain words. */
	function agreement(item: StationCoreItem, unit: string) {
		if (item.sources === 0) return '';
		if (item.sources === 1 || item.spread === null) return 'From one source';
		return item.spread === 0
			? `${item.sources} sources, all the same`
			: `${item.sources} sources, ${formatValue(item.spread, unit)} apart`;
	}

	// ---- Domains ---------------------------------------------------------------------------
	const CARD_READINGS = 3;
	const domainKey = (domain: { id: string } | null) => domain?.id ?? 'other';
	const valueKey = (value: PublicValue) => `${value.source.id}|${value.parameterId}`;

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

{#snippet review(state: ReviewState)}
	<span class="chip">
		<span class={['size-2 rounded-full', REVIEW_TONE[state]]} aria-hidden="true"></span>
		{REVIEW[state]}
	</span>
{/snippet}

{#snippet stat(
	label: string,
	value: number | null,
	unit: string,
	state: ReviewState,
	notes: string[]
)}
	<div class="card flex min-h-39 flex-col gap-2 p-5">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<p class="text-sm text-muted">{label}</p>
			{@render review(state)}
		</div>
		<p class="font-mono text-3xl text-ink tabular-nums">
			{#if value === null}<span class="font-sans text-base text-muted italic">no data</span
				>{:else}{formatValue(value, unit)}{/if}
		</p>
		{#each notes.filter(Boolean) as note (note)}
			<p class="text-xs text-muted">{note}</p>
		{/each}
	</div>
{/snippet}

{#snippet panelHead(title: string, sub: string)}
	<h2 class="font-display text-2xl">{title}</h2>
	<p class="mt-1 max-w-2xl text-sm text-muted">{sub}</p>
{/snippet}

{#snippet domainCard(card: PublicDomainCard, classroomId: string)}
	<article class="card flex flex-col p-5">
		<h3 class="flex items-center gap-2 font-medium text-ink">
			<Icon
				name={card.domain?.icon ?? FALLBACK_DOMAIN_ICON}
				fallback={FALLBACK_DOMAIN_ICON}
				class="size-5 text-teal"
			/>
			{card.domain?.label ?? 'Not in a domain yet'}
		</h3>
		<ul class="mt-2 divide-y divide-line">
			{#each card.values.slice(0, CARD_READINGS) as value (valueKey(value))}
				{@const param = paramsById.get(value.parameterId)}
				<li class="flex items-center justify-between gap-3 py-2.5">
					<span class="min-w-0">
						<a
							class="link text-sm"
							href={resolve('/explore/[classroom]/[parameter]', {
								classroom: classroomId,
								parameter: value.parameterId
							})}>{param?.label ?? value.parameterId}</a
						>
						<span class="block truncate text-xs text-muted"
							>{value.source.name ?? 'Field observations'}</span
						>
					</span>
					<span class="flex shrink-0 items-center gap-3">
						<Sparkline values={value.last7Days} />
						<span
							class="min-w-16 text-right font-mono text-sm whitespace-nowrap text-ink tabular-nums"
						>
							{formatValue(value.value, param?.unit, param?.valueType)}
						</span>
					</span>
				</li>
			{/each}
		</ul>
		{#if card.values.length > CARD_READINGS}
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				class="link mt-auto self-start pt-3 text-sm"
				href={`${resolve('/explore/[classroom]', { classroom: classroomId })}?domain=${domainKey(card.domain)}`}
				>All {card.values.length} readings</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/if}
	</article>
{/snippet}

{#if data.invitations.length}
	<div class="mb-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

{#if !data.classroom || !station}
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
	{@const classroom = data.classroom}
	{@const core = station.classroom.core}
	<header class="flex flex-wrap items-start justify-between gap-4">
		<div class="min-w-0">
			<p class="text-sm font-medium text-muted">
				Hello, {firstName} · {today.format(new Date())}
			</p>
			<h1 class="mt-2 font-display text-4xl">{classroom.name}</h1>
			<p class="mt-2 flex flex-wrap items-center gap-2 text-muted">
				{[station.classroom.placeName, station.classroom.country].filter(Boolean).join(', ') ||
					classroom.institutionName}
				<StageBadge {stage} stageId={station.classroom.stageId} />
				{#if classroom.myRole}
					<span class="chip"
						><Icon name="user" class="size-3" /> You're {CLASSROOM_ROLE_LABELS[
							classroom.myRole
						].toLowerCase()}</span
					>
				{/if}
			</p>
			<p class="mt-1 text-sm text-muted">
				{core.lastSync ? `Last sync ${dateTime.format(new Date(core.lastSync))}` : 'No sync yet'}
				{#if station.siteCodes.length}
					· Site codes <span class="font-mono text-text">{station.siteCodes.join(', ')}</span>
				{/if}
			</p>
		</div>
		<a class="btn btn-ghost" href={resolve('/explore/[classroom]', { classroom: classroom.id })}
			>See the public page <Icon name="arrow" class="size-4" /></a
		>
	</header>

	<!-- Your classrooms, as tabs over the three columns, like the location tabs on the public page. -->
	<nav
		aria-label="Your classrooms"
		class="-mx-4 mt-7 flex gap-7 overflow-x-auto px-4 text-sm font-medium whitespace-nowrap sm:justify-center"
	>
		{#each data.classrooms as option (option.id)}
			{@const current = option.id === classroom.id}
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				href={resolve('/(app)/dashboard') + sensorQuery(option.id)}
				aria-current={current ? 'page' : undefined}
				class={[
					'border-b-2 pb-2 transition-colors',
					current ? 'border-teal text-ink' : 'border-transparent text-muted hover:text-ink'
				]}>{option.name}</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/each}
		<a
			href={resolve('/(app)/classrooms/new')}
			class="flex items-center gap-1.5 border-b-2 border-transparent pb-2 text-muted hover:text-ink"
			><Icon name="plus" class="size-4" /> Join or start one</a
		>
	</nav>

	<div
		class={[
			'mt-5 grid gap-5 lg:h-[34rem]',
			station.classroom.location
				? 'lg:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)_minmax(0,1fr)]'
				: 'lg:grid-cols-2'
		]}
	>
		<NowCard tiles={station.now}>
			{#snippet actions()}
				{#if data.canRecord}
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
		</NowCard>
		{#if station.classroom.location}
			<section
				class="card relative h-80 overflow-hidden p-0 sm:h-96 lg:h-full"
				aria-label="Where {classroom.name} is"
			>
				<div class="absolute inset-0">
					{#key classroom.id}
						<Globe
							{markers}
							bind:selectedId={globeSelected}
							ariaLabel="Globe of classrooms, turned to {classroom.name}"
						/>
					{/key}
				</div>
				<p
					class="pointer-events-none absolute top-4 left-4 flex flex-wrap gap-3 text-xs text-text"
					aria-hidden="true"
				>
					<span class="flex items-center gap-1.5"
						><span class="size-2.5 rounded-full bg-lime"></span>Your classrooms</span
					>
					<span class="flex items-center gap-1.5"
						><span class="size-2 rounded-full bg-muted"></span>Public data only</span
					>
				</p>
				<span
					class="pointer-events-none absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-line-strong bg-surface px-3 py-1 text-xs whitespace-nowrap text-ink"
				>
					{markers.find((m) => m.id === globeSelected)?.label ?? classroom.name}
				</span>
			</section>
		{/if}
		{#if institution}
			<InstitutionOverview overview={institution} class="overflow-hidden">
				{#snippet actions()}
					{#if classroom.myRole === 'institution_head'}
						<a
							class="btn btn-ghost flex-1 rounded-full"
							href={resolve('/(app)/classrooms/[id]', { id: classroom.id })}>Edit overview</a
						>
					{/if}
				{/snippet}
			</InstitutionOverview>
		{/if}
	</div>

	<section class="mt-5" aria-label="Core readings">
		<div class="grid gap-3 sm:grid-cols-3">
			{@render stat('Rain today', core.rainToday, station.units.rain, station.core.rain.review, [
				agreement(station.core.rain, station.units.rain)
			])}
			{@render stat(
				'Rain, last 7 days',
				core.rainLast7Days,
				station.units.rain,
				station.core.rain.review,
				[core.rainLast7Days !== null && core.rainDays < 7 ? `${core.rainDays} of 7 days` : '']
			)}
			{@render stat(
				'Air temperature now',
				core.airTemperature,
				station.units.temperature,
				station.core.temperature.review,
				[
					station.core.temperature.low !== null && station.core.temperature.high !== null
						? `Today ${formatValue(station.core.temperature.low, station.units.temperature)} to ${formatValue(station.core.temperature.high, station.units.temperature)}`
						: '',
					agreement(station.core.temperature, station.units.temperature)
				]
			)}
		</div>
	</section>

	<section bind:this={sectionsEl} class="mt-14 scroll-mt-28" aria-label="Classroom sections">
		<VerticalTabs tabs={sections} bind:active label="Classroom sections">
			{#if active === 'alerts'}
				{@render panelHead(
					'Alerts',
					alertCount
						? `What needs you in ${classroom.name}, from each device's latest reading.`
						: `Nothing needs you in ${classroom.name} right now.`
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
									domainQuery(classroom.id, alert.device.id, {
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
								href={`${resolve('/(app)/classrooms/[id]', { id: classroom.id })}?tab=consent`}
								>Record paper consent</a
							>
							<!-- eslint-enable svelte/no-navigation-without-resolve -->
						</li>
					{/each}
				</ul>
			{:else if active === 'domains'}
				{@render panelHead(
					'What this classroom measures',
					'The latest released value of each, with the last 7 days.'
				)}
				{#if station.domains.length}
					<div class="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
						{#each station.domains as card (domainKey(card.domain))}
							{@render domainCard(card, classroom.id)}
						{/each}
					</div>
				{:else}
					<p class="mt-4 text-muted">Nothing from this classroom has been released yet.</p>
				{/if}
			{:else if active === 'devices'}
				<div class="flex flex-wrap items-end justify-between gap-3">
					<div>
						{@render panelHead(
							'Devices',
							`The devices in ${classroom.name}'s labs, each with its latest reading.`
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
									href={resolve('/(app)/dashboard/readings') + sensorQuery(classroom.id, option.id)}
									>Raw data</a
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
					`${classroom.name}'s labs, with their devices and site profile.`
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

	{#if device && data.canRecord && data.labLocation && !manual}
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
