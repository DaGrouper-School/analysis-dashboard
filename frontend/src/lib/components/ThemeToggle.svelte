<script lang="ts">
	import { theme, toggleTheme } from '#lib/theme.svelte.ts';

	const next = $derived(theme.current === 'dark' ? 'light' : 'dark');
</script>

<button
	type="button"
	class="grid size-8 shrink-0 place-items-center rounded-full border border-line-strong text-muted transition hover:border-ink hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none sm:size-9"
	aria-label="Switch to {next} theme"
	title="Switch to {next} theme"
	onclick={toggleTheme}
>
	<svg
		viewBox="0 0 24 24"
		class="size-4"
		fill="none"
		stroke="currentColor"
		stroke-width="2"
		stroke-linecap="round"
		aria-hidden="true"
	>
		{#if theme.current === 'dark'}
			<!-- Sun: shown in the dark theme, switches to light. -->
			<circle cx="12" cy="12" r="4" />
			<path
				d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"
			/>
		{:else}
			<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" />
		{/if}
	</svg>
</button>
