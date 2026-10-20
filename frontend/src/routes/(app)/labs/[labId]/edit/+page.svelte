<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import LabForm from '#lib/components/LabForm.svelte';
	import type { CreateLabInput } from '#lib/domain/types.ts';
	import { services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function save(input: CreateLabInput) {
		await services.labs.update(data.lab.id, input);
		await goto(resolve('/(app)/labs/[labId]', { labId: data.lab.id }), { invalidate: ['app:lab'] });
	}
</script>

<svelte:head><title>Edit {data.lab.name} · DaGrouper Analysis</title></svelte:head>

<a
	href={resolve('/(app)/labs/[labId]', { labId: data.lab.id })}
	class="text-sm text-muted hover:text-ink">← {data.lab.name}</a
>
<h1 class="mt-4 font-display text-3xl sm:text-4xl">Edit lab</h1>

<LabForm initial={data.lab} submitLabel="Save changes" {save} />
