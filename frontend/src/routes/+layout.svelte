<script lang="ts">
	import './layout.css';
	import { invalidateAll, onNavigate } from '$app/navigation';
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

	// Crossfade between pages, and between sensors on the same page, instead of swapping the
	// content in one frame. Browsers without view transitions, and people who ask for reduced
	// motion, get the plain swap.
	onNavigate((navigation) => {
		if (!document.startViewTransition) return;
		// Switching classrooms on an Explore station keeps the page and shows a skeleton while
		// the next one loads (AP-43), so it is not faded as a whole.
		const station = '/explore/[classroom]';
		if (navigation.from?.route.id === station && navigation.to?.route.id === station) return;
		if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		return new Promise((done) => {
			document.startViewTransition(async () => {
				done();
				await navigation.complete;
			});
		});
	});

	/**
	 * In-page links ("View domains", "Join") glide to their section instead of jumping, then move
	 * focus there for keyboard and screen reader users. Reduced motion jumps as before.
	 */
	function smoothHashLink(event: MouseEvent) {
		if (event.defaultPrevented || event.button !== 0) return;
		if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
		const link = (event.target as Element | null)?.closest?.('a[href^="#"]');
		const id = link?.getAttribute('href')?.slice(1);
		const target = id ? document.getElementById(decodeURIComponent(id)) : null;
		if (!target) return;
		event.preventDefault();
		const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
		target.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
		if (!target.matches('a[href], button, input, select, textarea, [tabindex]')) {
			target.setAttribute('tabindex', '-1');
		}
		target.focus({ preventScroll: true });
	}
</script>

<svelte:document onclickcapture={smoothHashLink} />

<svelte:head>
	<!-- The site logo. The SVG follows the browser's light or dark theme; the PNG is a fallback. -->
	<link rel="icon" href={favicon} type="image/svg+xml" />
	<link rel="icon" href={faviconPng} type="image/png" sizes="32x32" />
	<link rel="apple-touch-icon" href={appleTouchIcon} />
</svelte:head>

{@render children()}
