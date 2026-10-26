<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Snippet } from 'svelte';
	import Logo from '#lib/components/Logo.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import type { SiteContent, User } from '#lib/domain/types.ts';
	import { nextRoute } from '#lib/routing.ts';
	import { isMockDataSource } from '#lib/services/index.ts';
	import { sectionHref } from '#lib/site.ts';

	/**
	 * The public site's frame (AP-20): every section in the top bar, a language switch and
	 * Sign in, and a footer with the programme line and the policy pages. No sign-in needed.
	 */
	let { content, user, children }: { content: SiteContent; user: User | null; children: Snippet } =
		$props();

	let menuOpen = $state(false);

	// Signed-in people go on to their dashboard, or to the next step of sign-up.
	const account = $derived(
		user
			? {
					href: resolve(nextRoute(user)),
					label: user.onboardingCompleted ? 'Go to dashboard' : 'Finish signing up'
				}
			: { href: resolve('/(auth)/signup'), label: 'Sign in' }
	);
</script>

{#snippet sectionLinks(extra: string)}
	{#each content.sections as section (section.id)}
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
		<a href={sectionHref(section)} class={['text-muted hover:text-ink', extra]}>{section.label}</a>
	{/each}
{/snippet}

<div class="flex min-h-dvh flex-col">
	<header class="sticky top-0 z-20 border-b border-line bg-canvas/85 backdrop-blur">
		<div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-3 sm:px-6">
			<a href={resolve('/')} aria-label="Home"><Logo compact /></a>
			<nav aria-label="Sections" class="ml-4 hidden items-center gap-5 text-sm lg:flex">
				{@render sectionLinks('')}
			</nav>
			<div class="ml-auto flex items-center gap-2">
				<label class="sr-only" for="site-language">Language</label>
				<select id="site-language" class="input w-auto py-1.5 pr-9 text-sm" value="en">
					{#each content.languages as language (language.code)}
						<option value={language.code} disabled={!language.ready}
							>{language.label}{language.ready ? '' : ' (coming soon)'}</option
						>
					{/each}
				</select>
				<ThemeToggle />
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a href={account.href} class="btn btn-primary hidden sm:inline-flex">{account.label}</a>
				<button
					type="button"
					class="btn btn-ghost lg:hidden"
					aria-expanded={menuOpen}
					aria-controls="site-menu"
					onclick={() => (menuOpen = !menuOpen)}>Menu</button
				>
			</div>
		</div>
		{#if menuOpen}
			<nav
				id="site-menu"
				aria-label="Sections"
				class="mx-auto flex max-w-6xl flex-col gap-3 border-t border-line px-4 py-4 sm:px-6 lg:hidden"
			>
				{@render sectionLinks('py-1')}
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a href={account.href} class="btn btn-primary mt-2 sm:hidden">{account.label}</a>
			</nav>
		{/if}
		{#if isMockDataSource}
			<p class="border-t border-line bg-raised py-1 text-center text-xs text-muted">
				Demo mode: data is mock and saved only in this browser.
			</p>
		{/if}
	</header>

	<main class="mx-auto w-full max-w-6xl flex-1 px-4 py-10 sm:px-6">
		{@render children()}
	</main>

	<footer class="border-t border-line">
		<div
			class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 text-sm sm:flex-row sm:items-start sm:justify-between sm:px-6"
		>
			<div class="max-w-md space-y-1">
				<p class="text-ink">{content.programme}</p>
				<p class="text-muted">{content.oceanDecade}</p>
			</div>
			<nav aria-label="Policies" class="flex flex-wrap gap-x-5 gap-y-2">
				{#each content.policies as policy (policy.id)}
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={sectionHref(policy)} class="text-muted hover:text-ink">{policy.label}</a>
				{/each}
			</nav>
		</div>
	</footer>
</div>
