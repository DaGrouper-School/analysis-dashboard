<script lang="ts">
	import ColorIcon from '#lib/components/ColorIcon.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { Snippet } from 'svelte';
	import AccordionCard from '#lib/components/AccordionCard.svelte';
	import type { InstitutionOverview } from '#lib/domain/types.ts';
	import { formatCoordinates } from '#lib/features/map/geo.ts';
	import { initials } from '#lib/utils/format.ts';

	/**
	 * Who runs a classroom and what its community does (AP-43): the institution's photo, or a
	 * placeholder with its initials, its name, hub, a short history and its kinds of device. Shared by the
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
	const showHub = $derived(!!overview.hub || overview.deviceTypes.length > 0);
</script>

{#snippet factLabel(icon: string, label: string)}
	<p class="flex items-center gap-1.5 text-xs font-medium text-muted">
		<Icon name={icon} class="size-4" />{label}
	</p>
{/snippet}

<AccordionCard
	id="institution"
	label="institution overview"
	bind:open
	openFrom="lg"
	fill
	class={className}
	bodyClass="flex flex-1 flex-col"
	sub="What our community does here"
>
	{#snippet heading()}Institution overview{/snippet}
	{#snippet aside()}
		{#if overview.location}
			<p class="font-mono text-xs text-muted max-lg:hidden">
				{formatCoordinates(overview.location)}
			</p>
		{/if}
	{/snippet}
	{#if overview.location}
		<p class="mt-3 font-mono text-xs text-muted lg:hidden">
			{formatCoordinates(overview.location)}
		</p>
	{/if}

	<!-- On a wide screen the photo takes half the width and the words sit beside it, so the card
	     fits under the map without scrolling. -->
	<div class="mt-5 flex flex-1 flex-col lg:grid lg:grid-cols-2 lg:gap-5">
		<!-- On a wide screen the photo fills its half, as tall as the words beside it. -->
		<div class="lg:relative lg:min-h-40">
			{#if overview.photoUrl}
				<img
					src={overview.photoUrl}
					alt={overview.photoAlt}
					class="aspect-video w-full rounded-md border border-line object-cover lg:absolute lg:inset-0 lg:aspect-auto lg:h-full"
					loading="lazy"
				/>
			{:else}
				<div
					class="flex aspect-video w-full flex-col items-center justify-center gap-1 rounded-md border border-line bg-gradient-to-br from-forest to-raised lg:absolute lg:inset-0 lg:aspect-auto lg:h-full"
					role="img"
					aria-label="No photo of {overview.name} yet"
				>
					<span class="text-4xl font-medium tracking-wider text-teal" aria-hidden="true"
						>{initials(overview.name)}</span
					>
					<span class="text-xs text-muted" aria-hidden="true">Photo of the institution</span>
				</div>
			{/if}
		</div>

		<div class="flex flex-1 flex-col lg:h-full">
			<h3 class="mt-5 text-lg font-medium lg:mt-0">{overview.name}</h3>
			{#if showHub}
				<div class="mt-3">
					{@render factLabel('institution', 'Hub')}
					<p class="mt-1 text-sm">{hub}</p>
				</div>
			{/if}
			{#if overview.history}
				<!-- Clamped on wide screens, so a long history never makes the row taller. -->
				<p class="mt-3 text-sm leading-relaxed lg:line-clamp-3" title={overview.history}>
					{overview.history}
				</p>
			{/if}
			{#if overview.deviceTypes.length}
				<div class="mt-3">
					{@render factLabel('device', 'Devices')}
					<!-- Each kind of device in its registry icon and colour, as everywhere else. -->
					<ul class="mt-1.5 flex flex-wrap gap-x-4 gap-y-2 text-sm">
						{#each overview.deviceTypes as type (type.id)}
							<li class="flex items-center gap-2">
								<ColorIcon name={type.icon} color={type.color} />{type.label}
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			{#if website || actions}
				<div class="mt-auto flex flex-wrap gap-3 pt-5">
					{#if website}
						<!-- The institution's own site, not an app route. -->
						<!-- eslint-disable svelte/no-navigation-without-resolve -->
						<a class="btn btn-ghost flex-1" href={website} target="_blank" rel="noopener noreferrer"
							><Icon name="info" class="size-4" /> Learn more</a
						>
						<!-- eslint-enable svelte/no-navigation-without-resolve -->
					{/if}
					{@render actions?.()}
				</div>
			{/if}
		</div>
	</div>
</AccordionCard>
