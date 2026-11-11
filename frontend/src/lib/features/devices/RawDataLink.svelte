<script lang="ts">
	import { resolve } from '$app/paths';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';

	/** Opens a device's raw readings, to read or export. Used on its card and beside the picker. */
	let {
		classroomId,
		deviceId,
		class: className = ''
	}: { classroomId: string; deviceId?: string; class?: string } = $props();
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -->
<a
	class={['btn btn-ghost', className]}
	href={resolve('/(app)/dashboard/readings') + sensorQuery(classroomId, deviceId)}>Raw data</a
>
<!-- eslint-enable svelte/no-navigation-without-resolve -->
