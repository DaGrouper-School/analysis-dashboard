<script lang="ts">
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import Select, { type SelectOption } from '#lib/components/Select.svelte';

	/** The details page's header (AP-45): back to the dashboard, and the classroom and device picked. */
	let {
		dashboardHref,
		loading,
		classroomOptions,
		classroomId,
		deviceOptions,
		deviceId,
		onclassroom,
		ondevice
	}: {
		dashboardHref: string;
		/** A classroom is being opened: its sections show as loading. */
		loading: boolean;
		classroomOptions: SelectOption[];
		classroomId: string;
		deviceOptions: SelectOption[];
		deviceId: string | undefined;
		onclassroom: (id: string) => void;
		ondevice: (id: string) => void;
	} = $props();
</script>

<Breadcrumbs
	items={[{ label: 'Dashboard', href: dashboardHref }, { label: 'Classroom details' }]}
/>
<header class={['mt-4', loading && 'is-loading']}>
	<h1 class="font-display text-4xl">Classroom details</h1>
	<div class="mt-5 grid gap-3 sm:grid-cols-2 lg:w-2/3">
		<Select
			options={classroomOptions}
			value={classroomId}
			label="Classroom"
			onchange={onclassroom}
		/>
		{#if deviceOptions.length}
			<Select options={deviceOptions} value={deviceId} label="Device" onchange={ondevice} />
		{/if}
	</div>
</header>
