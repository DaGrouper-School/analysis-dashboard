<script lang="ts">
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import Avatar from '#lib/components/Avatar.svelte';
	import NavLinks from '#lib/components/NavLinks.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import InboxBell from '#lib/views/InboxBell.svelte';
	import { outbox } from '#lib/features/devices/outbox.svelte.ts';
	import { classroomsHref, domainsHref } from '#lib/utils/site.ts';
	import { isMockDataSource } from '#lib/services/index.ts';

	let { data, children } = $props();

	// Observations saved on this device sync when the app opens and when the connection is back.
	$effect(() => outbox.start(data.user.id));
	const syncNote = $derived(
		[!outbox.online && 'Offline', outbox.waiting && `${outbox.waiting} not synced`]
			.filter(Boolean)
			.join(' · ')
	);
	// The header chip always shows the connection: "Online" when there is nothing to report.
	const syncChip = $derived(
		syncNote
			? {
					text: syncNote,
					class: outbox.online ? 'border-orange/50 text-orange' : 'text-muted',
					dot: outbox.online ? 'bg-orange' : 'bg-muted'
				}
			: { text: 'Online', class: 'border-lime/50 text-lime', dot: 'bg-lime' }
	);

	// Everything about your classrooms starts on the dashboard (AP-45): the logo and the first
	// link open it. Members also get Domains, what the classroom they are on measures, and
	// Classrooms, its sections; recording is on the dashboard. Platform admins also
	// manage the registry and host applications. The links look like the public site's, the
	// current one underlined.
	const dashboard = resolve('/(app)/dashboard');
	const domainsPage = resolve('/(app)/dashboard/domains');
	const classroomsPage = resolve('/(app)/dashboard/classrooms');
	const under = (path: string) =>
		page.url.pathname === path || page.url.pathname.startsWith(`${path}/`);
	const context = $derived(data.contextClassroom);
	const nav = $derived([
		// The dashboard and every page under it, except Domains and Classrooms.
		{
			href: dashboard,
			label: 'Dashboard',
			current: under(dashboard) && !under(domainsPage) && !under(classroomsPage)
		},
		...(context
			? [
					// A value's page is under Domains.
					{ href: domainsHref(context.id), label: 'Domains', current: under(domainsPage) },
					{
						href: classroomsHref(context.id),
						label: 'Classrooms',
						current: under(classroomsPage)
					}
				]
			: []),
		...(data.user.platformAdmin
			? [
					{
						href: resolve('/(app)/registry'),
						label: 'Registry',
						current: under(resolve('/(app)/registry'))
					},
					{
						href: resolve('/(app)/host-applications'),
						label: 'Applications',
						current: under(resolve('/(app)/host-applications'))
					}
				]
			: [])
	]);
</script>

<div class="min-h-dvh">
	<header
		class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur [view-transition-name:app-header] print:hidden"
	>
		<div class="mx-auto flex max-w-6xl items-center gap-2 px-3 py-3 sm:gap-4 sm:px-6">
			<a href={resolve('/(app)/dashboard')} aria-label="Dashboard"><Logo compact /></a>
			<!-- On a phone the links get their own row, under this one. -->
			<nav
				aria-label="Main"
				class="-my-2 ml-4 flex min-w-0 items-center gap-5 overflow-x-auto py-2 text-sm max-sm:hidden"
			>
				<NavLinks links={nav} />
			</nav>
			<div class="ml-auto"></div>
			<a
				href="{resolve('/(app)/observe')}#sync"
				class={['chip hidden shrink-0 gap-1.5 text-xs sm:inline-flex', syncChip.class]}
			>
				<span class={['size-2 rounded-full', syncChip.dot]} aria-hidden="true"></span>
				{syncChip.text}
			</a>
			<InboxBell inbox={data.inbox} invitations={data.invitations} parameters={data.parameters} />
			<ThemeToggle />
			<a href={resolve('/(app)/settings')} class="shrink-0" aria-label="Your account">
				<Avatar name={data.user.fullName} src={data.user.avatarUrl} />
			</a>
		</div>
		<nav
			aria-label="Main"
			class="flex items-center justify-around gap-4 border-t border-line px-3 py-2.5 text-sm sm:hidden"
		>
			<NavLinks links={nav} />
		</nav>
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
