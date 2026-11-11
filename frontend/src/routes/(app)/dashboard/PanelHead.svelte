<script lang="ts">
	/** A classroom section's heading and the line under it. */
	let { title, sub }: { title: string; sub: string } = $props();
</script>

<h2 class="font-display text-2xl">{title}</h2>
<p class="mt-1 max-w-2xl text-sm text-muted">{sub}</p>
