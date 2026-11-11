<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * "Alerts and messages" at the top of the dashboard: how many things need the user, what they
	 * are (`children`), and a link to the classroom's Alerts tab. It can be collapsed; this browser
	 * remembers that, and the count stays in view.
	 */
	let { count, href, children }: { count: number; href: string; children: Snippet } = $props();

	const KEY = 'dagrouper:alerts-panel-open';
	let open = $state(remembered());
	function remembered() {
		try {
			return localStorage.getItem(KEY) !== 'false';
		} catch {
			return true;
		}
	}
	function toggle() {
		open = !open;
		try {
			localStorage.setItem(KEY, String(open));
		} catch {
			// Not remembered in this browser; the panel still opens and closes.
		}
	}
</script>

<section class="card mb-8 p-5" aria-labelledby="alerts-panel-heading">
	<div class="flex flex-wrap items-center gap-3">
		<h2 id="alerts-panel-heading" class="flex items-center gap-2 text-xl font-medium">
			<Icon name="alert" class="size-5 text-orange" />
			Alerts and messages
			<span
				class="grid min-w-6 place-items-center rounded-full bg-orange/15 px-1.5 text-sm text-orange"
				aria-label="{count} need you">{count}</span
			>
		</h2>
		<div class="ml-auto flex items-center gap-2">
			<!-- Callers pass a resolved path. -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a class="link text-sm" {href}>Open alerts</a>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
			<button
				type="button"
				class="grid size-8 place-items-center rounded-full text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
				aria-expanded={open}
				aria-controls="alerts-panel-body"
				aria-label={open ? 'Hide alerts and messages' : 'Show alerts and messages'}
				onclick={toggle}
			>
				<Icon name="chevron" class="size-5 transition-transform {open ? 'rotate-180' : ''}" />
			</button>
		</div>
	</div>
	<div id="alerts-panel-body" class="mt-4" hidden={!open}>{@render children()}</div>
</section>
