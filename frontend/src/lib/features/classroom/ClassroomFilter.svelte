<script lang="ts">
	import {
		filterOptions,
		NO_FILTER,
		type DirectoryFilter
	} from '#lib/features/classroom/directory.ts';
	import type { PublicStation } from '#lib/domain/types.ts';
	import Select from '#lib/components/Select.svelte';

	/**
	 * The public Classrooms filters (AP-28, AP-45): region, country and stage, offering only what
	 * the classrooms use. They narrow the classroom tabs and the globe.
	 */
	let {
		places,
		content,
		filter,
		onchange,
		class: className = ''
	}: {
		places: PublicStation['places'];
		content: PublicStation['content'];
		filter: DirectoryFilter;
		onchange: (filter: DirectoryFilter) => void;
		class?: string;
	} = $props();

	const options = $derived(filterOptions(places, content));
	const filtered = $derived(Object.values(filter).some(Boolean));
	const pickers = $derived([
		{
			id: 'region' as const,
			label: 'Region',
			any: 'All regions',
			items: options.regions.map((r) => ({ value: r.id, label: r.label }))
		},
		{
			id: 'country' as const,
			label: 'Country',
			any: 'All countries',
			items: options.countries.map((c) => ({ value: c, label: c }))
		},
		{
			id: 'stage' as const,
			label: 'Stage',
			any: 'All stages',
			items: options.stages.map((s) => ({ value: s.id, label: s.label, color: s.color }))
		}
	]);
</script>

<form
	class={['flex flex-wrap items-end gap-3', className]}
	aria-label="Filter classrooms"
	onsubmit={(e) => e.preventDefault()}
>
	{#each pickers as picker (picker.id)}
		<Select
			label={picker.label}
			size="compact"
			class="max-w-56"
			options={[{ value: '', label: picker.any }, ...picker.items]}
			value={filter[picker.id]}
			onchange={(value) => onchange({ ...filter, [picker.id]: value })}
		/>
	{/each}
	<button
		type="button"
		class="btn btn-ghost py-1.5 text-sm"
		disabled={!filtered}
		onclick={() => onchange(NO_FILTER)}>Clear</button
	>
</form>
