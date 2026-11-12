<script lang="ts">
	import type { Snippet } from 'svelte';
	import AccordionCard from '#lib/components/AccordionCard.svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * "Alerts and messages" at the top of the dashboard: how many things need the user, what they
	 * are (`children`), and a link to the classroom's Alerts tab. It can be collapsed, and the
	 * count stays in view. On a phone it starts closed, with the title, count and toggle on one
	 * line and the link inside; on a wider screen this browser remembers whether it was open.
	 */
	let { count, href, children }: { count: number; href: string; children: Snippet } = $props();

	const KEY = 'dagrouper:alerts-panel-open';
	const PHONE = '(max-width: 639px)';
	let open = $state(remembered());
	function remembered() {
		if (typeof matchMedia === 'function' && matchMedia(PHONE).matches) return false;
		try {
			return localStorage.getItem(KEY) !== 'false';
		} catch {
			return true;
		}
	}
	function remember(open: boolean) {
		// A phone always starts closed, so only a wider screen's choice is remembered.
		if (matchMedia(PHONE).matches) return;
		try {
			localStorage.setItem(KEY, String(open));
		} catch {
			// Not remembered in this browser; the panel still opens and closes.
		}
	}
</script>

<AccordionCard
	id="alerts-panel"
	label="alerts and messages"
	bind:open
	ontoggle={remember}
	class="mb-8 p-5"
	bodyClass="pt-4"
>
	{#snippet heading()}
		<Icon name="alert" class="size-5 text-orange" />
		Alerts and messages
		<span
			class="grid min-w-6 place-items-center rounded-full bg-orange/15 px-1.5 text-sm text-orange"
			aria-label="{count} need you">{count}</span
		>
	{/snippet}
	{#snippet aside()}
		<!-- Callers pass a resolved path. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a class="link text-sm max-sm:hidden" {href}>Open alerts</a>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/snippet}
	{@render children()}
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a class="link mt-4 inline-block text-sm sm:hidden" {href}>Open alerts</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</AccordionCard>
