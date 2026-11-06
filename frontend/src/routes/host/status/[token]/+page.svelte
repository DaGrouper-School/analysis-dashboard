<script lang="ts">
	import { resolve } from '$app/paths';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import { HOST_STATUS_LABELS } from '#lib/domain/hosting.ts';
	import type { PageProps } from './$types';

	/** Where a host application stands (AP-30), for the person who sent it. */
	let { data }: PageProps = $props();

	const application = $derived(data.application);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<svelte:head><title>{data.content.hosting.title} · {data.content.hero.eyebrow}</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<div class="mx-auto max-w-xl py-10">
		<p class="text-sm text-muted">{data.content.hosting.title}</p>
		{#if !application}
			<h1 class="mt-1 font-display text-3xl">This status link is not valid</h1>
			<p class="mt-3 text-muted">Check that you copied the whole link.</p>
			<a class="btn btn-ghost mt-6" href={resolve('/host')}>Start an application</a>
		{:else}
			<h1 class="mt-1 font-display text-3xl">{application.institutionName}</h1>
			<p class="mt-4 text-sm" role="status">
				Status:
				<span
					class={[
						'chip ml-1',
						application.status === 'accepted' && 'border-lime/50 text-lime',
						application.status === 'declined' && 'border-danger/50 text-danger'
					]}>{HOST_STATUS_LABELS[application.status]}</span
				>
				{#if application.decidedAt}
					<span class="ml-1 text-muted"
						>since {dateTime.format(new Date(application.decidedAt))}</span
					>
				{/if}
			</p>
			{#if application.status === 'submitted'}
				<p class="mt-4">{data.content.hosting.submitted}</p>
			{/if}
			<dl class="card mt-6 grid gap-3 text-sm sm:grid-cols-[9rem_1fr]">
				<dt class="text-muted">Sent</dt>
				<dd>{dateTime.format(new Date(application.submittedAt))}</dd>
				<dt class="text-muted">Contact</dt>
				<dd>{application.contactName}, {application.contactRole}</dd>
				<dt class="text-muted">Site</dt>
				<dd>{application.site.placeName}, {application.site.country}</dd>
				<dt class="text-muted">Ecosystems</dt>
				<dd>{application.site.ecosystems.join(', ')}</dd>
			</dl>
		{/if}
	</div>
</PublicShell>
