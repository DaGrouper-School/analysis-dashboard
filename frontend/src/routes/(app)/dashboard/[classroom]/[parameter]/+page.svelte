<script lang="ts">
	import ValuePage from '#lib/components/ValuePage.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<ValuePage {data} inApp />
