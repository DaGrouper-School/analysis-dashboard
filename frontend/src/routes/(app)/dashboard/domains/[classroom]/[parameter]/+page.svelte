<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import { thresholdStatus, thresholdsFor } from '#lib/domain/alerts.ts';
	import { chartOf } from '#lib/domain/charts.ts';
	import type { PublicParameterPage } from '#lib/domain/types.ts';
	import RawDataButton from '#lib/features/measurements/RawDataButton.svelte';
	import ChartAlerts from '#lib/features/alerts/ChartAlerts.svelte';
	import {
		dailyStats,
		dayOf,
		daysOf,
		latestOf,
		rainDays,
		rainNow,
		RANGES,
		type RangeId
	} from '#lib/features/measurements/parameter-page.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import { deviceTypeLabel } from '#lib/utils/labels.ts';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';
	import { classroomsHref, domainsHref } from '#lib/utils/site.ts';
	import DailySummary from './DailySummary.svelte';
	import DataQuality from './DataQuality.svelte';
	import HowMeasured from './HowMeasured.svelte';
	import ValueChart from './ValueChart.svelte';
	import ValueNow from './ValueNow.svelte';
	import { dateTime, dayText, showValue, sourceName } from './value-format.ts';
	import type { PageProps } from './$types';

	/**
	 * One value of one classroom (AP-23), always in the same order: what it means here, the value
	 * now, the main chart (compare in its header), the daily summary (download in its header), data
	 * quality, and how it is measured (the curriculum link in its header). For the classroom's
	 * members, so values still in review show too. This page loads and shapes the data; each
	 * section is its own component beside it.
	 */
	let { data }: PageProps = $props();

	const parameter = $derived(data.parameter);
	const chart = $derived(chartOf(parameter));
	const show = (value: number | string | null | undefined) => showValue(value, parameter);

	let range = $state<RangeId>('7d');
	/** Values other than rain show one source at a time; rain shows every source together. */
	// svelte-ignore state_referenced_locally
	let sourceId = $state<string | null>(data.device);
	const series = $derived(
		data.series.find((s) => s.source.id === sourceId) ?? data.series[0] ?? null
	);
	const latest = $derived(latestOf(data.series));
	const rangeDays = $derived(
		latest ? daysOf(dayOf(latest), RANGES.find((r) => r.id === range)!.days) : []
	);

	// Rain: every source by day, the gauge as reference. Other numbers: one source, by day.
	const rain = $derived(
		data.rain && chart === 'daily_bars'
			? rainDays(data.series, data.rain.referenceSourceId, data.rain.tolerance)
			: null
	);
	const rainInRange = $derived(
		rain?.filter((d) => d.day >= rangeDays[0] && d.day <= rangeDays.at(-1)!) ?? []
	);
	const stats = $derived(series ? dailyStats(series.points) : []);
	/** The shown device's registry colour tints the page, so the value shows its sensor. */
	const accent = $derived(rain ? null : (series?.source.color ?? null));
	const statsInRange = $derived(stats.filter((d) => d.day >= (rangeDays[0] ?? '')));

	// Alert levels: members of the classroom only, for the device shown.
	const alerts = $derived(
		data.alerts.find((a) => a.deviceId === (rain ? data.device : series?.source.id)) ??
			data.alerts.find((a) => thresholdsFor(a.rules, parameter.id).length) ??
			null
	);
	const levels = $derived(alerts ? thresholdsFor(alerts.rules, parameter.id) : []);
	const alertNow = $derived(
		alerts
			? thresholdStatus(
					alerts.rules,
					parameter.id,
					data.series.find((s) => s.source.id === alerts.deviceId)?.points.at(-1)?.value ??
						undefined
				)
			: null
	);

	// Compare with another classroom.
	let compareId = $state('');
	let compared = $state<PublicParameterPage | null>(null);
	let compareError = $state('');
	$effect(() => {
		const id = compareId;
		compared = null;
		compareError = '';
		if (!id) return;
		services.publicSite
			.parameter(id, parameter.id)
			.then((page) => {
				if (compareId === id) compared = page;
			})
			.catch((e) => (compareError = errorMessage(e)));
	});

	const now = $derived.by(() => {
		if (rain) {
			const day = rainNow(rain, data.series, parameter.id);
			if (!day) return null;
			const from = data.series.find((s) => s.source.id === day.sourceId);
			return {
				value: show(day.value),
				caption: `Rain on ${dayText(day.day)}${from ? `, from ${sourceName(from)}` : ''}`,
				review: day.review
			};
		}
		const point = series?.points.at(-1);
		if (!point || !series) return null;
		const today = stats.at(-1);
		return {
			value: show(point.value),
			caption: `${sourceName(series)}, ${dateTime.format(new Date(point.observedAt))}`,
			range:
				chart === 'line_band' && today && today.day === dayOf(point.observedAt) && today.count > 1
					? `Today ${show(today.low)} to ${show(today.high)}`
					: null,
			review: false
		};
	});

	const quality = $derived.by(() => {
		const days30 = latest ? daysOf(dayOf(latest), 30) : [];
		const withData = new Set(
			(rain ? rain.map((d) => d.day) : stats.map((d) => d.day)).filter((d) => days30.includes(d))
		);
		return {
			received: withData.size,
			expected: days30.length,
			flagged: rain?.filter((d) => d.review && days30.includes(d.day)).length ?? 0
		};
	});
</script>

<svelte:head
	><title>{parameter.label} · {data.classroom.name} · {data.content.hero.eyebrow}</title
	></svelte:head
>

<!-- Back up: the dashboard, the classroom's page, and the value's domain on the device shown. -->
<Breadcrumbs
	items={[
		{ label: 'Dashboard', href: resolve('/(app)/dashboard') + sensorQuery(data.classroom.id) },
		{ label: data.classroom.name, href: classroomsHref(data.classroom.id) },
		{
			label: data.domain?.label ?? 'Domains',
			href: domainsHref(
				data.classroom.id,
				!rain && series?.source.kind === 'device' ? series.source.id : null,
				data.domain?.id
			)
		},
		{ label: parameter.label }
	]}
/>

<div class={['contents', accent && 'accent-scope']} style:--accent={accent}>
	<header class="mt-4 flex flex-wrap items-start justify-between gap-4">
		<div>
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="font-display text-4xl">{parameter.label}</h1>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
			<p class="mt-2 text-sm text-muted">Parameter details</p>
		</div>
		<!-- Every raw value of this measurement, from the device shown (rain: every source). -->
		<RawDataButton
			primary
			label="Export CSV"
			query={{
				classroomId: data.classroom.id,
				parameterId: parameter.id,
				sourceId: !rain && series?.source.kind === 'device' ? series.source.id : undefined
			}}
		/>
	</header>

	<!-- 1. What it means here -->
	<section class="mt-6 max-w-2xl" aria-labelledby="meaning-heading">
		<h2 id="meaning-heading" class="text-sm font-medium text-muted">What it means here</h2>
		<p class="mt-1 text-lg">{data.note ?? parameter.plain ?? parameter.definition}</p>
		{#if data.note}
			<p class="mt-2 text-sm text-muted">
				Written by {data.classroom.name}. {parameter.plain ?? parameter.definition}
			</p>
		{/if}
	</section>

	<!-- 2. Value now -->
	{#if now}<ValueNow {now} {alertNow} />{/if}

	<!-- 3. Main chart, with compare -->
	<ValueChart
		page={data}
		{parameter}
		{chart}
		{latest}
		{rangeDays}
		{rain}
		{rainInRange}
		{series}
		{stats}
		{compared}
		{compareError}
		{levels}
		bind:range
		bind:compareId
		bind:sourceId
	>
		{#snippet levelsEditor()}
			{#if alerts && parameter.valueType !== 'text' && parameter.valueType !== 'time'}
				<ChartAlerts
					{alerts}
					{parameter}
					typeName={deviceTypeLabel(data.typeDefinitions, alerts.deviceType)}
					onchange={() => invalidateAll()}
				/>
			{/if}
		{/snippet}
	</ValueChart>

	<!-- 4. Daily summary, with download -->
	<DailySummary
		page={data}
		{parameter}
		{chart}
		rainInRange={rain ? rainInRange : null}
		{statsInRange}
	/>

	<!-- 5. Data quality -->
	<DataQuality
		{quality}
		releasedThrough={data.releasedThrough}
		inReview={data.inReview}
		rain={!!rain}
		sources={data.series.map(sourceName).join(', ')}
	/>

	<!-- 6. How it is measured, with the curriculum link -->
	<HowMeasured
		{parameter}
		learn={data.content.sections.find((s) => s.id === 'learn')}
		rainSeries={rain ? data.series : null}
		referenceSourceId={data.rain?.referenceSourceId ?? null}
		deviceTypes={data.deviceTypes}
	/>
</div>
