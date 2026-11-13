<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * The site's main links, the same in the public site and the members' app: plain text, the
	 * current page in ink and underlined in teal. `extra` adds classes to each link, such as
	 * the menu's padding.
	 */
	let {
		links,
		extra = ''
	}: {
		/** Resolved paths; `current` marks the page the person is on. */
		links: { href: string; label: string; current: boolean; icon?: string }[];
		extra?: string;
	} = $props();
</script>

{#each links as link (link.href)}
	<!-- Callers pass resolved paths. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		href={link.href}
		aria-current={link.current ? 'page' : undefined}
		class={[
			'inline-flex items-center gap-1.5 whitespace-nowrap underline-offset-[6px] hover:text-ink',
			link.current ? 'text-ink underline decoration-teal decoration-2' : 'text-muted',
			extra
		]}
		>{#if link.icon}<Icon
				name={link.icon}
				class={link.current ? 'size-4 text-teal' : 'size-4'}
			/>{/if}{link.label}</a
	>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
{/each}
