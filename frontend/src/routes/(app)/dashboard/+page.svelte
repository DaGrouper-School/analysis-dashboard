<script lang="ts">
	import Dashboard from './Dashboard.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<Dashboard {data} view="overview" />
