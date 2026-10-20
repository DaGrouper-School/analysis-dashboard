<script lang="ts">
	import type { Snippet } from 'svelte';
	import Logo from './Logo.svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="grid min-h-dvh lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
	<aside
		class="relative hidden overflow-hidden bg-forest p-10 text-white lg:flex lg:flex-col lg:justify-between"
	>
		<div
			class="pointer-events-none absolute -right-24 -bottom-24 size-[28rem] rounded-full bg-lime/20 blur-3xl"
			aria-hidden="true"
		></div>
		<div
			class="pointer-events-none absolute top-1/3 -left-16 size-72 rounded-full bg-lime-strong/10 blur-2xl"
			aria-hidden="true"
		></div>
		<Logo inverted />
		<div class="relative max-w-md">
			<h1 class="font-display text-5xl leading-tight">
				Community science, read from your own station.
			</h1>
			<p class="mt-4 text-white/70">
				Air, rain, wind, sun, waves and lightning from the Lab in a Bag stations near you, explained
				in plain language.
			</p>
		</div>
		<p class="relative text-sm text-white/50">DaGrouper School · PlanetWerx · Tefer</p>
	</aside>
	<main class="flex flex-col px-5 py-8 sm:px-10">
		<div class="lg:hidden"><Logo /></div>
		<div class="mx-auto flex w-full max-w-md flex-1 flex-col justify-center py-10">
			{@render children()}
		</div>
	</main>
</div>
