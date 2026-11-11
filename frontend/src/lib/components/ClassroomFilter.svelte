<script lang="ts">
	import { filterOptions, NO_FILTER, type DirectoryFilter } from '#lib/directory.ts';
	import type { PublicStation } from '#lib/domain/types.ts';

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
			items: options.stages.map((s) => ({ value: s.id, label: s.label }))
		}
	]);
</script>

<form
	class={['flex flex-wrap items-end gap-3', className]}
	aria-label="Filter classrooms"
	onsubmit={(e) => e.preventDefault()}
>
	{#each pickers as picker (picker.id)}
		<label class="flex flex-col gap-1 text-xs text-muted">
			{picker.label}
			<select
				class="input w-auto max-w-48 py-1.5 pr-9 text-sm"
				value={filter[picker.id]}
				onchange={(e) => onchange({ ...filter, [picker.id]: e.currentTarget.value })}
			>
				<option value="">{picker.any}</option>
				{#each picker.items as item (item.value)}
					<option value={item.value}>{item.label}</option>
				{/each}
			</select>
		</label>
	{/each}
	<button
		type="button"
		class="btn btn-ghost py-1.5 text-sm"
		disabled={!filtered}
		onclick={() => onchange(NO_FILTER)}>Clear</button
	>
</form>
