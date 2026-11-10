<script lang="ts">
	import InstitutionOverview from '#lib/components/InstitutionOverview.svelte';
	import ContributorId from '#lib/components/ContributorId.svelte';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	/** A classroom's public profile (AP-29): who they are, what they study, their kit. */
	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const profile = $derived(data.profile);
	const stage = $derived(data.content.stages.find((s) => s.id === classroom.stageId));
	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));
	const place = $derived([classroom.placeName, classroom.country].filter(Boolean).join(', '));
	const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'full', timeStyle: 'short' });
	const years = (n: number) => `${n} ${n === 1 ? 'year' : 'years'}`;
</script>

<svelte:head><title>{classroom.name} · {data.content.hero.eyebrow}</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<nav aria-label="Breadcrumb" class="text-sm text-muted">
		<a class="hover:text-ink" href={resolve('/global-classrooms')}
			>{directory?.label ?? 'Global Classrooms'}</a
		>
		<span aria-hidden="true" class="px-1">›</span><span class="text-ink" aria-current="page"
			>{classroom.name}</span
		>
	</nav>

	<header class="mt-4 flex flex-wrap items-start justify-between gap-4">
		<div class="max-w-3xl">
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="font-display text-4xl">{classroom.name}</h1>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
			<p class="mt-2 flex flex-wrap items-center gap-2 text-muted">
				{#if place}<span>{place}</span>{/if}
				{#if profile.areaCode}<span class="chip font-mono">{profile.areaCode}</span>{/if}
				<StageBadge {stage} stageId={classroom.stageId} />
			</p>
			<dl class="mt-3 grid gap-x-6 gap-y-1 text-sm sm:grid-cols-[auto_1fr]">
				{#if profile.language}
					<dt class="text-muted">Language</dt>
					<dd>{profile.language}</dd>
				{/if}
				{#if classroom.institutionName}
					<dt class="text-muted">Host institution</dt>
					<dd>{classroom.institutionName}</dd>
				{/if}
				{#if profile.ecosystems.length}
					<dt class="text-muted">Ecosystems</dt>
					<dd>{profile.ecosystems.join(', ')}</dd>
				{/if}
			</dl>
			{#if profile.description}<p class="mt-4 text-ink">{profile.description}</p>{/if}
		</div>
		<div class="flex flex-wrap gap-2">
			<a class="btn btn-primary" href={resolve('/explore/[classroom]', { classroom: classroom.id })}
				>See this classroom's data</a
			>
			{#if !data.member}
				<a class="btn btn-ghost" href="#join">{data.content.joinClassroom.title}</a>
			{/if}
			{#if data.canManage}
				<a class="btn btn-ghost" href={resolve('/(app)/classrooms/[id]', { id: classroom.id })}
					>Manage classroom</a
				>
			{/if}
		</div>
	</header>

	{#if data.institution}
		<InstitutionOverview overview={data.institution} class="mt-8 max-w-xl" />
	{/if}

	<section class="mt-10" aria-labelledby="projects-heading">
		<h2 id="projects-heading" class="font-display text-2xl">Research projects</h2>
		{#each data.projects as project (project.id)}
			<article class="card mt-4">
				<h3 class="text-lg font-semibold">{project.title}</h3>
				<p class="mt-1 text-muted">{project.question}</p>
				<ul class="mt-4 grid gap-3 md:grid-cols-2">
					{#each project.sites as site (site.code)}
						<li class="rounded-md border border-line p-3">
							<p class="flex flex-wrap items-center gap-2 font-medium text-ink">
								{site.name}<span class="chip font-mono text-xs">{site.code}</span>
							</p>
							<dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
								<dt class="text-muted">Ecosystem</dt>
								<dd>{site.ecosystem}</dd>
								<dt class="text-muted">Readings</dt>
								<dd>{site.readings.join(', ')}</dd>
								<dt class="text-muted">How often</dt>
								<dd>{site.frequency}</dd>
								<dt class="text-muted">Planned for</dt>
								<dd>{years(site.yearsPlanned)}</dd>
							</dl>
						</li>
					{/each}
				</ul>
			</article>
		{:else}
			<p class="mt-3 text-muted">No research projects yet.</p>
		{/each}
	</section>

	<section class="mt-10" aria-labelledby="kit-heading">
		<h2 id="kit-heading" class="font-display text-2xl">Lab in a Bag here</h2>
		{#if data.custodianRole}
			<p class="mt-1 text-sm text-muted">
				Looked after by the {CLASSROOM_ROLE_LABELS[data.custodianRole].toLowerCase()}.
			</p>
		{/if}
		{#if data.kit.length}
			<ul class="mt-4 grid gap-4 md:grid-cols-2">
				{#each data.kit as item (item.deviceType.id)}
					<li class="card">
						<p class="flex items-center gap-2 font-semibold text-ink">
							<Icon name={item.deviceType.icon} class="size-5 text-muted" />
							{item.deviceType.label}{item.count > 1 ? ` × ${item.count}` : ''}
						</p>
						{#if item.measures.length}
							<p class="mt-2 text-sm text-muted">Measures {item.measures.join(', ')}</p>
						{/if}
						{#if item.serials.length}
							<p class="mt-2 font-mono text-xs break-all text-muted">
								Serial {item.serials.join(', ')}
							</p>
						{/if}
					</li>
				{/each}
			</ul>
		{:else}
			<p class="mt-3 text-muted">No devices registered yet.</p>
		{/if}
	</section>

	<section class="mt-10" aria-labelledby="sessions-heading">
		<h2 id="sessions-heading" class="font-display text-2xl">Next sessions</h2>
		{#if data.sessions.length}
			<ul class="mt-4 divide-y divide-line rounded-md border border-line">
				{#each data.sessions as session (session.id)}
					<li class="p-4">
						<p class="font-medium text-ink">{session.title}</p>
						<p class="text-sm text-muted">
							<time datetime={session.startsAt}>{when.format(new Date(session.startsAt))}</time> ·
							{session.place}
						</p>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="mt-3 text-muted">No sessions planned yet.</p>
		{/if}
	</section>

	<section class="mt-10" aria-labelledby="people-heading">
		<h2 id="people-heading" class="font-display text-2xl">Community scientists and contributors</h2>
		{#if data.contributors.length}
			<p class="mt-1 text-sm text-muted">
				Every value is credited to a participant ID. Open one to check what they recorded.
			</p>
			<ul class="mt-3 flex flex-wrap gap-2">
				{#each data.contributors as person (person.participantId)}
					<li class="chip gap-2">
						{#if person.name}{person.name}{/if}
						<ContributorId participantId={person.participantId} />
					</li>
				{/each}
			</ul>
		{:else}
			<p class="mt-3 text-muted">No released contributions yet.</p>
		{/if}
	</section>

	{#if !data.member}
		<section id="join" class="card mt-10 scroll-mt-24" aria-labelledby="join-heading">
			<h2 id="join-heading" class="text-lg font-semibold">{data.content.joinClassroom.title}</h2>
			<p class="mt-1 text-muted">{data.content.joinClassroom.body}</p>
			{#if !data.user}
				<a class="btn btn-primary mt-4" href={resolve('/(auth)/signup')}
					>{data.content.joinClassroom.action}</a
				>
			{/if}
		</section>
	{/if}
</PublicShell>
