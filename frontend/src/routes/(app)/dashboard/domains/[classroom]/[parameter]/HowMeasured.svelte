<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { Parameter, PublicParameterSeries, SiteSection } from '#lib/domain/types.ts';
	import { sectionHref } from '#lib/utils/site.ts';
	import { sourceName } from './value-format.ts';

	/** How it is measured (AP-23): who takes it, the method, the unit, and where to learn more. */
	let {
		parameter,
		learn,
		rainSeries,
		referenceSourceId,
		deviceTypes
	}: {
		parameter: Parameter;
		learn: SiteSection | undefined;
		/** For rain: every source, the reference gauge read by hand. */
		rainSeries: PublicParameterSeries[] | null;
		referenceSourceId: string | null;
		deviceTypes: string[];
	} = $props();

	const METHOD = {
		measured: 'Measured',
		derived: 'Calculated from other values',
		manual: 'Entered by hand'
	};
	const SOURCE = {
		weather_station: 'A weather station',
		device: 'A sensor',
		manual: 'People, on a guided form'
	};
</script>

<section class="mt-8" aria-labelledby="measured-heading">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 id="measured-heading" class="font-display text-2xl">How it is measured</h2>
		{#if learn}
			<!-- eslint-disable svelte/no-navigation-without-resolve -- sectionHref resolves -->
			<a class="link inline-flex items-center gap-1.5 text-sm" href={sectionHref(learn)}
				>Learn about it <Icon name="arrow" class="size-4" /></a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/if}
	</div>
	<dl class="mt-3 max-w-2xl space-y-2 text-sm">
		<div class="flex gap-2">
			<dt class="w-32 shrink-0 text-muted">Taken by</dt>
			<dd>
				{#if rainSeries}
					{rainSeries
						.map((s) =>
							referenceSourceId === s.source.id ? `${sourceName(s)}, read by hand` : sourceName(s)
						)
						.join('; ')}
				{:else}
					{SOURCE[parameter.source]}{deviceTypes.length ? ` (${deviceTypes.join(', ')})` : ''}
				{/if}
			</dd>
		</div>
		{#if parameter.calculation}
			<div class="flex gap-2">
				<dt class="w-32 shrink-0 text-muted">Method</dt>
				<dd>{METHOD[parameter.calculation.method]}: {parameter.calculation.formula}</dd>
			</div>
			<div class="flex gap-2">
				<dt class="w-32 shrink-0 text-muted">Registry</dt>
				<dd>SAM registry {parameter.calculation.version}</dd>
			</div>
		{/if}
		<div class="flex gap-2">
			<dt class="w-32 shrink-0 text-muted">Unit</dt>
			<dd>
				{parameter.unit || 'None'}{parameter.min !== undefined && parameter.max !== undefined
					? `, accepted from ${parameter.min} to ${parameter.max}`
					: ''}
			</dd>
		</div>
	</dl>
</section>
