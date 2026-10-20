<script lang="ts">
	import type { Snippet } from 'svelte';
	import Globe, { type GlobeMarker } from './Globe.svelte';
	import Logo from './Logo.svelte';

	// The seeded labs, as a hint of what the app shows.
	const SITES: GlobeMarker[] = [
		{ id: 'addis', lat: 9.03, lng: 38.74, color: '#3cc04b', label: 'Addis Ababa' },
		{ id: 'shashamane', lat: 7.2, lng: 38.6, color: '#00bdff', label: 'Shashamane' },
		{ id: 'hargeisa', lat: 9.56, lng: 44.06, color: '#3cc04b', label: 'Hargeisa' },
		{ id: 'nassau', lat: 25.06, lng: -77.34, color: '#f6921e', label: 'Nassau' }
	];

	let { children }: { children: Snippet } = $props();
</script>

<div class="grid min-h-dvh lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
	<aside
		class="relative hidden overflow-hidden border-r border-line bg-canvas p-10 lg:flex lg:flex-col"
	>
		<div class="pointer-events-none absolute inset-x-0 top-[42%] bottom-[-38%]">
			<Globe markers={SITES} interactive={false} />
		</div>
		<Logo inverted />
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
		<div class="lg:hidden"><Logo /></div>
		<div class="mx-auto flex w-full max-w-md flex-1 flex-col justify-center py-10">
			{@render children()}
		</div>
	</main>
</div>
