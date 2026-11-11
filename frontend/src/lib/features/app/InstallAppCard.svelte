<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { install, promptInstall } from './install.svelte.ts';

	let { icon = 'device' }: { icon?: string } = $props();
</script>

<section class="card max-w-2xl">
	<h2 class="flex items-center gap-2 text-lg font-semibold">
		<Icon name={icon} class="size-5 text-muted" /> Install the app
	</h2>
	{#if install.installed}
		<p class="mt-3 flex items-center gap-2 text-sm">
			<Icon name="check" class="size-4 text-lime" /> Installed. Open DaGrouper from your home screen or
			app list.
		</p>
	{:else}
		<p class="mt-3 text-sm text-muted">
			Add DaGrouper to your home screen or desktop. It opens in its own window and keeps working
			without a signal: readings you record offline are sent when you are back online.
		</p>
		{#if install.prompt}
			<button class="btn btn-accent mt-5" onclick={promptInstall}>
				<Icon name="download" class="size-4" /> Install app
			</button>
		{:else if install.ios}
			<p class="mt-4 text-sm">
				In Safari, tap <span class="font-medium text-ink">Share</span>, then
				<span class="font-medium text-ink">Add to Home Screen</span>.
			</p>
		{:else}
			<p class="mt-4 text-sm">
				Use your browser's menu and choose <span class="font-medium text-ink">Install app</span> or
				<span class="font-medium text-ink">Add to Home screen</span>.
			</p>
		{/if}
	{/if}
</section>
