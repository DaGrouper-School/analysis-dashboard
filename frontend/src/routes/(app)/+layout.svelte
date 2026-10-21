<script lang="ts">
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import Avatar from '#lib/components/Avatar.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { isMockDataSource } from '#lib/services/index.ts';

	let { data, children } = $props();

	// Platform admins also manage the shared registry of device types and domains.
	const nav = $derived([
		{ href: resolve('/(app)/dashboard'), label: 'Dashboard' },
		{ href: resolve('/(app)/map'), label: 'Map' },
		{ href: resolve('/(app)/classrooms'), label: 'Classrooms' },
		...(data.user.platformAdmin ? [{ href: resolve('/(app)/registry'), label: 'Registry' }] : []),
		{ href: resolve('/(app)/profile'), label: 'Profile' }
	]);
</script>

<div class="min-h-dvh">
	<header
		class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur [view-transition-name:app-header]"
	>
		<div class="mx-auto flex max-w-6xl items-center gap-2 px-3 py-3 sm:gap-4 sm:px-6">
			<a href={resolve('/(app)/dashboard')} aria-label="Dashboard"><Logo compact /></a>
			<nav
				class="ml-auto flex items-center gap-0.5 rounded-full border border-line-strong bg-canvas/80 p-1 sm:gap-1"
			>
				{#each nav as item (item.href)}
					{@const current = page.url.pathname.startsWith(item.href)}
					<a
						href={item.href}
						class={[
							'rounded-full px-2 py-1.5 text-[13px] font-medium sm:px-4 sm:text-sm',
							current ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'
						]}
						aria-current={current ? 'page' : undefined}>{item.label}</a
					>
				{/each}
			</nav>
			<ThemeToggle />
			<a href={resolve('/(app)/profile')} class="hidden sm:block" aria-label="Your profile">
				<Avatar name={data.user.fullName} src={data.user.avatarUrl} />
			</a>
		</div>
		{#if isMockDataSource}
			<p class="border-b border-line bg-raised py-1 text-center text-xs text-muted">
				Demo mode: data is mock and saved only in this browser.
			</p>
		{/if}
	</header>
	<!-- While the next page or sensor loads: a thin bar under the header, and the current
	     content dims after a beat, so a quick switch doesn't flicker. -->
	{#if navigating.to}
		<div
			class="nav-progress fixed inset-x-0 top-0 z-30 h-0.5 overflow-hidden"
			role="progressbar"
			aria-label="Loading"
		></div>
	{/if}
	<main
		class={[
			'mx-auto max-w-6xl px-4 py-8 transition-opacity duration-200 sm:px-6',
			navigating.to && 'opacity-60 delay-150'
		]}
		aria-busy={navigating.to ? 'true' : undefined}
	>
		{@render children()}
	</main>
</div>
