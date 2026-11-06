<script lang="ts">
	import { goto, invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import { rememberClassroom } from '#lib/classroom-context.ts';
	import Avatar from '#lib/components/Avatar.svelte';
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

	// Platform admins also manage the shared registry and review host applications.
	const nav = $derived([
		{ href: resolve('/(app)/dashboard'), label: 'Dashboard' },
		{ href: resolve('/(app)/map'), label: 'Map' },
		{ href: resolve('/(app)/labs'), label: 'Labs' },
		...(data.user.platformAdmin
			? [
					{ href: resolve('/(app)/registry'), label: 'Registry' },
					{ href: resolve('/(app)/host-applications'), label: 'Applications' }
				]
			: []),
		{ href: resolve('/(app)/settings'), label: 'Settings' }
	]);

	// The classroom in context: the one the open page shows, else the one picked last.
	const classroomId = $derived(
		(page.data.contextClassroomId as string | undefined) ??
			(page.data.classroom as { id: string } | null | undefined)?.id ??
			data.contextClassroom?.id ??
			''
	);

	/** Puts another classroom in context; the dashboard, domains and labs follow it. */
	async function pickClassroom(id: string) {
		rememberClassroom(id);
		const path = page.url.pathname;
		const query = `?classroom=${encodeURIComponent(id)}`;
		// Paths are resolved; only the query is added.
		/* eslint-disable svelte/no-navigation-without-resolve */
		if (path.startsWith(resolve('/(app)/dashboard'))) await goto(path + query);
		else if (path.startsWith(resolve('/(app)/labs'))) await goto(resolve('/(app)/labs') + query);
		else await invalidate('app:classroom-context');
		/* eslint-enable svelte/no-navigation-without-resolve */
	}
</script>

{#snippet classroomPicker(extra: string)}
	<div class={['min-w-0', extra]}>
		<label class="sr-only" for="context-classroom-{extra ? 'wide' : 'narrow'}">Classroom</label>
		{#if data.memberClassrooms.length > 1}
			<select
				id="context-classroom-{extra ? 'wide' : 'narrow'}"
				class="input w-full max-w-64 py-1.5 pr-9 text-sm"
				value={classroomId}
				onchange={(e) => pickClassroom(e.currentTarget.value)}
			>
				{#each data.memberClassrooms as classroom (classroom.id)}
					<option value={classroom.id}>{classroom.name}</option>
				{/each}
			</select>
		{:else}
			<p
				id="context-classroom-{extra ? 'wide' : 'narrow'}"
				class="max-w-64 truncate text-sm text-muted"
			>
				{data.memberClassrooms[0].name}
			</p>
		{/if}
	</div>
{/snippet}

<div class="min-h-dvh">
	<header
		class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur [view-transition-name:app-header]"
	>
		<div class="mx-auto flex max-w-6xl items-center gap-2 px-3 py-3 sm:gap-4 sm:px-6">
			<a href={resolve('/(app)/dashboard')} aria-label="Dashboard"><Logo compact /></a>
			{#if data.memberClassrooms.length}
				{@render classroomPicker('hidden md:block')}
			{/if}
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
			<a href={resolve('/(app)/settings')} class="hidden sm:block" aria-label="Settings">
				<Avatar name={data.user.fullName} src={data.user.avatarUrl} />
			</a>
		</div>
		{#if data.memberClassrooms.length}
			<div class="mx-auto max-w-6xl px-3 pb-3 md:hidden">{@render classroomPicker('')}</div>
		{/if}
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
			'mx-auto max-w-6xl px-4 py-8 transition-opacity duration-200 sm:px-6',
			navigating.to && 'opacity-60 delay-150'
		]}
		aria-busy={navigating.to ? 'true' : undefined}
	>
		{@render children()}
	</main>
</div>
