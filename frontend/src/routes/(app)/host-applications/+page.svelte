<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { sectionQuery } from '#lib/utils/sensor-selection.ts';
	import { resolve } from '$app/paths';
	import { HOST_STATUS_LABELS } from '#lib/domain/hosting.ts';
	import type { HostApplication } from '#lib/domain/types.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	/** The back office for host applications (AP-30): read, accept or decline. */
	let { data }: PageProps = $props();

	// svelte-ignore state_referenced_locally
	let applications = $state<HostApplication[]>(data.allowed ? data.applications : []);
	let busy = $state<string | null>(null);
	let error = $state<{ id: string; message: string } | null>(null);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const waiting = $derived(applications.filter((a) => a.status === 'submitted').length);

	const answer = (a: HostApplication, id: string) => {
		const value = a.answers[id];
		return value === 'yes' ? 'Yes' : value === 'no' ? 'No' : value || '—';
	};

	async function decide(id: string, accept: boolean) {
		busy = id;
		error = null;
		try {
			const updated = await services.hosting.decide(id, accept);
			applications = applications.map((a) => (a.id === id ? updated : a));
		} catch (e) {
			error = { id, message: errorMessage(e) };
		} finally {
			busy = null;
		}
	}
</script>

<svelte:head><title>Host applications · DaGrouper Analysis</title></svelte:head>

<h1 class="font-display text-3xl sm:text-4xl">Host applications</h1>

{#if !data.allowed}
	<div class="card mt-8 max-w-xl">
		<p class="text-sm text-muted">Platform admins review applications to host a classroom.</p>
	</div>
{:else}
	<p class="mt-1 text-muted" aria-live="polite">
		{applications.length
			? `${waiting} waiting for review, ${applications.length} in all.`
			: 'No applications yet.'}
	</p>
	<ul class="mt-6 space-y-4">
		{#each applications as a (a.id)}
			<li class="card">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div class="min-w-0">
						<h2 class="text-lg font-semibold text-ink">{a.institutionName}</h2>
						<p class="text-sm text-muted">
							{a.site.placeName}, {a.site.country} · sent {dateTime.format(new Date(a.submittedAt))}
						</p>
					</div>
					<span
						class={[
							'chip shrink-0',
							a.status === 'accepted' && 'border-lime/50 text-lime',
							a.status === 'declined' && 'border-danger/50 text-danger'
						]}>{HOST_STATUS_LABELS[a.status]}</span
					>
				</div>
				<dl class="mt-4 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[14rem_1fr]">
					<dt class="text-muted">Contact</dt>
					<dd>
						{a.contactName}, {a.contactRole} ·
						<a class="link" href="mailto:{a.contactEmail}">{a.contactEmail}</a>{a.contactPhone
							? ` · ${a.contactPhone}`
							: ''}
					</dd>
					{#each data.questions as q (q.id)}
						<dt class="text-muted">{q.label}</dt>
						<dd>{answer(a, q.id)}</dd>
					{/each}
					<dt class="text-muted">Ecosystems</dt>
					<dd>{a.site.ecosystems.join(', ')}</dd>
					<dt class="text-muted">Learners</dt>
					<dd>{a.site.learners}</dd>
					<dt class="text-muted">Wants to study</dt>
					<dd>{a.site.studyGoals}</dd>
				</dl>
				<div class="mt-4 flex flex-wrap items-center gap-2">
					{#if a.status === 'submitted'}
						<button
							type="button"
							class="btn btn-primary"
							disabled={busy !== null}
							onclick={() => decide(a.id, true)}><Icon name="check" class="size-4" /> Accept</button
						>
						<button
							type="button"
							class="btn btn-ghost"
							disabled={busy !== null}
							onclick={() => decide(a.id, false)}
							><Icon name="close" class="size-4" /> Decline</button
						>
					{:else if a.status === 'accepted' && a.classroomId}
						<!-- eslint-disable svelte/no-navigation-without-resolve -->
						<a
							class="link text-sm"
							href={resolve('/(app)/dashboard/classrooms') + sectionQuery(a.classroomId, 'people')}
							>Open its classroom</a
						>
						<!-- eslint-enable svelte/no-navigation-without-resolve -->
					{:else if a.status === 'accepted'}
						<!-- eslint-disable svelte/no-navigation-without-resolve -->
						<a
							class="btn btn-primary"
							href={`${resolve('/(app)/classrooms/new')}?application=${encodeURIComponent(a.id)}`}
							><Icon name="plus" class="size-4" /> Create its classroom</a
						>
						<!-- eslint-enable svelte/no-navigation-without-resolve -->
					{/if}
					{#if error?.id === a.id}<p class="field-error" role="alert">{error.message}</p>{/if}
				</div>
			</li>
		{/each}
	</ul>
{/if}
