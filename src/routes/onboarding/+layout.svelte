<script lang="ts">
	import { page } from '$app/state';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import { isProfileStepDone } from '#lib/routing.ts';

	let { data, children } = $props();

	const steps = $derived([
		{ href: '/onboarding/profile', label: 'Profile & contact', done: isProfileStepDone(data.user) },
		{ href: '/onboarding/address', label: 'Address', done: Boolean(data.user.address) }
	]);
</script>

<AuthShell>
	<ol class="mb-8 flex gap-2" aria-label="Onboarding steps">
		{#each steps as step, i (step.href)}
			{@const current = page.url.pathname === step.href}
			<li class="flex-1">
				<div class={['h-1.5 rounded-full', current || step.done ? 'bg-ink' : 'bg-line']}></div>
				<p
					class={['mt-2 text-xs font-medium', current ? 'text-ink' : 'text-muted']}
					aria-current={current ? 'step' : undefined}
				>
					Step {i + 1} · {step.label}
				</p>
			</li>
		{/each}
	</ol>
	{@render children()}
</AuthShell>
