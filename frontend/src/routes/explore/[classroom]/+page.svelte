<script lang="ts">
	import { untrack } from 'svelte';
	import { resolve } from '$app/paths';
	import { goto, replaceState } from '$app/navigation';
	import { navigating, page } from '$app/state';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import InstitutionOverview from '#lib/components/InstitutionOverview.svelte';
	import NowCard from '#lib/components/NowCard.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import Sparkline from '#lib/components/Sparkline.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import type {
		DomainCompleteness,
		InstitutionOverview as InstitutionInfo,
		PublicDomainCard,
		PublicSource,
		PublicValue,
		ReviewState,
		StationCoreItem
	} from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
	import { formatValue } from '#lib/key-values.ts';
	import { placeLabel } from '#lib/station-now.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const core = $derived(classroom.core);
	const params = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const explore = $derived(data.content.sections.find((s) => s.id === 'explore'));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const dayMonth = new Intl.DateTimeFormat(undefined, {
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	});
	const monthName = new Intl.DateTimeFormat(undefined, {
		month: 'long',
		year: 'numeric',
		timeZone: 'UTC'
	});

	// The location tabs and the globe (AP-43): every public classroom, this one selected.
	const stageColor = $derived(new Map(data.content.stages.map((s) => [s.id, s.color])));
	const markers = $derived<GlobeMarker[]>(
		data.places.map((p) => ({
			id: p.id,
			lat: p.location!.lat,
			lng: p.location!.lng,
			color: stageColor.get(p.stageId) ?? '#8e98a3',
			label: p.name
		}))
	);
	// The classroom being opened: the tabs, the globe and the title move to it at once, and the
	// rest of the page shows a skeleton until its data arrives, instead of blinking.
	const pendingId = $derived(
		(navigating.to?.route.id === '/explore/[classroom]' && navigating.to.params?.classroom) ||
			classroom.id
	);
	const loading = $derived(pendingId !== classroom.id);
	const shownPlace = $derived(data.places.find((p) => p.id === pendingId));
	const shownName = $derived(shownPlace?.name ?? classroom.name);
	const shownStageId = $derived(shownPlace?.stageId ?? classroom.stageId);
	const shownStage = $derived(data.content.stages.find((s) => s.id === shownStageId));
	const institution = $derived<InstitutionInfo>(
		data.institution ?? {
			name: classroom.name,
			location: classroom.location,
			deviceTypes: [],
			photoUrl: null,
			photoAlt: '',
			history: '',
			hub: null,
			website: null
		}
	);
	// The sliding underline under the location tabs, measured from the tab being opened.
	const tabEls: Record<string, HTMLAnchorElement> = $state({});
	let navWidth = $state(0);
	const underline = $derived.by(() => {
		void navWidth;
		const el = tabEls[pendingId];
		return el ? { left: el.offsetLeft, width: el.offsetWidth } : null;
	});
	// Follows the classroom being opened; picking a marker overrides it until the next page.
	let globeSelected = $derived(pendingId);
	// Picking another classroom on the globe opens its station.
	$effect(() => {
		const id = globeSelected;
		if (id && id !== untrack(() => pendingId)) {
			goto(resolve('/explore/[classroom]', { classroom: id }));
		}
	});

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
	/** A card shows this many readings; its domain tab shows them all. */
	const CARD_READINGS = 3;

	const sourceName = (source: PublicSource) => source.name ?? 'Field observations';
	const domainKey = (domain: { id: string } | null) => domain?.id ?? 'other';
	const valueKey = (value: PublicValue) => `${value.source.id}|${value.parameterId}`;

	/** How the sources behind a core reading compare, in plain words. */
	function agreement(item: StationCoreItem, unit: string) {
		if (item.sources === 0) return '';
		if (item.sources === 1 || item.spread === null) return 'From one source';
		return item.spread === 0
			? `${item.sources} sources, all the same`
			: `${item.sources} sources, ${formatValue(item.spread, unit)} apart`;
	}

	// The open domain tab, kept in the URL (`?domain=`) so a shared link opens the same one.
	let active = $state('all');
	untrack(() => {
		const wanted = page.url.searchParams.get('domain');
		if (wanted && data.domains.some((c) => domainKey(c.domain) === wanted)) active = wanted;
	});
	const tabs = $derived([
		{ id: 'all', label: 'All domains', icon: 'layers', count: undefined as number | undefined },
		...data.domains.map((card) => ({
			id: domainKey(card.domain),
			label: card.domain?.label ?? 'Other',
			icon: card.domain?.icon ?? FALLBACK_DOMAIN_ICON,
			count: card.values.length
		}))
	]);
	const openCard = $derived(data.domains.find((c) => domainKey(c.domain) === active));

	function open(id: string) {
		active = id;
		const url = new URL(page.url.href);
		if (id === 'all') url.searchParams.delete('domain');
		else url.searchParams.set('domain', id);
		// The current page's own URL, with only the domain changed.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		replaceState(url, page.state);
	}

	function keys(event: KeyboardEvent) {
		const step = { ArrowRight: 1, ArrowLeft: -1 }[event.key];
		if (!step) return;
		event.preventDefault();
		const i = tabs.findIndex((t) => t.id === active);
		open(tabs[(i + step + tabs.length) % tabs.length].id);
		document.getElementById(`domain-tab-${active}`)?.focus();
	}

	const percent = (d: DomainCompleteness) =>
		d.expected ? Math.round((d.received / d.expected) * 100) : 0;
	const missingDays = (d: DomainCompleteness) =>
		d.days.filter((day) => day.status === 'missing').map((day) => day.day);
	const DAY_TONE = {
		complete: 'bg-teal',
		partial: 'bg-teal/40',
		missing: 'border border-dashed border-line-strong'
	} as const;
	const DAY_WORD = { complete: 'complete', partial: 'partly received', missing: 'no readings' };
</script>

<svelte:head><title>{classroom.name} · {data.content.hero.eyebrow}</title></svelte:head>

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
	<!-- A steady height, so a classroom with less to say does not move what is below. -->
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

{#snippet reading(value: PublicValue, compact: boolean)}
	{@const param = params.get(value.parameterId)}
	<li class="flex items-center justify-between gap-3 py-2.5">
		<span class="min-w-0">
			<a
				class="link"
				href={resolve('/explore/[classroom]/[parameter]', {
					classroom: classroom.id,
					parameter: value.parameterId
				})}>{param?.label ?? value.parameterId}</a
			>
			<span class="block truncate text-xs text-muted">
				{sourceName(value.source)}{compact
					? ''
					: ` · ${dateTime.format(new Date(value.observedAt))}`}
			</span>
		</span>
		<span class="flex shrink-0 items-center gap-3">
			<Sparkline values={value.last7Days} />
			<span
				class={[
					'max-w-40 min-w-20 text-right font-mono text-ink tabular-nums',
					typeof value.value === 'number' && 'whitespace-nowrap'
				]}
			>
				{formatValue(value.value, param?.unit, param?.valueType)}
			</span>
		</span>
	</li>
{/snippet}

{#snippet domainTitle(card: PublicDomainCard)}
	<Icon
		name={card.domain?.icon ?? FALLBACK_DOMAIN_ICON}
		fallback={FALLBACK_DOMAIN_ICON}
		class="size-5 text-muted"
	/>
	{card.domain?.label ?? 'Other'}
{/snippet}

<PublicShell content={data.content} user={data.user}>
	<nav aria-label="Breadcrumb" class="text-sm text-muted">
		<a class="hover:text-ink" href={resolve('/explore')}>{explore?.label ?? 'Explore data'}</a>
		{#if shownPlace?.country ?? classroom.country}<span aria-hidden="true" class="px-1">›</span
			>{shownPlace?.country ?? classroom.country}{/if}
		<span aria-hidden="true" class="px-1">›</span><span class="text-ink" aria-current="page"
			>{shownName}</span
		>
	</nav>

	<header
		class={['mt-4 flex flex-wrap items-start justify-between gap-4', loading && 'is-loading']}
	>
		<div class="min-w-0">
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="font-display text-4xl">{shownName}</h1>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
			<p class="mt-2 flex flex-wrap items-center gap-2 text-muted">
				{[shownPlace?.placeName ?? classroom.placeName, shownPlace?.country ?? classroom.country]
					.filter(Boolean)
					.join(', ') || classroom.institutionName}
				<StageBadge stage={shownStage} stageId={shownStageId} />
			</p>
			<p class="mt-1 w-fit text-sm text-muted" data-skeleton>
				{core.lastSync ? `Last sync ${dateTime.format(new Date(core.lastSync))}` : 'No sync yet'}
				{#if data.siteCodes.length}
					· Site codes
					<span class="font-mono text-text">{data.siteCodes.join(', ')}</span>
				{/if}
			</p>
		</div>
		{#if data.member}
			<!-- Members also see raw values and data still in review on their dashboard. -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				class="btn btn-ghost"
				href={`${resolve('/(app)/dashboard')}?classroom=${encodeURIComponent(classroom.id)}`}
				>Open in your dashboard</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/if}
	</header>

	{#if data.places.length > 1}
		<nav
			aria-label="Locations"
			class="relative -mx-4 mt-6 flex gap-6 overflow-x-auto px-4 text-sm font-medium whitespace-nowrap sm:justify-center"
			bind:clientWidth={navWidth}
		>
			{#each data.places as place (place.id)}
				{@const current = place.id === pendingId}
				<a
					bind:this={tabEls[place.id]}
					href={resolve('/explore/[classroom]', { classroom: place.id })}
					data-sveltekit-noscroll
					aria-current={current ? 'page' : undefined}
					title={place.name}
					class={['pb-2 transition-colors', current ? 'text-ink' : 'text-muted hover:text-ink']}
					>{placeLabel(place.placeName, place.country) || place.name}</a
				>
			{/each}
			<!-- One underline that slides to the tab being opened, as soon as it is picked. -->
			{#if underline}
				<span
					class="pointer-events-none absolute bottom-0 h-0.5 rounded-full bg-teal transition-[left,width] duration-300 ease-out motion-reduce:transition-none"
					style:left="{underline.left}px"
					style:width="{underline.width}px"
					aria-hidden="true"
				></span>
			{/if}
		</nav>
	{/if}

	<div class={['contents', loading && 'is-loading']} aria-busy={loading}>
		<div
			class={[
				'mt-6 grid gap-5 lg:h-[34rem]',
				classroom.location
					? 'lg:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)_minmax(0,1fr)]'
					: 'lg:grid-cols-2'
			]}
		>
			<NowCard tiles={data.now}>
				{#snippet actions()}
					<a class="btn btn-ghost flex-1 rounded-full" href="#domains-heading">View domains</a>
				{/snippet}
			</NowCard>
			{#if classroom.location}
				<section
					class="card relative h-80 overflow-hidden p-0 [view-transition-name:station-map] sm:h-96 lg:h-full"
					aria-label="Where {shownName} is"
				>
					<!-- Out of the flow, so the canvas never sets the card's height and grows it. -->
					<div class="absolute inset-0">
						<Globe
							{markers}
							bind:selectedId={globeSelected}
							ariaLabel="Globe of Global Classrooms, turned to {shownName}"
						/>
					</div>
					<span
						class="pointer-events-none absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-line-strong bg-surface px-3 py-1 text-xs whitespace-nowrap text-ink"
					>
						{shownName}
					</span>
				</section>
			{/if}
			<!-- Always a card, with just the name until there is more, so the row never reflows. -->
			<InstitutionOverview overview={institution} class="overflow-hidden" />
		</div>

		<section class="mt-6" aria-labelledby="core-heading">
			<h2 id="core-heading" class="sr-only">Core readings</h2>
			<div class="grid gap-3 sm:grid-cols-3">
				{@render stat('Rain today', core.rainToday, data.units.rain, data.core.rain.review, [
					agreement(data.core.rain, data.units.rain)
				])}
				{@render stat(
					'Rain, last 7 days',
					core.rainLast7Days,
					data.units.rain,
					data.core.rain.review,
					[core.rainLast7Days !== null && core.rainDays < 7 ? `${core.rainDays} of 7 days` : '']
				)}
				{@render stat(
					'Air temperature now',
					core.airTemperature,
					data.units.temperature,
					data.core.temperature.review,
					[
						data.core.temperature.low !== null && data.core.temperature.high !== null
							? `Today ${formatValue(data.core.temperature.low, data.units.temperature)} to ${formatValue(data.core.temperature.high, data.units.temperature)}`
							: '',
						agreement(data.core.temperature, data.units.temperature)
					]
				)}
			</div>
		</section>

		<section class="mt-10" aria-labelledby="domains-heading">
			<h2 id="domains-heading" class="font-display text-2xl">What this classroom measures</h2>
			{#if data.domains.length}
				<div
					class="mt-4 flex gap-1 overflow-x-auto border-b border-line"
					role="tablist"
					aria-label="Domains"
					tabindex="-1"
					onkeydown={keys}
				>
					{#each tabs as tab (tab.id)}
						{@const selected = tab.id === active}
						<button
							type="button"
							role="tab"
							id="domain-tab-{tab.id}"
							aria-selected={selected}
							aria-controls="domain-panel"
							tabindex={selected ? 0 : -1}
							class={[
								'-mb-px flex shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm whitespace-nowrap transition',
								selected
									? 'border-ink font-medium text-ink'
									: 'border-transparent text-muted hover:text-ink'
							]}
							onclick={() => open(tab.id)}
						>
							<Icon name={tab.icon} fallback={FALLBACK_DOMAIN_ICON} class="size-4" />
							{tab.label}
							{#if tab.count !== undefined}<span class="text-xs text-muted">{tab.count}</span>{/if}
						</button>
					{/each}
				</div>

				<div
					id="domain-panel"
					class="mt-4"
					role="tabpanel"
					aria-labelledby="domain-tab-{active}"
					tabindex="-1"
				>
					{#if openCard}
						<article class="card">
							<h3 class="flex items-center gap-2 text-lg font-semibold">
								{@render domainTitle(openCard)}
							</h3>
							{#if openCard.domain?.description}
								<p class="mt-1 text-sm text-muted">{openCard.domain.description}</p>
							{/if}
							<p class="mt-3 text-xs text-muted">
								Sparklines show the last reading of each of 7 days.
							</p>
							<ul class="mt-2 divide-y divide-line">
								{#each openCard.values as value (valueKey(value))}
									{@render reading(value, false)}
								{/each}
							</ul>
						</article>
					{:else}
						<div class="grid gap-4 md:grid-cols-2">
							{#each data.domains as card (domainKey(card.domain))}
								<article class="card flex flex-col">
									<h3 class="text-lg font-semibold">
										<button
											type="button"
											class="flex items-center gap-2 hover:text-teal"
											onclick={() => open(domainKey(card.domain))}
										>
											{@render domainTitle(card)}
										</button>
									</h3>
									<ul class="mt-3 divide-y divide-line">
										{#each card.values.slice(0, CARD_READINGS) as value (valueKey(value))}
											{@render reading(value, true)}
										{/each}
									</ul>
									<button
										type="button"
										class="link mt-auto self-start pt-3 text-sm"
										onclick={() => open(domainKey(card.domain))}
									>
										{card.values.length > CARD_READINGS
											? `All ${card.values.length} readings`
											: `Open ${card.domain?.label ?? 'Other'}`}
									</button>
								</article>
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<p class="mt-3 text-muted">Nothing from this classroom has been released yet.</p>
			{/if}
		</section>

		{#if data.completeness}
			{@const completeness = data.completeness}
			<section class="mt-10" aria-labelledby="complete-heading">
				<h2 id="complete-heading" class="font-display text-2xl">How complete is this month?</h2>
				<p class="mt-1 max-w-2xl text-sm text-muted">
					Readings received in {monthName.format(new Date(`${completeness.month}-01T00:00:00Z`))},
					as a share of what the classroom's research protocol expects. Missing days stay empty;
					they are never filled in.
				</p>
				<div class="card mt-4 divide-y divide-line p-0">
					{#each completeness.domains as d (domainKey(d.domain))}
						{@const missing = missingDays(d)}
						<div class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center">
							<p class="flex w-40 shrink-0 items-center gap-2 font-medium text-ink">
								<Icon
									name={d.domain?.icon ?? FALLBACK_DOMAIN_ICON}
									fallback={FALLBACK_DOMAIN_ICON}
									class="size-4 text-muted"
								/>
								{d.domain?.label ?? 'Other'}
							</p>
							<p class="w-36 shrink-0 text-sm">
								<span class="font-mono text-ink tabular-nums">{percent(d)}%</span>
								<span class="text-muted">· {d.received} of {d.expected}</span>
							</p>
							{#if d.days.length}
								<div class="min-w-0 flex-1">
									<ol class="flex flex-wrap gap-1" aria-label="Days of the month">
										{#each d.days as day (day.day)}
											<li
												class={['size-3.5 rounded-[3px]', DAY_TONE[day.status]]}
												title="{dayMonth.format(new Date(`${day.day}T00:00:00Z`))}: {DAY_WORD[
													day.status
												]}"
											>
												<span class="sr-only"
													>{dayMonth.format(new Date(`${day.day}T00:00:00Z`))}: {DAY_WORD[
														day.status
													]}</span
												>
											</li>
										{/each}
									</ol>
									{#if missing.length}
										<p class="mt-1.5 text-xs text-muted">
											No readings on {missing
												.map((m) => dayMonth.format(new Date(`${m}T00:00:00Z`)))
												.join(', ')}
										</p>
									{/if}
								</div>
							{:else}
								<p class="text-sm text-muted">Measured weekly</p>
							{/if}
						</div>
					{/each}
				</div>
				<p class="mt-2 flex flex-wrap gap-4 text-xs text-muted" aria-hidden="true">
					<span class="flex items-center gap-1.5"
						><span class="size-3 rounded-[3px] bg-teal"></span>Complete</span
					>
					<span class="flex items-center gap-1.5"
						><span class="size-3 rounded-[3px] bg-teal/40"></span>Partly received</span
					>
					<span class="flex items-center gap-1.5"
						><span class="size-3 rounded-[3px] border border-dashed border-line-strong"></span>No
						readings</span
					>
				</p>
			</section>
		{/if}
	</div>
</PublicShell>
