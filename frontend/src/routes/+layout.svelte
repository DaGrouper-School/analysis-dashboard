<script lang="ts">
	import './layout.css';
	import { invalidateAll } from '$app/navigation';
	import appleTouchIcon from '#lib/assets/apple-touch-icon.png';
	import favicon from '#lib/assets/favicon.svg';
	import faviconPng from '#lib/assets/favicon-32.png';
	import { watchOtherTabs } from '#lib/data/mock-db.ts';
	import { isMockDataSource } from '#lib/services/index.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	// Every account in this browser shares the mock data: when another tab records a reading or
	// changes a classroom, reload this page's data so it shows up here too.
	$effect(() => (isMockDataSource ? watchOtherTabs(() => invalidateAll()) : undefined));
</script>

<svelte:head>
	<!-- The site logo. The SVG follows the browser's light or dark theme; the PNG is a fallback. -->
	<link rel="icon" href={favicon} type="image/svg+xml" />
	<link rel="icon" href={faviconPng} type="image/png" sizes="32x32" />
	<link rel="apple-touch-icon" href={appleTouchIcon} />
</svelte:head>

{@render children()}
