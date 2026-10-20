<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Avatar from '#lib/components/Avatar.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import { isMockDataSource } from '#lib/services/index.ts';

	let { data, children } = $props();

	const nav = [
		{ href: resolve('/(app)/dashboard'), label: 'Dashboard' },
		{ href: resolve('/(app)/map'), label: 'Map' },
		{ href: resolve('/(app)/classrooms'), label: 'Classrooms' },
		{ href: resolve('/(app)/profile'), label: 'Profile' }
	];
</script>

<div class="min-h-dvh">
	<header class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur">
		<div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-3 sm:gap-4 sm:px-6">
			<a href={resolve('/(app)/dashboard')} aria-label="Dashboard"><Logo compact /></a>
			<nav
				class="ml-auto flex items-center gap-0.5 rounded-full border border-line-strong bg-black/60 p-1 sm:gap-1"
			>
				{#each nav as item (item.href)}
					{@const current = page.url.pathname.startsWith(item.href)}
					<a
						href={item.href}
						class={[
							'rounded-full px-2.5 py-1.5 text-[13px] font-medium sm:px-4 sm:text-sm',
							current ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'
						]}
						aria-current={current ? 'page' : undefined}>{item.label}</a
					>
				{/each}
			</nav>
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
	<main class="mx-auto max-w-6xl px-4 py-8 sm:px-6">
		{@render children()}
	</main>
</div>
