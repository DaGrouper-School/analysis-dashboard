<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const classroom = $derived(data.contextClassroom);
</script>

<svelte:head><title>Labs · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-3xl sm:text-4xl">Labs</h1>
		{#if classroom}<p class="mt-1 text-muted">{classroom.name}</p>{/if}
	</div>
	{#if classroom && data.canAddLabs}
		<a href={resolve('/(app)/classrooms/[id]', { id: classroom.id })} class="btn btn-ghost"
			><Icon name="plus" class="size-4" /> Add a lab in the classroom</a
		>
	{/if}
</div>

{#if !classroom}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">You're not in a classroom yet</h2>
		<p class="mt-2 text-sm text-muted">
			Labs belong to classrooms. Join one with an invite link to see its labs.
		</p>
		<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
	</div>
{:else if data.labs.length}
	<ul class="mt-8 grid gap-4 md:grid-cols-2">
		{#each data.labs as lab (lab.id)}
			<li>
				<a
					href={resolve('/(app)/labs/[labId]', { labId: lab.id })}
					class="card block transition hover:border-line-strong"
				>
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name="flask" class="size-5 text-muted" />
						<span class="min-w-0 break-words">{lab.name}</span>
					</h2>
					<p class="mt-1 text-sm text-muted">{lab.placeName}, {lab.country}</p>
				</a>
			</li>
		{/each}
	</ul>
{:else}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">No labs in {classroom.name} yet</h2>
		<p class="mt-2 text-sm text-muted">
			{data.canAddLabs
				? 'Add one from the classroom page.'
				: 'Your facilitator or institution head adds labs to the classroom.'}
		</p>
	</div>
{/if}
