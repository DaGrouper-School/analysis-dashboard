<script lang="ts">
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
		links: { href: string; label: string; current: boolean }[];
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
			'whitespace-nowrap underline-offset-[6px] hover:text-ink',
			link.current ? 'text-ink underline decoration-teal decoration-2' : 'text-muted',
			extra
		]}>{link.label}</a
	>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
{/each}
