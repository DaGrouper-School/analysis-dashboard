<script lang="ts">
	import type { Snippet } from 'svelte';
	import Globe, { type GlobeMarker } from '#lib/features/map/Globe.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';

	/** The Global Classrooms on the globe beside the form, as a hint of what the app shows. */
	let { children, markers }: { children: Snippet; markers: Promise<GlobeMarker[]> } = $props();
</script>

<div class="grid min-h-dvh lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
	<aside
		class="relative hidden overflow-hidden border-r border-line bg-canvas p-10 lg:flex lg:flex-col"
	>
		<div class="pointer-events-none absolute inset-x-0 top-[42%] bottom-[-38%]">
			{#await markers then shown}<Globe markers={shown} interactive={false} />{/await}
		</div>
		<Logo />
		<div class="relative mt-16 mb-auto max-w-md">
			<h1 class="font-display text-3xl leading-tight font-light sm:text-5xl">
				Community science, read from your own station.
			</h1>
			<p class="mt-4 text-text">
				Air, rain, wind, sun, waves and lightning from the Lab in a Bag stations near you, explained
				in plain language.
			</p>
		</div>
		<p class="relative text-sm text-muted">DaGrouper School · PlanetWerx · Tefer</p>
	</aside>
	<main class="flex flex-col px-5 py-8 sm:px-10">
		<div class="flex items-center justify-between">
			<div class="lg:invisible"><Logo /></div>
			<ThemeToggle />
		</div>
		<div class="mx-auto flex w-full max-w-md flex-1 flex-col justify-center py-10">
			{@render children()}
		</div>
	</main>
</div>
