<script lang="ts" module>
	import type { PublicStation } from '#lib/domain/types.ts';

	/** One classroom tab: where it opens, and its full name when the label is shorter. */
	export interface ClassroomTab {
		id: string;
		label: string;
		href: string;
		title?: string;
	}

	/** The classroom shown while the next one loads: what is known before its data arrives. */
	export type ShownPlace = Pick<
		PublicStation['places'][number],
		'id' | 'name' | 'placeName' | 'country' | 'stageId'
	>;
</script>

<script lang="ts">
	import { untrack, type Snippet } from 'svelte';
	import CoreStrip from '#lib/components/CoreStrip.svelte';
	import DeviceStrip from '#lib/components/DeviceStrip.svelte';
	import DevicePicker, { type PickableDevice } from '#lib/components/DevicePicker.svelte';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import InstitutionOverview from '#lib/components/InstitutionOverview.svelte';
	import NowCard from '#lib/components/NowCard.svelte';
	import Select from '#lib/components/Select.svelte';
	import { MOBILE_TABS_MAX } from '#lib/tabs.ts';
	import { goto } from '$app/navigation';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import type {
		DeviceSummaryCard,
		InstitutionOverview as InstitutionInfo,
		NowTile
	} from '#lib/domain/types.ts';
	import { stageColor } from '#lib/labels.ts';
	import { valueHref } from '#lib/site.ts';

	/**
	 * A classroom at a glance (AP-43, AP-45), the same on its public page and on the dashboard:
	 * the header, the classroom tabs, what's happening now, the globe, the institution and the core
	 * readings. Switching classrooms keeps the page: the underline slides, the
	 * globe turns and the title changes at once, and the cards show a skeleton until the next
	 * classroom's data arrives. What differs between the pages comes in as snippets.
	 */
	let {
		station,
		tabs,
		tabsLabel,
		pendingId,
		markerLabel,
		places,
		eyebrow,
		badges,
		headerActions,
		tabsStart,
		nowActions,
		devices,
		deviceId,
		onpickdevice,
		now,
		accent,
		summary,
		released = true,
		onexplain,
		institutionActions,
		onpick,
		children
	}: {
		station: PublicStation;
		tabs: ClassroomTab[];
		tabsLabel: string;
		/** The classroom being opened, or the one shown when nothing is loading. */
		pendingId: string;
		/** A classroom's name on the globe, such as "· yours" on the dashboard. */
		markerLabel?: (place: ShownPlace) => string;
		/** The classrooms on the globe, such as the ones a filter keeps; all of them by default. */
		places?: PublicStation['places'];
		/** Above the title: a breadcrumb or a greeting, for the classroom being opened. */
		eyebrow?: Snippet<[ShownPlace]>;
		/** Beside the stage, such as the viewer's role. */
		badges?: Snippet;
		headerActions?: Snippet;
		/** Above the classroom tabs, such as the public page's filter. */
		tabsStart?: Snippet;
		nowActions?: Snippet;
		/** The device picker on "What's happening now?": its devices, the one shown, and a pick. */
		devices?: PickableDevice[];
		deviceId?: string;
		onpickdevice?: (id: string) => void;
		/** The tiles of the picked device, in place of the classroom's. */
		now?: NowTile[];
		/** Tints "What's happening now?", such as with the picked device's colour. */
		accent?: string;
		/** The picked device's card numbers from the Registry, charted in place of the classroom's core readings. */
		summary?: DeviceSummaryCard[];
		/** The tiles are released data only; false for a member's raw readings. */
		released?: boolean;
		/** Opens how a value is calculated; signed-in pages only. */
		onexplain?: (parameterId: string) => void;
		institutionActions?: Snippet;
		/** A classroom picked on the globe. */
		onpick?: (id: string) => void;
		/** The rest of the page, under the core readings; it shows the skeleton too. */
		children?: Snippet;
	} = $props();

	const classroom = $derived(station.classroom);
	/** A tile's or card's value page, on the picked device. Value pages are for members only. */
	const valueOf = $derived(
		station.member
			? (parameterId: string) => valueHref(classroom.id, parameterId, deviceId)
			: undefined
	);
	const loading = $derived(pendingId !== classroom.id);
	const shown = $derived<ShownPlace>(
		station.places.find((p) => p.id === pendingId) ?? {
			id: pendingId,
			name: tabs.find((t) => t.id === pendingId)?.title ?? classroom.name,
			placeName: loading ? null : classroom.placeName,
			country: loading ? null : classroom.country,
			stageId: classroom.stageId
		}
	);
	const stage = $derived(station.content.stages.find((s) => s.id === shown.stageId));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const core = $derived(classroom.core);
	// Always a card, with just the name until there is more, so the row never reflows.
	const institution = $derived<InstitutionInfo>(
		station.institution ?? {
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

	// The sliding underline under the classroom tabs, measured from the tab being opened.
	const tabEls: Record<string, HTMLAnchorElement> = $state({});
	let navWidth = $state(0);
	const underline = $derived.by(() => {
		void navWidth;
		const el = tabEls[pendingId];
		return el ? { left: el.offsetLeft, width: el.offsetWidth } : null;
	});

	// Follows the classroom being opened; picking a marker overrides it until the next page.
	let globeSelected = $derived(pendingId);
	$effect(() => {
		const id = globeSelected;
		if (id && id !== untrack(() => pendingId)) onpick?.(id);
	});
	// Every classroom with a place, in its stage's colour from the site's stages, with a legend.
	const stages = $derived(station.content.stages);
	const markers = $derived<GlobeMarker[]>(
		(places ?? station.places)
			.filter((p) => p.location)
			.map((p) => ({
				id: p.id,
				lat: p.location!.lat,
				lng: p.location!.lng,
				color: stageColor(stages, p.stageId),
				label: markerLabel?.(p) ?? p.name
			}))
	);
	const legend = $derived(
		stages.filter((s) => (places ?? station.places).some((p) => p.stageId === s.id))
	);
	const pill = $derived(markers.find((m) => m.id === globeSelected)?.label ?? shown.name);
</script>

{@render eyebrow?.(shown)}

<header class={['mt-4 flex flex-wrap items-start justify-between gap-4', loading && 'is-loading']}>
	<div class="min-w-0">
		<h1 class="font-display text-4xl">{shown.name}</h1>
		<p class="mt-2 flex flex-wrap items-center gap-2 text-muted">
			{[shown.placeName, shown.country].filter(Boolean).join(', ') || classroom.institutionName}
			<StageBadge {stage} stageId={shown.stageId} />
			{@render badges?.()}
		</p>
		<p class="mt-1 w-fit text-sm text-muted" data-skeleton>
			{core.lastSync ? `Last sync ${dateTime.format(new Date(core.lastSync))}` : 'No sync yet'}
			{#if station.siteCodes.length}
				· Site codes <span class="font-mono text-text">{station.siteCodes.join(', ')}</span>
			{/if}
		</p>
	</div>
	{@render headerActions?.()}
</header>

{#if tabsStart}<div class="mt-6">{@render tabsStart()}</div>{/if}

{#if tabs.length > MOBILE_TABS_MAX}
	<Select
		options={tabs.map((t) => ({ value: t.id, label: t.label, hint: t.title }))}
		value={pendingId}
		label={tabsLabel}
		class="mt-6 w-full lg:hidden"
		onchange={(id) => {
			if (onpick) return onpick(id);
			const tab = tabs.find((t) => t.id === id);
			// Callers pass resolved paths.
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			if (tab) goto(tab.href);
		}}
	/>
{/if}
{#if tabs.length > 1 || tabsStart}
	<nav
		class:max-lg:hidden={tabs.length > MOBILE_TABS_MAX}
		aria-label={tabsLabel}
		class="relative -mx-4 mt-6 flex gap-7 overflow-x-auto px-4 text-sm font-medium whitespace-nowrap sm:justify-center"
		bind:clientWidth={navWidth}
	>
		{#each tabs as tab (tab.id)}
			{@const current = tab.id === pendingId}
			<!-- Callers pass resolved paths. -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				bind:this={tabEls[tab.id]}
				href={tab.href}
				data-sveltekit-noscroll
				aria-current={current ? 'page' : undefined}
				title={tab.title}
				class={['pb-2 transition-colors', current ? 'text-ink' : 'text-muted hover:text-ink']}
				>{tab.label}</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
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

<!-- The picked device's colour runs through every card below (AP-45). -->
<div
	class={['contents', loading && 'is-loading', accent && 'accent-scope']}
	style:--accent={accent}
	aria-busy={loading}
>
	<!-- On a phone: the globe and the institution first, then the device, what's happening and
	     its cards. On a wide screen: the device picker, then the three columns, then the cards. -->
	<div class="mt-6 flex flex-col gap-5 lg:block">
		{#if devices?.length && onpickdevice}
			<DevicePicker class="max-lg:order-3" {devices} {deviceId} onpick={onpickdevice} />
		{/if}
		<div
			class={[
				'contents lg:grid lg:h-[38rem] lg:gap-5',
				devices?.length && onpickdevice && 'lg:mt-6',
				classroom.location
					? 'lg:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)_minmax(0,1fr)]'
					: 'lg:grid-cols-2'
			]}
		>
			<NowCard
				class="max-lg:order-4"
				tiles={now ?? station.now}
				{released}
				{accent}
				icon={devices?.find((d) => d.id === deviceId)?.icon}
				actions={nowActions}
				{onexplain}
				href={valueOf}
			/>
			{#if classroom.location}
				<section
					class="card relative h-80 overflow-hidden p-0 [view-transition-name:station-map] max-lg:order-1 sm:h-96 lg:h-full"
					aria-label="Where {shown.name} is"
				>
					<!-- Out of the flow, so the canvas never sets the card's height and grows it. -->
					<div class="absolute inset-0">
						<Globe
							{markers}
							bind:selectedId={globeSelected}
							ariaLabel="Globe of classrooms, turned to {shown.name}"
						/>
					</div>
					{#if legend.length}
						<ul
							class="pointer-events-none absolute top-4 left-4 flex flex-wrap gap-3 text-xs text-text"
							aria-label="Stages"
						>
							{#each legend as stage (stage.id)}
								<li class="flex items-center gap-1.5">
									<span class="size-2.5 rounded-full" style:background-color={stage.color}></span>
									{stage.label}
								</li>
							{/each}
						</ul>
					{/if}
					<span
						class="pointer-events-none absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-line-strong bg-surface px-3 py-1 text-xs whitespace-nowrap text-ink"
					>
						{pill}
					</span>
				</section>
			{/if}
			<InstitutionOverview
				overview={institution}
				actions={institutionActions}
				class="overflow-hidden max-lg:order-2"
			/>
		</div>

		<!-- A picked device's cards come from the Registry; a type with none set shows none. -->
		{#if summary}
			{#if summary.length}<DeviceStrip
					class="max-lg:order-5 lg:mt-5"
					cards={summary}
					href={valueOf}
				/>{/if}
		{:else}
			<CoreStrip class="max-lg:order-5 lg:mt-5" {station} />
		{/if}
	</div>

	{@render children?.()}
</div>
