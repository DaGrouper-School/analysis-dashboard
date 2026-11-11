<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import type { Parameter, RawPage, RawQuery } from '#lib/domain/types.ts';
	import RawFilters from '#lib/features/measurements/RawFilters.svelte';
	import { UTF8_BOM, type RawFilterChange } from '#lib/features/measurements/raw-data.ts';
	import { sourceLooks, type SourceLooks } from '#lib/features/measurements/source-looks.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * "Export CSV" (AP-14): opens a dialog with the raw data filters, set to what the page shows,
	 * and saves the matching values from SAM's export service, with the classroom's terms on
	 * every row. The same on the raw data page and in a value page's header.
	 */
	let {
		query,
		classrooms,
		class: className = ''
	}: {
		/** The filters the dialog starts from. */
		query: RawQuery;
		/** The classrooms the user belongs to. */
		classrooms: { id: string; name: string }[];
		class?: string;
	} = $props();

	const number = new Intl.NumberFormat();
	let open = $state(false);
	let filters = $state<RawQuery>({ classroomId: '' });
	let found = $state<RawPage | null>(null);
	let looks = $state<SourceLooks>({});
	let parameters = $state<ReadonlyMap<string, Parameter>>(new Map());
	let loadError = $state('');
	let exporting = $state(false);
	let exportError = $state('');

	function start() {
		filters = { ...query };
		found = null;
		exportError = '';
		open = true;
		if (!parameters.size)
			services.registry
				.listParameters()
				.then((list) => (parameters = new Map(list.map((p) => [p.id, p]))));
	}

	// What the filters match, and what each of them can offer, as they change.
	$effect(() => {
		if (!open || !filters.classroomId) return;
		const asked = { ...filters };
		loadError = '';
		Promise.all([
			services.exports.rawValues(asked, { page: 1, pageSize: 1 }),
			services.labs.listClassroomDevices(asked.classroomId),
			services.registry.listDeviceTypes()
		])
			.then(([page, devices, types]) => {
				if (JSON.stringify(asked) !== JSON.stringify(filters)) return;
				found = page;
				looks = sourceLooks(devices, types);
			})
			.catch((e) => (loadError = errorMessage(e)));
	});

	function change(next: RawFilterChange) {
		filters = { ...filters, ...next };
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
			open = false;
		} catch (e) {
			exportError = errorMessage(e);
		} finally {
			exporting = false;
		}
	}
</script>

<button type="button" class={['btn btn-primary', className]} aria-haspopup="dialog" onclick={start}>
	<Icon name="download" class="size-4" /> Export CSV
</button>

<Modal bind:open title="Export CSV" wide>
	<div class="grid gap-3 sm:grid-cols-2">
		{#if found}
			<RawFilters
				query={filters}
				{classrooms}
				options={found.options}
				{looks}
				{parameters}
				idPrefix="export"
				onchange={change}
			/>
		{/if}
	</div>
	<p class="mt-4 text-sm text-muted" aria-live="polite">
		{#if loadError}
			<span class="field-error" role="alert">{loadError}</span>
		{:else if found}
			{found.total
				? `${number.format(found.total)} value${found.total === 1 ? '' : 's'} from ${found.classroomName}. Every row carries the classroom's terms of use (${found.terms.licence}).`
				: 'No values match these filters. Widen the dates or pick another measurement.'}
		{:else}
			Loading…
		{/if}
	</p>
	{#if exportError}<p class="field-error mt-2" role="alert">{exportError}</p>{/if}
	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
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
					? `Export ${number.format(found.total)} values`
					: 'Export'}
		</button>
	{/snippet}
</Modal>
