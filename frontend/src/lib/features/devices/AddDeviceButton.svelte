<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import Icon from '#lib/components/Icon.svelte';
	import Select from '#lib/components/Select.svelte';
	import type { DeviceTypeDefinition, DomainDefinition, Parameter } from '#lib/domain/types.ts';
	import RegisterSensorModal from '#lib/features/devices/RegisterSensorModal.svelte';

	/**
	 * Adds a device to one of the classroom's labs: the lab to add it to (when there is more than
	 * one), and a button that opens the pairing steps. The same on the dashboard, beside the
	 * device picker, and in the classroom's Measurements. Only for whoever manages its devices.
	 */
	let {
		labs,
		deviceTypes,
		domains,
		parameters,
		label,
		class: className = ''
	}: {
		labs: { id: string; name: string }[];
		deviceTypes: DeviceTypeDefinition[];
		domains: DomainDefinition[];
		parameters: Parameter[];
		label: string;
		class?: string;
	} = $props();

	let adding = $state(false);
	let labId = $state('');
	const lab = $derived(labs.find((l) => l.id === labId) ?? labs[0]);
	const labOptions = $derived(labs.map((l) => ({ value: l.id, label: l.name })));
</script>

{#if lab}
	<div class={['flex flex-wrap items-center gap-2', className]}>
		{#if labs.length > 1}
			<Select
				options={labOptions}
				value={lab.id}
				label="Lab to add it to"
				size="compact"
				onchange={(id) => (labId = id)}
			/>
		{/if}
		<button
			type="button"
			class="btn btn-primary shrink-0"
			aria-haspopup="dialog"
			onclick={() => (adding = true)}
		>
			<Icon name="plus" class="size-4" />
			{label}
		</button>
	</div>
	<RegisterSensorModal
		bind:open={adding}
		labId={lab.id}
		labName={lab.name}
		{deviceTypes}
		{domains}
		{parameters}
		onclose={() => invalidateAll()}
	/>
{/if}
