<script lang="ts">
	import type { Snippet } from 'svelte';
	import Collapse from '#lib/components/Collapse.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { InstitutionOverview } from '#lib/domain/types.ts';
	import { formatCoordinates } from '#lib/features/map/geo.ts';
	import { initials } from '#lib/utils/format.ts';

	/**
	 * Who runs a classroom and what its community does (AP-43): the institution's photo, or a
	 * placeholder with its initials, its name, kit and hub, and a short history. Shared by the
	 * Explore station page, the classroom profile and the Classrooms tab. On a phone it is an
	 * accordion, closed until opened; on a wide screen it is always open.
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

	let open = $state(false);

	const hub = $derived(overview.hub ?? overview.name);
	// Only a web address opens from "Learn more", never another kind of link.
	const website = $derived(
		overview.website && /^https?:\/\//i.test(overview.website) ? overview.website : null
	);
	// Without kit or a hub there is nothing to say yet, so the card shows just the name.
	const kitLine = $derived(
		[
			overview.deviceTypes.join(', '),
			overview.deviceTypes.length || overview.hub ? `Hub: ${hub}` : ''
		]
			.filter(Boolean)
			.join(' · ')
	);
</script>

<section class={['card flex flex-col', className]} aria-labelledby="institution-heading">
	<div class="relative flex items-start justify-between gap-x-4 gap-y-1 lg:flex-wrap">
		<div>
			<h2 id="institution-heading" class="text-xl font-medium">Institution overview</h2>
			<p class="text-xs text-muted">What our community does here</p>
		</div>
		{#if overview.location}
			<p class="font-mono text-xs text-muted max-lg:hidden">
				{formatCoordinates(overview.location)}
			</p>
		{/if}
		<!-- On a phone the whole header opens and closes the card. -->
		<button
			type="button"
			class="grid size-8 shrink-0 place-items-center rounded-full text-muted after:absolute after:inset-0 hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none lg:hidden"
			aria-expanded={open}
			aria-controls="institution-body"
			aria-label={open ? 'Close the institution overview' : 'Open the institution overview'}
			onclick={() => (open = !open)}
		>
			<Icon name="chevron" class="size-5 transition-transform {open ? 'rotate-180' : ''}" />
		</button>
	</div>

	<Collapse
		id="institution-body"
		{open}
		openFrom="(min-width: 64rem)"
		fill
		class="flex flex-1 flex-col"
	>
		{#if overview.location}
			<p class="mt-3 font-mono text-xs text-muted lg:hidden">
				{formatCoordinates(overview.location)}
			</p>
		{/if}

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
		{#if overview.history}
			<!-- Clamped on wide screens, so a long history never makes the row taller. -->
			<p class="mt-3 text-sm leading-relaxed lg:line-clamp-5" title={overview.history}>
				{overview.history}
			</p>
		{/if}

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
	</Collapse>
</section>
