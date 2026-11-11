<script lang="ts">
	import { updated } from '$app/state';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * Says when a new version of the app has been published, so an open or installed app does not
	 * keep running the old one. SvelteKit checks for it on a timer and when the tab comes back.
	 */
	let dismissed = $state(false);
</script>

{#if updated.current && !dismissed}
	<div
		class="fixed inset-x-4 bottom-4 z-40 mx-auto flex max-w-md items-center gap-3 rounded-md border border-line-strong bg-raised px-4 py-3 shadow-lg"
		role="status"
	>
		<Icon name="sync" class="size-5 text-lime" />
		<p class="flex-1 text-sm text-ink">A new version of the app is ready.</p>
		<button class="btn btn-primary px-3 py-1.5" onclick={() => location.reload()}>Reload</button>
		<button
			class="rounded-sm p-1 text-muted hover:text-ink"
			aria-label="Dismiss"
			onclick={() => (dismissed = true)}
		>
			<Icon name="close" class="size-4" />
		</button>
	</div>
{/if}
