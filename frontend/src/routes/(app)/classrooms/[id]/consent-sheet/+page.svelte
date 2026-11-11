<script lang="ts">
	import { sectionQuery } from '#lib/sensor-selection.ts';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const sheet = $derived(data.sheet);
	const content = $derived(sheet.content);
	/** A sheet for one minor has their name on it; otherwise the line is left blank. */
	const minor = $derived(sheet.minors.find((m) => m.userId === data.memberId) ?? null);
	const sheetPath = $derived(
		resolve('/(app)/classrooms/[id]/consent-sheet', { id: data.classroomId })
	);

	function pick(userId: string) {
		// The path is resolved; only the query is added.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(userId ? `${sheetPath}?member=${encodeURIComponent(userId)}` : sheetPath, {
			replaceState: true
		});
	}
</script>

<svelte:head><title>Consent sheet · {sheet.classroomName} · DaGrouper Analysis</title></svelte:head>

<div class="print:hidden">
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		href={resolve('/(app)/dashboard') + sectionQuery(data.classroomId, 'people')}
		class="text-sm text-muted hover:text-ink">← {sheet.classroomName}</a
	>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
	<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="font-display text-3xl">Guardian consent sheet</h1>
			<p class="mt-1 max-w-2xl text-sm text-muted">
				Print it, hand it to the guardian, and record it in the classroom's Consent tab when it
				comes back signed.
			</p>
		</div>
		<div class="flex flex-wrap items-end gap-3">
			{#if sheet.minors.length}
				<div>
					<label class="label" for="sheet-member">Name on the sheet</label>
					<select
						id="sheet-member"
						class="input w-auto pr-9"
						value={minor?.userId ?? ''}
						onchange={(e) => pick(e.currentTarget.value)}
					>
						<option value="">Leave blank</option>
						{#each sheet.minors as m (m.userId)}
							<option value={m.userId}>{m.fullName}</option>
						{/each}
					</select>
				</div>
			{/if}
			<button class="btn btn-primary" onclick={() => window.print()}
				><Icon name="printer" class="size-4" /> Print</button
			>
		</div>
	</div>
</div>

<!-- A sheet of paper: black on white in either theme, and on its own when printed. -->
<article
	class="consent-sheet mx-auto mt-8 max-w-[210mm] rounded-md bg-white p-8 text-[15px] leading-relaxed text-black shadow-lg sm:p-12 print:mt-0 print:max-w-none print:rounded-none print:p-0 print:shadow-none"
	aria-label="Consent sheet"
>
	<header class="border-b border-black/30 pb-4">
		<p class="text-xs tracking-wide text-black/70 uppercase">{sheet.institutionName}</p>
		<h2 class="mt-1 text-2xl font-semibold text-black">{content.title}</h2>
		<p class="mt-1 text-sm text-black/70">{sheet.classroomName}</p>
	</header>

	<p class="mt-5">{content.intro}</p>

	{#each content.sections as section (section.heading)}
		<h3 class="mt-5 font-semibold text-black">{section.heading}</h3>
		<p class="mt-1">{section.body}</p>
	{/each}

	<div class="mt-7 grid gap-5">
		<div>
			<p class="text-sm text-black/70">{content.contributorField}</p>
			<p class="mt-1 min-h-8 border-b border-black/50 pb-1 font-medium">{minor?.fullName ?? ''}</p>
		</div>
	</div>

	<ul class="mt-6 space-y-3" aria-label="Decision">
		{#each [content.choices.give, content.choices.withdraw] as choice (choice)}
			<li class="flex items-start gap-3">
				<span class="mt-0.5 size-5 shrink-0 border-2 border-black" aria-hidden="true"></span>
				<span>{choice}</span>
			</li>
		{/each}
	</ul>

	<div class="mt-7 grid gap-6 sm:grid-cols-2 print:grid-cols-2">
		{#each content.fields as field (field)}
			<div>
				<p class="min-h-10 border-b border-black/50"></p>
				<p class="mt-1 text-sm text-black/70">{field}</p>
			</div>
		{/each}
	</div>

	<p class="mt-8 border-t border-dashed border-black/40 pt-3 text-xs text-black/70">
		{content.officeNote}
	</p>
</article>
