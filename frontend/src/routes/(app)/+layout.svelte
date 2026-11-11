<script lang="ts">
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import Avatar from '#lib/components/Avatar.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import NavLinks from '#lib/components/NavLinks.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { outbox } from '#lib/features/devices/outbox.svelte.ts';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';
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

	// Everything about your classrooms is on the dashboard (AP-45): the logo and the first link
	// open it. Members also get Classroom, the details page of the classroom they are on, and
	// Record if they record (not researchers). Platform admins also manage the registry and host
	// applications. The links look like the public site's, the current one underlined.
	const classroomHref = $derived(
		data.contextClassroom
			? resolve('/(app)/dashboard/details') + sensorQuery(data.contextClassroom.id)
			: null
	);
	const dashboard = resolve('/(app)/dashboard');
	const details = resolve('/(app)/dashboard/details');
	const under = (path: string) =>
		page.url.pathname === path || page.url.pathname.startsWith(`${path}/`);
	const nav = $derived([
		// The dashboard and every page under it, except the classroom's details page.
		{ href: dashboard, label: 'Dashboard', current: under(dashboard) && !under(details) },
		...(classroomHref
			? [
					{
						href: classroomHref,
						// The page has a classroom picker: plural for people in more than one.
						label: data.memberClassrooms.length > 1 ? 'Classrooms' : 'Classroom',
						current: under(details)
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
			<nav
				aria-label="Main"
				class="-my-2 ml-2 flex min-w-0 items-center gap-4 overflow-x-auto py-2 text-sm sm:ml-4 sm:gap-5"
			>
				<NavLinks links={nav} />
			</nav>
			<div class="ml-auto"></div>
			{#if data.canRecord}
				<!-- On a phone just the plus, so the links keep their room. -->
				<a
					class="btn btn-accent shrink-0 rounded-full max-sm:px-2.5"
					href={resolve('/(app)/observe')}
					><Icon name="plus" class="size-4" /><span class="max-sm:sr-only">Record</span></a
				>
			{/if}
			<a
				href="{resolve('/(app)/observe')}#sync"
				class={['chip hidden shrink-0 gap-1.5 text-xs sm:inline-flex', syncChip.class]}
			>
				<span class={['size-2 rounded-full', syncChip.dot]} aria-hidden="true"></span>
				{syncChip.text}
			</a>
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
