<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import type { RawSortKey } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { describeParameter, formatValue } from '#lib/key-values.ts';
	import { PAGE_SIZE, UTF8_BOM } from '#lib/raw-data.ts';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const page = $derived(data.page);
	const query = $derived(data.query);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const count = new Intl.NumberFormat();

	const parameterOptions = $derived(
		(page?.options.parameterIds ?? [])
			.map((id) => ({ id, label: describeParameter(id, paramsById).label }))
			.sort((a, b) => a.label.localeCompare(b.label))
	);
	const filtered = $derived(
		!!query && !!(query.from || query.to || query.parameterId || query.sourceId || query.labId)
	);
	const pickedDevice = $derived(
		page?.options.sources.find((s) => s.id === query?.sourceId && s.kind === 'device')
	);
	const back = $derived(
		resolve('/(app)/dashboard') + (query ? sensorQuery(query.classroomId, pickedDevice?.id) : '')
	);

	type Filters = {
		from?: string;
		to?: string;
		param?: string;
		source?: string;
		site?: string;
		sort?: RawSortKey;
		order?: 'asc' | 'desc';
		n?: number;
	};

	/** The address for these filters: what is set stays, `change` replaces it. */
	function address(change: Filters) {
		const next: Filters = {
			from: query?.from,
			to: query?.to,
			param: query?.parameterId,
			source: query?.sourceId,
			site: query?.labId,
			sort: query?.sort,
			order: query?.order,
			...change
		};
		const pairs: [string, string][] = query ? [['classroom', query.classroomId]] : [];
		for (const [key, value] of Object.entries(next)) {
			if (value !== undefined && value !== '') pairs.push([key, String(value)]);
		}
		const params = pairs.map(([k, v]) => `${k}=${encodeURIComponent(v)}`).join('&');
		return resolve('/(app)/dashboard/readings') + `?${params}`;
	}

	function apply(change: Filters) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(address(change), { replace: true, reset: false });
	}

	/** Same column again flips the order; a new column starts newest or A to Z. */
	function sortBy(key: RawSortKey) {
		const current = query?.sort ?? 'time';
		const order = query?.order ?? (current === 'time' ? 'desc' : 'asc');
		apply(
			key === current
				? { sort: key, order: order === 'asc' ? 'desc' : 'asc' }
				: { sort: key, order: undefined }
		);
	}

	function ariaSort(key: RawSortKey) {
		const current = query?.sort ?? 'time';
		if (key !== current) return 'none';
		const order = query?.order ?? (current === 'time' ? 'desc' : 'asc');
		return order === 'asc' ? 'ascending' : 'descending';
	}

	let exporting = $state(false);
	let exportError = $state('');

	/** Asks SAM for the matching rows as CSV and saves the file. */
	async function exportCsv() {
		if (!query) return;
		exporting = true;
		exportError = '';
		try {
			const file = await services.exports.rawCsv(query);
			const url = URL.createObjectURL(
				new Blob(
					// Excel reads a CSV without the mark as a local code page and mangles `°C`.
					[file.content.startsWith(UTF8_BOM) ? '' : UTF8_BOM, file.content],
					{ type: 'text/csv;charset=utf-8' }
				)
			);
			const link = Object.assign(document.createElement('a'), {
				href: url,
				download: file.filename
			});
			link.click();
			setTimeout(() => URL.revokeObjectURL(url), 1000);
		} catch (e) {
			exportError = errorMessage(e);
		} finally {
			exporting = false;
		}
	}
</script>

{#snippet sortHeader(key: RawSortKey, label: string)}
	{@const state = ariaSort(key)}
	<th class="px-4 py-3 font-medium" aria-sort={state}>
		<button
			type="button"
			class={['inline-flex items-center gap-1 hover:text-ink', state !== 'none' && 'text-ink']}
			onclick={() => sortBy(key)}
		>
			{label}
			<span aria-hidden="true" class={state === 'none' ? 'opacity-30' : ''}
				>{state === 'ascending' ? '↑' : '↓'}</span
			>
		</button>
	</th>
{/snippet}

<svelte:head><title>Raw measurements · DaGrouper Analysis</title></svelte:head>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
<a href={back} class="text-sm text-muted hover:text-ink">← Dashboard</a>

<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-3xl sm:text-4xl">Raw measurements</h1>
		{#if page}
			<p class="mt-1 text-muted">
				Every value {page.classroomName} has sent, exactly as it arrived.
			</p>
		{/if}
	</div>
	{#if page}
		<div class="flex flex-col items-end gap-1">
			<button
				type="button"
				class="btn btn-primary"
				disabled={exporting || !page.total}
				onclick={exportCsv}
			>
				<Icon name="download" class="size-4" />
				{exporting ? 'Exporting…' : `Export CSV (${count.format(page.total)})`}
			</button>
			{#if exportError}<p class="field-error" role="alert">{exportError}</p>{/if}
		</div>
	{/if}
</div>

{#if !page || !query}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">Nothing to show</h2>
		<p class="mt-2 text-sm text-muted">
			Raw data comes only from classrooms you belong to. Join one with an invite link to see its
			measurements.
		</p>
	</div>
{:else}
	<section class="card mt-6" aria-labelledby="terms-title">
		<h2 id="terms-title" class="text-sm font-semibold">Terms of use</h2>
		<dl class="mt-2 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
			<dt class="text-muted">Licence</dt>
			<dd>
				{#if page.terms.licenceUrl}
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a class="underline" href={page.terms.licenceUrl} target="_blank" rel="noreferrer"
						>{page.terms.licence}</a
					>
				{:else}
					{page.terms.licence}
				{/if}
			</dd>
			<dt class="text-muted">Credit</dt>
			<dd>{page.terms.attribution}</dd>
			{#if page.terms.restrictions.length}
				<dt class="text-muted">Restrictions</dt>
				<dd>
					<ul class="list-disc pl-4">
						{#each page.terms.restrictions as restriction (restriction)}
							<li>{restriction}</li>
						{/each}
					</ul>
				</dd>
			{/if}
			{#if page.terms.coordinateDecimals !== null}
				<dt class="text-muted">Positions</dt>
				<dd>Rounded to {page.terms.coordinateDecimals} decimal places.</dd>
			{/if}
		</dl>
		<p class="mt-3 text-xs text-muted">
			Every exported row carries these terms. Keep them with the data when you share it.
		</p>
	</section>

	<form
		class="mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-[repeat(5,minmax(0,1fr))_auto]"
		aria-label="Filter raw measurements"
		onsubmit={(e) => e.preventDefault()}
	>
		<div>
			<label class="label" for="raw-from">From</label>
			<input
				id="raw-from"
				type="date"
				class="input"
				value={query.from ?? ''}
				max={query.to}
				onchange={(e) => apply({ from: e.currentTarget.value, n: undefined })}
			/>
		</div>
		<div>
			<label class="label" for="raw-to">To</label>
			<input
				id="raw-to"
				type="date"
				class="input"
				value={query.to ?? ''}
				min={query.from}
				onchange={(e) => apply({ to: e.currentTarget.value, n: undefined })}
			/>
		</div>
		<div>
			<label class="label" for="raw-param">Parameter</label>
			<select
				id="raw-param"
				class="input"
				value={query.parameterId ?? ''}
				onchange={(e) => apply({ param: e.currentTarget.value, n: undefined })}
			>
				<option value="">All parameters</option>
				{#each parameterOptions as option (option.id)}
					<option value={option.id}>{option.label}</option>
				{/each}
			</select>
		</div>
		<div>
			<label class="label" for="raw-source">Source</label>
			<select
				id="raw-source"
				class="input"
				value={query.sourceId ?? ''}
				onchange={(e) => apply({ source: e.currentTarget.value, n: undefined })}
			>
				<option value="">All sources</option>
				{#each page.options.sources as option (option.id)}
					<option value={option.id}>{option.name}</option>
				{/each}
			</select>
		</div>
		<div>
			<label class="label" for="raw-site">Site</label>
			<select
				id="raw-site"
				class="input"
				value={query.labId ?? ''}
				onchange={(e) => apply({ site: e.currentTarget.value, n: undefined })}
			>
				<option value="">All sites</option>
				{#each page.options.sites as option (option.id)}
					<option value={option.id}>{option.name}</option>
				{/each}
			</select>
		</div>
		<div class="flex items-end">
			<button
				type="button"
				class="btn btn-ghost w-full"
				disabled={!filtered}
				onclick={() =>
					apply({
						from: undefined,
						to: undefined,
						param: undefined,
						source: undefined,
						site: undefined,
						n: undefined
					})}>Clear filters</button
			>
		</div>
	</form>

	{#if !page.total}
		<div class="card mt-6">
			<h2 class="text-xl font-semibold">
				{filtered ? 'No values match these filters' : 'No measurements yet'}
			</h2>
			<p class="mt-2 text-sm text-muted">
				{filtered
					? 'Widen the dates or clear a filter.'
					: 'Values show here once a measurement sends data or someone enters an observation.'}
			</p>
		</div>
	{:else}
		<p class="mt-6 text-sm text-muted" aria-live="polite">
			{page.rows.length < page.total
				? `Showing ${count.format(page.rows.length)} of ${count.format(page.total)} values.`
				: `${count.format(page.total)} value${page.total === 1 ? '' : 's'}.`}
			Dates are in your time zone; the date filters count whole days in UTC.
		</p>
		<div class="card mt-3 overflow-x-auto p-0">
			<table class="w-full min-w-[52rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						{@render sortHeader('time', 'Observed')}
						{@render sortHeader('parameter', 'Parameter')}
						<th class="px-4 py-3 text-right font-medium">Value</th>
						{@render sortHeader('source', 'Source')}
						{@render sortHeader('site', 'Site')}
						<th class="px-4 py-3 font-medium">Location</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each page.rows as row (row.id)}
						{@const param = describeParameter(row.parameterId, paramsById)}
						<tr class="align-top">
							<td class="px-4 py-2.5 whitespace-nowrap">
								{dateTime.format(new Date(row.observedAt))}
								{#if row.creditedTo}
									<p class="text-xs text-muted">Recorded by {row.creditedTo}</p>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{param.label}
								<p class="font-mono text-xs text-muted">{row.parameterId}</p>
							</td>
							<td class="px-4 py-2.5 text-right font-mono tabular-nums">
								{row.value}{param.unit ? ` ${param.unit}` : ''}
								{#if param.valueType === 'time'}
									<span class="block font-sans text-xs text-muted">
										{formatValue(row.value, param.unit, param.valueType)}
									</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{row.sourceName}
								{#if row.sourceKind === 'field'}
									<p class="text-xs text-muted">Entered by hand, no device</p>
								{/if}
							</td>
							<td class="px-4 py-2.5">{row.siteName}</td>
							<td class="px-4 py-2.5 text-xs whitespace-nowrap">
								{formatPoint(row.location)}
								<p class="text-muted">
									{row.locationSource === 'device' ? 'Device GPS' : 'Lab location'}
								</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if page.rows.length < page.total}
			<div class="mt-4 text-center">
				<button
					type="button"
					class="btn btn-ghost"
					onclick={() => apply({ n: data.limit + PAGE_SIZE })}>Show more</button
				>
			</div>
		{/if}
	{/if}
{/if}
