<script lang="ts">
	/** A page's trail back up the site; the last item is the page itself. */
	let { items }: { items: { label: string; href?: string }[] } = $props();
</script>

<nav aria-label="Breadcrumb" class="text-sm text-muted">
	{#each items as item, i (i)}
		{#if i}<span aria-hidden="true" class="px-1">›</span>{/if}
		{#if i === items.length - 1}
			<span class="text-ink" aria-current="page">{item.label}</span>
		{:else if item.href}
			<!-- Callers pass resolved paths. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="hover:text-ink" href={item.href}>{item.label}</a>
		{:else}
			{item.label}
		{/if}
	{/each}
</nav>
