<script lang="ts">
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import Avatar from '#lib/components/Avatar.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { outbox } from '#lib/outbox.svelte.ts';
	import { isMockDataSource } from '#lib/services/index.ts';

	let { data, children } = $props();

	// Observations saved on this device sync when the app opens and when the connection is back.
	$effect(() => outbox.start(data.user.id));
	const syncNote = $derived(
		[!outbox.online && 'Offline', outbox.waiting && `${outbox.waiting} not synced`]
			.filter(Boolean)
			.join(' · ')
	);

	// Everything about your classrooms is on the dashboard (AP-45): the logo opens it, and the
	// header holds only Record, for those who record (not researchers). Platform admins also manage the registry and host applications.
	const nav = $derived(
		data.user.platformAdmin
			? [
					{ href: resolve('/(app)/registry'), label: 'Registry' },
					{ href: resolve('/(app)/host-applications'), label: 'Applications' }
				]
			: []
	);
</script>

<div class="min-h-dvh">
	<header
		class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur [view-transition-name:app-header] print:hidden"
	>
		<div class="mx-auto flex max-w-6xl items-center gap-2 px-3 py-3 sm:gap-4 sm:px-6">
			<a href={resolve('/(app)/dashboard')} aria-label="Dashboard"><Logo compact /></a>
			<div class="ml-auto"></div>
			{#if nav.length}
				<nav
					aria-label="Admin"
					class="flex items-center gap-0.5 rounded-full border border-line-strong bg-canvas/80 p-1 sm:gap-1"
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
			{/if}
			{#if data.canRecord}
				<a class="btn btn-accent shrink-0 rounded-full" href={resolve('/(app)/observe')}
					><Icon name="plus" class="size-4" /> Record</a
				>
			{/if}
			{#if syncNote}
				<a
					href="{resolve('/(app)/observe')}#sync"
					class={[
						'chip hidden shrink-0 gap-1.5 text-xs sm:inline-flex',
						outbox.online ? 'border-orange/50 text-orange' : 'text-muted'
					]}
				>
					<span
						class={['size-2 rounded-full', outbox.online ? 'bg-orange' : 'bg-muted']}
						aria-hidden="true"
					></span>
					{syncNote}
				</a>
			{/if}
			<ThemeToggle />
			<a href={resolve('/(app)/settings')} class="shrink-0" aria-label="Your account">
				<Avatar name={data.user.fullName} src={data.user.avatarUrl} />
			</a>
		</div>
		{#if syncNote}
			<a
				href="{resolve('/(app)/observe')}#sync"
				class={[
					'block border-b border-line py-1 text-center text-xs sm:hidden',
					outbox.online ? 'text-orange' : 'text-muted'
				]}>{syncNote}</a
			>
		{/if}
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
			'mx-auto max-w-6xl px-4 py-8 transition-opacity duration-200 sm:px-6 print:max-w-none print:p-0',
			navigating.to && 'opacity-60 delay-150'
		]}
		aria-busy={navigating.to ? 'true' : undefined}
	>
		{@render children()}
	</main>
</div>
