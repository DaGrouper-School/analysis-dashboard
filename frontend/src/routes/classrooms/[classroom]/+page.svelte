<script lang="ts">
	import { goto, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import ClassroomFilter from '#lib/features/classroom/ClassroomFilter.svelte';
	import ClassroomOverview from '#lib/views/ClassroomOverview.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import {
		filterClassrooms,
		filterQuery,
		type DirectoryFilter
	} from '#lib/features/classroom/directory.ts';
	import { pickDevice, placeLabel } from '#lib/features/devices/station-now.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));
	/** A classroom's page, keeping the filter. */
	function hrefOf(id: string, filter: DirectoryFilter = data.filter) {
		return resolve('/classrooms/[classroom]', { classroom: id }) + filterQuery(filter);
	}
	function pick(id: string) {
		// Another classroom keeps the reader where they are, as on the dashboard.
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(hrefOf(id), { reset: false });
	}

	// ---- The filter: it narrows the classroom tabs and the globe -------------------------
	const sorted = $derived([...data.places].sort((a, b) => a.name.localeCompare(b.name)));
	const kept = $derived(filterClassrooms(sorted, data.filter));
	/** A new filter stays on this classroom while it is kept, else opens the first one kept. */
	function refilter(filter: DirectoryFilter) {
		const matches = filterClassrooms(sorted, filter);
		const next = matches.some((p) => p.id === classroom.id) ? classroom.id : matches[0]?.id;
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(hrefOf(next ?? classroom.id, filter), { replace: true, reset: false });
	}

	// ---- Devices: the one picked sets the tiles, the cards and the page's colour -------
	let deviceId = $state(page.url.searchParams.get('device'));
	const device = $derived(pickDevice(data.devices, deviceId));
	function pickDeviceShown(id: string) {
		deviceId = id;
		const url = new URL(page.url.href);
		url.searchParams.set('device', id);
		// The current page's own URL, with only the device changed.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		replaceState(url, page.state);
	}

	// Every classroom the filter keeps as a tab, and this one while a new filter loads.
	const tabs = $derived(
		(kept.some((p) => p.id === classroom.id)
			? kept
			: [...kept, ...sorted.filter((p) => p.id === classroom.id)]
		).map((p) => ({
			id: p.id,
			label: placeLabel(p.placeName, p.country) || p.name,
			title: p.name,
			href: hrefOf(p.id)
		}))
	);
	// The classroom being opened: the shared overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		(navigating.to?.route.id === '/classrooms/[classroom]' && navigating.to.params?.classroom) ||
			classroom.id
	);
</script>

<svelte:head><title>{classroom.name} · {data.content.hero.eyebrow}</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<ClassroomOverview
		station={data}
		{tabs}
		tabsLabel="Classrooms"
		{pendingId}
		places={kept}
		onpick={pick}
		devices={data.devices}
		deviceId={device?.id}
		onpickdevice={pickDeviceShown}
		now={device?.tiles}
		summary={device?.summary}
		accent={device?.color}
	>
		{#snippet eyebrow(shown)}
			<div class="flex flex-wrap items-center justify-between gap-3">
				<Breadcrumbs
					items={[
						{ label: directory?.label ?? 'Classrooms', href: resolve('/classrooms') },
						...(shown.country ? [{ label: shown.country }] : []),
						{ label: shown.name }
					]}
				/>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
		{/snippet}
		{#snippet headerActions()}
			<a class="btn btn-ghost" href={resolve('/host')}>{data.content.hosting.title}</a>
		{/snippet}
		{#snippet tabsStart()}
			<ClassroomFilter
				class="mb-4 justify-center"
				places={data.places}
				content={data.content}
				filter={data.filter}
				onchange={refilter}
			/>
			{#if !kept.length}
				<p class="mb-4 text-center text-sm text-muted" aria-live="polite">
					No classroom matches these filters.
				</p>
			{/if}
		{/snippet}
	</ClassroomOverview>
</PublicShell>
