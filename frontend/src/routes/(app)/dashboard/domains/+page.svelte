<script lang="ts">
	import { goto } from '$app/navigation';
	import { navigating } from '$app/state';
	import { pickDevice } from '#lib/features/devices/station-now.ts';
	import DevicePicker from '#lib/features/devices/DevicePicker.svelte';
	import DomainBrowser from '#lib/features/measurements/DomainBrowser.svelte';
	import { domainsOf } from '#lib/features/measurements/domains.ts';
	import RawDataButton from '#lib/features/measurements/RawDataButton.svelte';
	import { givenName } from '#lib/utils/format.ts';
	import { domainsHref } from '#lib/utils/site.ts';
	import ClassroomPageHeader from '../ClassroomPageHeader.svelte';
	import NoClassroom from '../NoClassroom.svelte';
	import type { PageProps } from './$types';

	/**
	 * Domains (members only): one of the user's classrooms and one of its devices, picked from
	 * dropdowns, and what that device measures, domain by domain. Each value opens its own page.
	 */
	let { data }: PageProps = $props();

	const station = $derived(data.station);
	const shownDevice = $derived(station ? pickDevice(station.devices, data.device?.id) : null);
	const domains = $derived(domainsOf(station?.domains ?? [], shownDevice?.id));
	const classroomOptions = $derived(data.classrooms.map((c) => ({ value: c.id, label: c.name })));
	// The classroom being opened: the header moves to it at once and the page shows a skeleton.
	const pendingId = $derived(
		(navigating.to?.route.id === '/(app)/dashboard/domains' &&
			navigating.to.url.searchParams.get('classroom')) ||
			station?.classroom.id ||
			''
	);
	const loading = $derived(!!station && pendingId !== station.classroom.id);

	/** Moves to another classroom or device, staying on this page. */
	function choose(classroomId: string, deviceId?: string) {
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(domainsHref(classroomId, deviceId), { replace: true, reset: false });
	}
</script>

<svelte:head><title>Domains · DaGrouper Analysis</title></svelte:head>

{#if !station}
	<NoClassroom firstName={givenName(data.user.fullName)} />
{:else}
	<ClassroomPageHeader
		title="Domains"
		sub="What each of {station.classroom
			.name}'s devices measures, domain by domain. Open a value for its chart, daily summary and data quality."
		{loading}
		{classroomOptions}
		classroomId={pendingId}
		onclassroom={(id) => choose(id)}
	/>
	<section class={['mt-8', loading && 'is-loading']} aria-label="Domains">
		<!-- The picked device's raw data beside the picker, as on its card in Measurements. -->
		<div class="flex flex-wrap items-center justify-between gap-3 lg:flex-nowrap">
			<DevicePicker
				devices={station.devices}
				deviceId={shownDevice?.id}
				onpick={(id) => choose(station.classroom.id, id)}
			/>
			{#if shownDevice}<RawDataButton
					class="shrink-0"
					query={{ classroomId: station.classroom.id, sourceId: shownDevice.id }}
				/>{/if}
		</div>
		{#key shownDevice?.id}
			<DomainBrowser
				class="mt-6"
				{domains}
				parameters={station.parameters}
				classroomId={station.classroom.id}
			/>
		{/key}
	</section>
{/if}
