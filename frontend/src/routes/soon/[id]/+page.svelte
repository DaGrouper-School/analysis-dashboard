<script lang="ts">
	import { resolve } from '$app/paths';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>{data.section.label} · Da Grouper School</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<div class="mx-auto max-w-xl py-16 text-center">
		<p class="chip mx-auto w-fit">Coming soon</p>
		<h1 class="mt-4 font-display text-4xl">{data.section.label}</h1>
		<p class="mt-3 text-muted">{data.section.description}</p>
		<p class="mt-2 text-sm text-muted">This part of the site is not built yet.</p>
		<a href={resolve('/')} class="btn btn-ghost mt-8">Back to home</a>
	</div>
</PublicShell>
