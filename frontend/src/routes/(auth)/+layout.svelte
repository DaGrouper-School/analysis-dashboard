<script lang="ts">
	import AuthShell from '#lib/components/AuthShell.svelte';

	let { data, children } = $props();
</script>

<AuthShell markers={data.markers}>{@render children()}</AuthShell>
