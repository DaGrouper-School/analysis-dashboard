<script lang="ts">
	/** "Sample data" wherever sample data is shown (AP-21), while the site content says so. */
	let { show }: { show: boolean } = $props();
</script>

{#if show}
	<span class="chip shrink-0 border-orange/50 text-xs text-orange">Sample data</span>
{/if}
