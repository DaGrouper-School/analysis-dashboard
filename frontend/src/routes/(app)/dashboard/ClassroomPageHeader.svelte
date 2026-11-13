<script lang="ts">
	import Select, { type SelectOption } from '#lib/components/Select.svelte';

	/**
	 * The header of the members' Domains and Classrooms pages: the page's title, a line under it,
	 * and the classroom picked from the user's own classrooms.
	 */
	let {
		title,
		sub,
		loading,
		classroomOptions,
		classroomId,
		onclassroom
	}: {
		title: string;
		sub: string;
		/** A classroom is being opened: the page shows as loading. */
		loading: boolean;
		classroomOptions: SelectOption[];
		classroomId: string;
		onclassroom: (id: string) => void;
	} = $props();
</script>

<header class={[loading && 'is-loading']}>
	<h1 class="font-display text-4xl">{title}</h1>
	<p class="mt-2 max-w-2xl text-sm text-muted">{sub}</p>
	<Select
		class="mt-5 sm:w-1/2 lg:w-1/3"
		options={classroomOptions}
		value={classroomId}
		label="Classroom"
		onchange={onclassroom}
	/>
</header>
