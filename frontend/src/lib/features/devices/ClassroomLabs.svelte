<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import type { Lab } from '#lib/domain/types.ts';

	/**
	 * A classroom's labs (AP-45), each with its place, device count and site profile. Those who
	 * may add a lab get the button, or the next step when there is none yet.
	 */
	let {
		classroomId,
		labs,
		devices,
		canAdd
	}: {
		classroomId: string;
		labs: Lab[];
		devices: { labId: string }[];
		canAdd: boolean;
	} = $props();
</script>

{#if labs.length}
	{#if canAdd}
		<a
			href={resolve('/(app)/classrooms/[id]/labs/new', { id: classroomId })}
			class="btn btn-ghost mb-4"><Icon name="plus" class="size-4" /> Add a lab</a
		>
	{/if}
	<ul class="grid gap-3 sm:grid-cols-2">
		{#each labs as lab (lab.id)}
			{@const count = devices.filter((d) => d.labId === lab.id).length}
			<li>
				<a
					class="card block h-full p-5 transition hover:border-line-strong"
					href={resolve('/(app)/labs/[labId]', { labId: lab.id })}
				>
					<Icon name="flask" class="size-6 text-teal" />
					<p class="mt-3 font-medium text-ink">{lab.name}</p>
					<p class="text-sm text-muted">
						{lab.placeName}, {lab.country} · {count} device{count === 1 ? '' : 's'}
					</p>
					{#if lab.siteProfile?.description}
						<p class="mt-2 text-sm">{lab.siteProfile.description}</p>
					{/if}
				</a>
			</li>
		{/each}
	</ul>
{:else if canAdd}
	<div class="flex flex-wrap items-center justify-between gap-4 rounded-md bg-lime/25 p-4">
		<p class="text-sm">
			<strong>Next step:</strong> register your lab, its site profile and its devices.
		</p>
		<a
			href={resolve('/(app)/classrooms/[id]/labs/new?onboarding', { id: classroomId })}
			class="btn btn-primary">Set up your lab</a
		>
	</div>
{:else}
	<p class="text-muted">Your facilitator hasn't registered a lab yet.</p>
{/if}
