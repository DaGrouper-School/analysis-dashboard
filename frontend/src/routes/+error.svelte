<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Logo from '#lib/components/Logo.svelte';

	/**
	 * Any page that is missing or failed: what happened, in plain words, with a way back and a way
	 * home. Home is the public site, which opens the dashboard for signed-in members.
	 */
	const missing = $derived(page.status === 404);
	const heading = $derived(missing ? 'This page is not here' : 'Something went wrong');
</script>

<svelte:head><title>{heading} · DaGrouper</title></svelte:head>

<div class="flex min-h-dvh flex-col bg-canvas">
	<header class="border-b border-line">
		<div class="mx-auto flex max-w-6xl items-center px-4 py-3 sm:px-6">
			<a href={resolve('/')} aria-label="Home"><Logo /></a>
		</div>
	</header>
	<main class="mx-auto grid w-full max-w-xl flex-1 place-items-center px-4 py-16 text-center">
		<div>
			<p class="font-mono text-sm text-muted">{page.status}</p>
			<h1 class="mt-2 font-display text-3xl sm:text-4xl">{heading}</h1>
			<p class="mt-3 text-muted">
				{page.error?.message ||
					(missing
						? 'The link may be old, or the page has moved.'
						: 'Please try again in a moment.')}
			</p>
			<div class="mt-8 flex flex-wrap justify-center gap-3">
				<button type="button" class="btn btn-ghost" onclick={() => history.back()}>Go back</button>
				<a class="btn btn-primary" href={resolve('/')}>Home</a>
			</div>
		</div>
	</main>
</div>
