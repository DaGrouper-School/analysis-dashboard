<script lang="ts">
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import { resolve } from '$app/paths';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import { formatValue } from '#lib/features/measurements/key-values.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/utils/labels.ts';
	import type { PageProps } from './$types';

	/**
	 * A contributor's public profile (AP-39), found by participant ID: SAM stores only the ID, and
	 * this platform shows what they recorded. Who they are shows only with their consent.
	 */
	let { data }: PageProps = $props();

	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));
	const title = $derived(data.profile?.fullName || `Participant ${data.participantId}`);
	const ROLE_LABELS: Record<string, string> = CLASSROOM_ROLE_LABELS;
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<svelte:head><title>{title} · {data.content.hero.eyebrow}</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<Breadcrumbs
		items={[
			{ label: directory?.label ?? 'Classrooms', href: resolve('/classrooms') },
			{ label: title }
		]}
	/>

	<header class="mt-4">
		<div class="flex flex-wrap items-center gap-3">
			<h1 class="font-display text-4xl">{title}</h1>
			<SampleDataBadge show={data.content.sampleData} />
		</div>
		<p class="mt-2 text-muted">
			{#if data.profile}
				Participant <span class="font-mono text-text">{data.participantId}</span> &middot;
			{/if}
			{data.total}
			released {data.total === 1 ? 'value' : 'values'}
		</p>
		{#if data.profile}
			<p class="mt-1 flex flex-wrap gap-x-3 text-sm text-muted">
				{#if ROLE_LABELS[data.profile.role]}<span>{ROLE_LABELS[data.profile.role]}</span>{/if}
				{#if data.profile.email}<span class="select-all">{data.profile.email}</span>{/if}
			</p>
		{:else}
			<p class="mt-1 text-sm text-muted">This participant keeps their profile private.</p>
		{/if}
		{#if data.classrooms.length}
			<p class="mt-3 flex flex-wrap items-center gap-2 text-sm">
				<span class="text-muted">Contributes to</span>
				{#each data.classrooms as classroom (classroom.id)}
					<a
						class="chip hover:border-ink hover:text-ink"
						href={resolve('/classrooms/[classroom]', { classroom: classroom.id })}
						>{classroom.name}</a
					>
				{/each}
			</p>
		{/if}
	</header>

	<section class="mt-8" aria-labelledby="contributions-heading">
		<h2 id="contributions-heading" class="font-display text-2xl">Contributions</h2>
		<p class="mt-1 text-sm text-muted">
			Released values recorded by this participant{data.total > data.recent.length
				? `, latest ${data.recent.length} of ${data.total}`
				: ''}.
		</p>
		<div class="card mt-4 overflow-x-auto p-0">
			<table class="w-full min-w-[36rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						<th class="px-4 py-3 font-medium">Observed</th>
						<th class="px-4 py-3 font-medium">Reading</th>
						<th class="px-4 py-3 text-right font-medium">Value</th>
						<th class="px-4 py-3 font-medium">Classroom</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each data.recent as c, i (i)}
						<tr>
							<td class="px-4 py-2.5 whitespace-nowrap">
								{dateTime.format(new Date(c.observedAt))}
							</td>
							<td class="px-4 py-2.5">
								<a
									class="link"
									href={resolve('/classrooms/[classroom]/[parameter]', {
										classroom: c.classroomId,
										parameter: c.parameterId
									})}>{c.parameterLabel}</a
								>
							</td>
							<td class="px-4 py-2.5 text-right font-mono text-ink tabular-nums">
								{formatValue(c.value, c.unit, c.valueType)}
							</td>
							<td class="px-4 py-2.5">
								<a
									class="hover:text-ink"
									href={resolve('/classrooms/[classroom]', { classroom: c.classroomId })}
									>{c.classroomName}</a
								>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
</PublicShell>
