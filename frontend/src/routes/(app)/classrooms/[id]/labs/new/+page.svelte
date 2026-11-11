<script lang="ts">
	import { sectionQuery } from '#lib/utils/sensor-selection.ts';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import LabForm from '#lib/features/devices/LabForm.svelte';
	import SetupSteps from '#lib/features/classroom/SetupSteps.svelte';
	import type { CreateLabInput } from '#lib/domain/types.ts';
	import { services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function save(input: CreateLabInput) {
		const lab = await services.labs.create(data.classroom.id, input);
		await goto(
			data.onboarding
				? resolve('/(app)/labs/[labId]?onboarding', { labId: lab.id })
				: resolve('/(app)/labs/[labId]', { labId: lab.id })
		);
	}
</script>

<svelte:head><title>Add a lab · DaGrouper Analysis</title></svelte:head>

<!-- eslint-disable svelte/no-navigation-without-resolve -->
<a
	href={resolve('/(app)/dashboard') + sectionQuery(data.classroom.id, 'labs')}
	class="text-sm text-muted hover:text-ink">← {data.classroom.name}</a
>
<!-- eslint-enable svelte/no-navigation-without-resolve -->
<div class="mt-4">
	{#if data.onboarding}<SetupSteps current={2} />{/if}
	<h1 class="font-display text-3xl sm:text-4xl">Register your lab</h1>
	<p class="mt-1 max-w-xl text-muted">
		The lab is where your classroom works with the Lab in a Bag. Next you'll register its devices.
	</p>
</div>

<LabForm submitLabel="Save lab and add devices" {save} />
