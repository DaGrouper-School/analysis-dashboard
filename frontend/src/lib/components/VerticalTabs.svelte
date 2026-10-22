<script lang="ts" module>
	export interface TabItem<T extends string = string> {
		id: T;
		label: string;
		icon: string;
		/** A small number after the label, such as how many items the tab holds. */
		count?: number;
		/** Shown instead of the count when the tab needs attention. */
		badge?: { text: string; title: string };
	}
</script>

<script lang="ts" generics="Id extends string">
	import { untrack, type Snippet } from 'svelte';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * Tabs down the left on wide screens, a scrolling row on narrow ones. The open tab is kept in
	 * the URL (`?tab=`), so a reload or a shared link opens the same one. The panel is `children`.
	 */
	let {
		tabs,
		active = $bindable(),
		label,
		children
	}: {
		tabs: readonly TabItem<Id>[];
		active?: Id;
		/** Names the tab list for screen readers. */
		label: string;
		children: Snippet;
	} = $props();

	// The first tab to show: the bound one, else the one named in the URL, else the first.
	untrack(() => {
		if (active && tabs.some((t) => t.id === active)) return;
		active = (tabs.find((t) => t.id === page.url.searchParams.get('tab')) ?? tabs[0]).id;
	});

	function open(id: Id) {
		active = id;
		const url = new URL(page.url.href);
		url.searchParams.set('tab', id);
		// The current page's own URL, with only the tab changed.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		replaceState(url, page.state);
	}

	/** Arrow keys move between tabs, as in any tab list. */
	function keys(event: KeyboardEvent) {
		const step = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[event.key];
		if (!step) return;
		event.preventDefault();
		const i = tabs.findIndex((t) => t.id === active);
		open(tabs[(i + step + tabs.length) % tabs.length].id);
		document.getElementById(`tab-${active}`)?.focus();
	}
</script>

<div class="grid gap-6 lg:grid-cols-[13rem_minmax(0,1fr)]">
	<div
		class="flex gap-1 overflow-x-auto border-b border-line lg:sticky lg:top-28 lg:flex-col lg:self-start lg:border-r lg:border-b-0 lg:pr-3"
		role="tablist"
		aria-label={label}
		aria-orientation="vertical"
		tabindex="-1"
		onkeydown={keys}
	>
		{#each tabs as item (item.id)}
			{@const selected = item.id === active}
			<button
				type="button"
				role="tab"
				id="tab-{item.id}"
				aria-selected={selected}
				aria-controls="panel-{item.id}"
				tabindex={selected ? 0 : -1}
				class={[
					'-mb-px flex shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-left text-sm whitespace-nowrap transition lg:-mr-[13px] lg:mb-0 lg:border-r-2 lg:border-b-0',
					selected
						? 'border-ink font-medium text-ink'
						: 'border-transparent text-muted hover:text-ink'
				]}
				onclick={() => open(item.id)}
			>
				<Icon name={item.icon} fallback="tag" class="size-4" />
				<span class="flex-1">{item.label}</span>
				{#if item.badge}
					<span
						class="rounded-full bg-orange/15 px-1.5 text-xs text-orange"
						title={item.badge.title}>{item.badge.text}</span
					>
				{:else if item.count !== undefined}
					<span class="text-xs text-muted">{item.count}</span>
				{/if}
			</button>
		{/each}
	</div>

	<div
		class="min-w-0"
		role="tabpanel"
		id="panel-{active}"
		aria-labelledby="tab-{active}"
		tabindex="-1"
	>
		{@render children()}
	</div>
</div>
