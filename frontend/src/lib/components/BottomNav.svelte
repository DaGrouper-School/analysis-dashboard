<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * The app's main links as a bar at the bottom of a phone's screen, each with its icon to the
	 * left of its name, the current one in teal. Wider screens keep the links in the top bar.
	 */
	let {
		links,
		label = 'Main'
	}: {
		/** Resolved paths; `current` marks the page the person is on. */
		links: { href: string; label: string; icon: string; current: boolean }[];
		label?: string;
	} = $props();
</script>

<nav
	aria-label={label}
	class="fixed inset-x-0 bottom-0 z-20 flex border-t border-line bg-canvas/95 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:hidden print:hidden"
>
	{#each links as link (link.href)}
		<!-- Callers pass resolved paths. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a
			href={link.href}
			aria-current={link.current ? 'page' : undefined}
			class={[
				'flex min-w-0 flex-1 items-center justify-center gap-1.5 border-t-2 py-5 text-sm',
				link.current ? 'border-teal text-ink' : 'border-transparent text-muted hover:text-ink'
			]}
		>
			<Icon
				name={link.icon}
				class={['size-5 shrink-0', link.current && 'text-teal'].filter(Boolean).join(' ')}
			/>
			<span class="truncate">{link.label}</span>
		</a>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/each}
</nav>
