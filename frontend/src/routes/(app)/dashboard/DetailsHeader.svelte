<script lang="ts">
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import Select, { type SelectOption } from '#lib/components/Select.svelte';

	/** The details page's header (AP-45): back to the dashboard, and the classroom picked. */
	let {
		dashboardHref,
		loading,
		classroomOptions,
		classroomId,
		onclassroom
	}: {
		dashboardHref: string;
		/** A classroom is being opened: its sections show as loading. */
		loading: boolean;
		classroomOptions: SelectOption[];
		classroomId: string;
		onclassroom: (id: string) => void;
	} = $props();
</script>

<Breadcrumbs
	items={[{ label: 'Dashboard', href: dashboardHref }, { label: 'Classroom details' }]}
/>
<header class={['mt-4', loading && 'is-loading']}>
	<h1 class="font-display text-4xl">Classroom details</h1>
	<Select
		class="mt-5 sm:w-1/2 lg:w-1/3"
		options={classroomOptions}
		value={classroomId}
		label="Classroom"
		onchange={onclassroom}
	/>
</header>
