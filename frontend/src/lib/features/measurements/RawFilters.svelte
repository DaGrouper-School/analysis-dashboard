<script lang="ts">
	import Select from '#lib/components/Select.svelte';
	import type { Parameter, RawPage, RawQuery } from '#lib/domain/types.ts';
	import { describeParameter } from '#lib/features/measurements/key-values.ts';
	import type { RawFilterChange } from '#lib/features/measurements/raw-data.ts';
	import type { SourceLooks } from '#lib/features/measurements/source-looks.ts';

	/**
	 * The raw data filters (AP-14), in order: classroom, then measurement, then parameter, then
	 * dates. Each narrows the next, so picking one clears the ones after it. On the raw data page
	 * and in the CSV export.
	 */
	let {
		query,
		classrooms,
		options,
		looks,
		parameters,
		idPrefix = 'raw',
		onchange
	}: {
		query: RawQuery;
		/** The classrooms the user belongs to. */
		classrooms: { id: string; name: string }[];
		options: RawPage['options'];
		looks: SourceLooks;
		parameters: ReadonlyMap<string, Parameter>;
		/** Keeps the field IDs apart when the page and a dialog both show the filters. */
		idPrefix?: string;
		onchange: (change: RawFilterChange) => void;
	} = $props();

	const parameterOptions = $derived(
		options.parameterIds
			.map((id) => ({ value: id, label: describeParameter(id, parameters).label }))
			.sort((a, b) => a.label.localeCompare(b.label))
	);
</script>

<div>
	<label class="label" for="{idPrefix}-classroom">Classroom</label>
	<Select
		id="{idPrefix}-classroom"
		options={classrooms.map((c) => ({ value: c.id, label: c.name }))}
		value={query.classroomId}
		onchange={(value) =>
			onchange({ classroomId: value, sourceId: undefined, parameterId: undefined })}
	/>
</div>
<div>
	<label class="label" for="{idPrefix}-source">Measurement</label>
	<Select
		id="{idPrefix}-source"
		options={[
			{ value: '', label: 'All measurements' },
			...options.sources.map((o) => ({ value: o.id, label: o.name, ...looks[o.id] }))
		]}
		value={query.sourceId ?? ''}
		onchange={(value) => onchange({ sourceId: value || undefined, parameterId: undefined })}
	/>
</div>
<div>
	<label class="label" for="{idPrefix}-param">Parameter</label>
	<Select
		id="{idPrefix}-param"
		options={[{ value: '', label: 'All parameters' }, ...parameterOptions]}
		value={query.parameterId ?? ''}
		onchange={(value) => onchange({ parameterId: value || undefined })}
	/>
</div>
<div>
	<label class="label" for="{idPrefix}-from">From</label>
	<input
		id="{idPrefix}-from"
		type="date"
		class="input"
		value={query.from ?? ''}
		max={query.to}
		onchange={(e) => onchange({ from: e.currentTarget.value || undefined })}
	/>
</div>
<div>
	<label class="label" for="{idPrefix}-to">To</label>
	<input
		id="{idPrefix}-to"
		type="date"
		class="input"
		value={query.to ?? ''}
		min={query.from}
		onchange={(e) => onchange({ to: e.currentTarget.value || undefined })}
	/>
</div>
