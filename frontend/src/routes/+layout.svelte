<script lang="ts">
	import './layout.css';
	import { invalidateAll } from '$app/navigation';
	import favicon from '#lib/assets/favicon.svg';
	import { watchOtherTabs } from '#lib/data/mock-db.ts';
	import { isMockDataSource } from '#lib/services/index.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	// Every account in this browser shares the mock data: when another tab records a reading or
	// changes a classroom, reload this page's data so it shows up here too.
	$effect(() => (isMockDataSource ? watchOtherTabs(() => invalidateAll()) : undefined));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
