<script lang="ts">
	import Modal from '#lib/components/Modal.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Pagination from '#lib/components/Pagination.svelte';
	import type {
		DomainDefinition,
		Parameter,
		RawPage,
		RawQuery,
		RawSortKey
	} from '#lib/domain/types.ts';
	import ContributorId from '#lib/features/consent/ContributorId.svelte';
	import { formatPoint } from '#lib/features/map/geo.ts';
	import { describeParameter, formatValue } from '#lib/features/measurements/key-values.ts';
	import RawFilters from '#lib/features/measurements/RawFilters.svelte';
	import {
		PAGE_SIZE,
		UTF8_BOM,
		type RawFilterChange
	} from '#lib/features/measurements/raw-data.ts';
	import { sourceLooks, type SourceLooks } from '#lib/features/measurements/source-looks.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * Raw measurements (AP-14) in a dialog: every value a classroom has sent, exactly as it
	 * arrived, filtered, sorted and paged by SAM's export service. "Export CSV" saves exactly what
	 * the filters show, with the classroom's terms on every row. Opened with the filters set,
	 * such as a device's or a value's.
	 */
	let {
		open = $bindable(false),
		query
	}: {
		open?: boolean;
		/** The filters it opens with. */
		query: RawQuery;
	} = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const number = new Intl.NumberFormat();

	let filters = $state<RawQuery>({ classroomId: '' });
	let pageNo = $state(1);
	let found = $state<RawPage | null>(null);
	let loading = $state(false);
	let loadError = $state('');
	let classrooms = $state<{ id: string; name: string }[]>([]);
	let parameters = $state<ReadonlyMap<string, Parameter>>(new Map());
	let domains = $state<DomainDefinition[]>([]);
	let looks = $state<SourceLooks>({});
	let exporting = $state(false);
	let exportError = $state('');

	// Each time it opens: the filters it was given, from the first page.
	$effect(() => {
		if (!open) return;
		filters = { ...query };
		pageNo = 1;
		exportError = '';
	});

	// The registry and the user's classrooms, once.
	$effect(() => {
		if (!open || parameters.size) return;
		Promise.all([
			services.registry.listParameters(),
			services.registry.listDomains(),
			services.classrooms.listMine()
		]).then(([list, registryDomains, mine]) => {
			parameters = new Map(list.map((p) => [p.id, p]));
			domains = registryDomains;
			classrooms = mine.filter((c) => c.myRole !== null);
		});
	});

	// The page of values the filters match, and what each filter can offer.
	$effect(() => {
		if (!open || !filters.classroomId) return;
		const asked = { ...filters };
		const paging = { page: pageNo, pageSize: PAGE_SIZE };
		loading = true;
		loadError = '';
		Promise.all([
			services.exports.rawValues(asked, paging),
			services.labs.listClassroomDevices(asked.classroomId),
			services.registry.listDeviceTypes()
		])
			.then(([page, devices, types]) => {
				if (JSON.stringify(asked) !== JSON.stringify(filters) || paging.page !== pageNo) return;
				found = page;
				looks = sourceLooks(devices, types);
			})
			.catch((e) => (loadError = errorMessage(e)))
			.finally(() => (loading = false));
	});

	const filtered = $derived(
		!!(filters.from || filters.to || filters.parameterId || filters.sourceId)
	);

	function change(next: RawFilterChange) {
		filters = { ...filters, ...next, ...('classroomId' in next && { labId: undefined }) };
		pageNo = 1;
	}

	/** Same column again flips the order; a new column starts newest or A to Z. */
	function sortBy(key: RawSortKey) {
		const current = filters.sort ?? 'time';
		const order = filters.order ?? (current === 'time' ? 'desc' : 'asc');
		filters =
			key === current
				? { ...filters, sort: key, order: order === 'asc' ? 'desc' : 'asc' }
				: { ...filters, sort: key, order: undefined };
		pageNo = 1;
	}

	function ariaSort(key: RawSortKey) {
		const current = filters.sort ?? 'time';
		if (key !== current) return 'none';
		const order = filters.order ?? (current === 'time' ? 'desc' : 'asc');
		return order === 'asc' ? 'ascending' : 'descending';
	}

	async function exportCsv() {
		exporting = true;
		exportError = '';
		try {
			const file = await services.exports.rawCsv(filters);
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

<Modal bind:open title="Raw measurements" wide>
	<p class="text-sm text-muted">
		Every value {found?.classroomName ?? 'the classroom'} has sent, exactly as it arrived. Export CSV
		saves what the filters show.
	</p>

	<form
		class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
		aria-label="Filter raw measurements"
		onsubmit={(e) => e.preventDefault()}
	>
		{#if found}
			<RawFilters
				query={filters}
				{classrooms}
				options={found.options}
				{looks}
				{parameters}
				{domains}
				onchange={change}
			/>
			<div class="flex items-end">
				<button
					type="button"
					class="btn btn-ghost w-full"
					disabled={!filtered}
					onclick={() =>
						change({
							sourceId: undefined,
							parameterId: undefined,
							from: undefined,
							to: undefined
						})}><Icon name="close" class="size-4" /> Clear filters</button
				>
			</div>
		{/if}
	</form>

	{#if loadError}
		<p class="field-error mt-4" role="alert">{loadError}</p>
	{:else if !found}
		<p class="mt-4 text-sm text-muted">Loading…</p>
	{:else if !found.total}
		<div class="mt-4 rounded-md border border-dashed border-line-strong p-6 text-center">
			<p class="font-medium text-ink">
				{filtered ? 'No values match these filters' : 'No measurements yet'}
			</p>
			<p class="mt-1 text-sm text-muted">
				{filtered
					? 'Widen the dates or clear a filter.'
					: 'Values show here once a measurement sends data or someone enters an observation.'}
			</p>
		</div>
	{:else}
		<p class="mt-4 text-sm text-muted" aria-live="polite">
			{number.format(found.total)} value{found.total === 1 ? '' : 's'}. Dates are in your time zone;
			the date filters count whole days in UTC.
		</p>
		<div class={['mt-3 overflow-x-auto rounded-md border border-line', loading && 'opacity-60']}>
			<table class="w-full min-w-[52rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						{@render sortHeader('time', 'Observed')}
						{@render sortHeader('parameter', 'Parameter')}
						<th class="px-4 py-3 text-right font-medium">Value</th>
						{@render sortHeader('source', 'Measurement')}
						{@render sortHeader('site', 'Site')}
						<th class="px-4 py-3 font-medium">Location</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each found.rows as row (row.id)}
						{@const param = describeParameter(row.parameterId, parameters)}
						<tr class="align-top">
							<td class="px-4 py-2.5 whitespace-nowrap">
								{dateTime.format(new Date(row.observedAt))}
								{#if row.contributorId !== null}
									<p class="text-xs text-muted">
										Recorded by <ContributorId participantId={row.contributorId} />
									</p>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{param.label}
								<p class="font-mono text-xs text-muted">{row.parameterId}</p>
							</td>
							<td class="px-4 py-2.5 text-right font-mono whitespace-nowrap tabular-nums">
								{row.value}{param.unit ? ` ${param.unit}` : ''}
								{#if param.valueType === 'time'}
									<span class="block font-sans text-xs text-muted">
										{formatValue(row.value, param.unit, param.valueType)}
									</span>
								{/if}
							</td>
							<td class="min-w-40 px-4 py-2.5">
								{row.sourceName}
								{#if row.sourceKind === 'field'}
									<p class="text-xs text-muted">Entered by hand, no device</p>
								{/if}
							</td>
							<td class="px-4 py-2.5 whitespace-nowrap">{row.siteName}</td>
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
		<Pagination
			class="mt-3"
			page={found.page}
			pageSize={found.pageSize}
			total={found.total}
			onpage={(p) => (pageNo = p)}
		/>
		<p class="mt-4 text-xs text-muted">
			Licence:
			{#if found.terms.licenceUrl}
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a class="underline" href={found.terms.licenceUrl} target="_blank" rel="noreferrer"
					>{found.terms.licence}</a
				>{:else}{found.terms.licence}{/if}. Credit: {found.terms.attribution}{found.terms
				.restrictions.length
				? `. ${found.terms.restrictions.join(' ')}`
				: ''}{found.terms.coordinateDecimals !== null
				? `. Positions rounded to ${found.terms.coordinateDecimals} decimal places`
				: ''}. Every exported row carries these terms.
		</p>
	{/if}
	{#if exportError}<p class="field-error mt-2" role="alert">{exportError}</p>{/if}

	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}
			><Icon name="close" class="size-4" /> Close</button
		>
		<button
			type="button"
			class="btn btn-primary"
			disabled={exporting || !found?.total}
			onclick={exportCsv}
		>
			<Icon name="download" class="size-4" />
			{exporting
				? 'Exporting…'
				: found?.total
					? `Export CSV (${number.format(found.total)})`
					: 'Export CSV'}
		</button>
	{/snippet}
</Modal>
