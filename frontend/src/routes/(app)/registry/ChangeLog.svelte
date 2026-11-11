<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { RegistryChange } from '#lib/domain/types.ts';

	/** The Registry's change log (AP-09): the latest 30 changes, newest first. */
	let { changes, icon }: { changes: RegistryChange[]; icon: string } = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<div>
	<h2 class="flex items-center gap-2 text-xl font-semibold">
		<Icon name={icon} class="size-5 text-muted" /> Change log
	</h2>
	{#if changes.length}
		<ol class="card mt-4 divide-y divide-line p-0">
			{#each changes.slice(0, 30) as change (change.id)}
				<li class="px-4 py-3 text-sm">
					<p class="text-ink">{change.summary}</p>
					<p class="text-xs text-muted">
						{change.byName} · {dateTime.format(new Date(change.at))}
					</p>
				</li>
			{/each}
		</ol>
	{:else}
		<p class="mt-2 text-sm text-muted">No changes yet.</p>
	{/if}
</div>
