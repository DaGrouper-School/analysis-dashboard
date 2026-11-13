<script lang="ts">
	import { goto, invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import InstitutionSetupWizard from '#lib/features/classroom/InstitutionSetupWizard.svelte';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const dashboard = $derived(resolve('/(app)/dashboard') + sensorQuery(data.setup.classroomId));

	async function done() {
		await invalidate('app:classroom');
		// A new classroom goes on to its lab and devices; one that has them goes back to them.
		const next = data.hasLab
			? dashboard
			: resolve('/(app)/classrooms/[id]/labs/new', { id: data.setup.classroomId }) + '?onboarding';
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		await goto(next);
	}

	async function later() {
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		await goto(dashboard);
	}
</script>

<svelte:head><title>Set up {data.setup.institutionName} · DaGrouper Analysis</title></svelte:head>

<!-- eslint-disable svelte/no-navigation-without-resolve -->
<a href={dashboard} class="text-sm text-muted hover:text-ink">← {data.setup.classroomName}</a>
<!-- eslint-enable svelte/no-navigation-without-resolve -->
<div class="mt-4">
	<h1 class="font-display text-3xl sm:text-4xl">Set up {data.setup.institutionName}</h1>
	<p class="mt-1 max-w-xl text-muted">
		Your answers make the institution overview members see, and visitors once the host agreement is
		signed.
	</p>
</div>

<div class="card mt-6 max-w-2xl">
	<InstitutionSetupWizard form={data.form} setup={data.setup} ondone={done} onlater={later} />
</div>
