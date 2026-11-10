<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { InstitutionOverview } from '#lib/domain/types.ts';
	import { formatCoordinates, initials } from '#lib/station-now.ts';

	/**
	 * Who runs a classroom and what its community does (AP-43): the institution's photo, or a
	 * placeholder with its initials, its name, kit and hub, and a short history. Shared by the
	 * Explore station page, the classroom profile and the Classrooms tab.
	 */
	let {
		overview,
		actions,
		class: className = ''
	}: {
		overview: InstitutionOverview;
		/** Extra buttons beside "Learn more", such as "Edit overview" for the head. */
		actions?: Snippet;
		class?: string;
	} = $props();

	const hub = $derived(overview.hub ?? overview.name);
	// Only a web address opens from "Learn more", never another kind of link.
	const website = $derived(
		overview.website && /^https?:\/\//i.test(overview.website) ? overview.website : null
	);
	const kitLine = $derived(
		[overview.deviceTypes.join(', '), `Hub: ${hub}`].filter(Boolean).join(' · ')
	);
</script>

<section class={['card flex flex-col', className]} aria-labelledby="institution-heading">
	<div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
		<div>
			<h2 id="institution-heading" class="text-xl font-medium">Institution overview</h2>
			<p class="text-xs text-muted">What our community does here</p>
		</div>
		{#if overview.location}
			<p class="font-mono text-xs text-muted">{formatCoordinates(overview.location)}</p>
		{/if}
	</div>

	{#if overview.photoUrl}
		<img
			src={overview.photoUrl}
			alt={overview.photoAlt}
			class="mt-5 aspect-video w-full rounded-md border border-line object-cover"
			loading="lazy"
		/>
	{:else}
		<div
			class="mt-5 flex aspect-video w-full flex-col items-center justify-center gap-1 rounded-md border border-line bg-gradient-to-br from-forest to-raised"
			role="img"
			aria-label="No photo of {overview.name} yet"
		>
			<span class="text-4xl font-medium tracking-wider text-teal" aria-hidden="true"
				>{initials(overview.name)}</span
			>
			<span class="text-xs text-muted" aria-hidden="true">Photo of the institution</span>
		</div>
	{/if}

	<h3 class="mt-5 text-lg font-medium">{overview.name}</h3>
	{#if kitLine}<p class="text-xs text-muted">{kitLine}</p>{/if}
	{#if overview.history}<p class="mt-3 text-sm leading-relaxed">{overview.history}</p>{/if}

	{#if website || actions}
		<div class="mt-auto flex flex-wrap gap-3 pt-5">
			{#if website}
				<!-- The institution's own site, not an app route. -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class="btn btn-ghost flex-1 rounded-full"
					href={website}
					target="_blank"
					rel="noopener noreferrer">Learn more</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{/if}
			{@render actions?.()}
		</div>
	{/if}
</section>
