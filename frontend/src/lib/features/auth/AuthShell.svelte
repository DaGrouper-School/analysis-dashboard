<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';

	/**
	 * The sign-in pages' frame: the Global Classrooms on the globe beside the form, as a hint of
	 * what the app shows, and a way back to the home page (the logo, and a link by the theme).
	 */
	let { children, markers }: { children: Snippet; markers: Promise<GlobeMarker[]> } = $props();
</script>

<div class="grid min-h-dvh lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
	<aside
		class="relative hidden overflow-hidden border-r border-line bg-canvas p-10 lg:flex lg:flex-col"
	>
		<div class="pointer-events-none absolute inset-x-0 top-[42%] bottom-[-38%]">
			{#await markers then shown}<Globe markers={shown} interactive={false} />{/await}
		</div>
		<a href={resolve('/')} class="relative self-start" aria-label="DaGrouper home"><Logo /></a>
		<div class="relative mt-16 mb-auto max-w-md">
			<h1 class="font-display text-3xl leading-tight font-light sm:text-5xl">
				Community science, read from your own station.
			</h1>
			<p class="mt-4 text-text">
				Air, rain, wind, sun, waves and lightning from the Lab in a Bag stations near you, explained
				in plain language.
			</p>
		</div>
		<p class="relative text-sm text-muted">DaGrouper School · PlanetWerx</p>
	</aside>
	<main class="flex flex-col px-5 py-8 sm:px-10">
		<div class="flex items-center justify-between">
			<a href={resolve('/')} class="lg:invisible" aria-label="DaGrouper home"><Logo /></a>
			<div class="flex items-center gap-3">
				<a href={resolve('/')} class="btn btn-ghost px-3 py-2"
					><Icon name="arrow" class="size-4 rotate-180" /> Home</a
				>
				<ThemeToggle />
			</div>
		</div>
		<div class="mx-auto flex w-full max-w-md flex-1 flex-col justify-center py-10">
			{@render children()}
		</div>
	</main>
</div>
