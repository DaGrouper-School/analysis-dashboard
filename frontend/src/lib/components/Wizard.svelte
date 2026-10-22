<script lang="ts">
	import type { Snippet } from 'svelte';
	import { tick } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import StepTrack from '#lib/components/StepTrack.svelte';

	/**
	 * A form split into steps: the step track on top, one step at a time, Back and Next below. Next
	 * runs `check` for the step and stays put while it fails; the last step submits. Enter in a field
	 * moves on like Next. `children` draws the step at `current`.
	 */
	let {
		steps,
		current = $bindable(0),
		check = () => true,
		onsubmit,
		submitLabel,
		busy = false,
		busyLabel = 'Saving…',
		label,
		extra,
		children
	}: {
		steps: { label: string; icon: string }[];
		current?: number;
		/** Whether the step at `step` is ready to leave. Set its errors here. */
		check?: (step: number) => boolean;
		onsubmit: (event: SubmitEvent) => unknown;
		submitLabel: string;
		busy?: boolean;
		busyLabel?: string;
		/** Names the form for screen readers. */
		label: string;
		/** More buttons beside Back, such as Cancel. */
		extra?: Snippet;
		children: Snippet;
	} = $props();

	let body = $state<HTMLElement>();
	const last = $derived(current === steps.length - 1);

	async function go(step: number) {
		current = step;
		await tick();
		body?.focus({ preventScroll: true });
		body?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
	}

	function next() {
		if (check(current)) go(current + 1);
	}

	function submit(event: SubmitEvent) {
		if (!last) {
			event.preventDefault();
			next();
			return;
		}
		if (!check(current)) {
			event.preventDefault();
			return;
		}
		onsubmit(event);
	}
</script>

<form class="space-y-6" aria-label={label} novalidate onsubmit={submit}>
	<div style:--accent="var(--color-teal)">
		<StepTrack {steps} {current} />
		<p class="mt-2 text-xs text-muted sm:hidden">
			Step {current + 1} of {steps.length} · {steps[current].label}
		</p>
	</div>

	<div class="scroll-mt-28 outline-none" tabindex="-1" bind:this={body}>
		{@render children()}
	</div>

	<div class="flex flex-wrap items-center gap-2 border-t border-line pt-4">
		{#if current > 0}
			<button type="button" class="btn btn-ghost" onclick={() => go(current - 1)}>
				<Icon name="back" class="size-4" /> Back
			</button>
		{/if}
		{@render extra?.()}
		<span class="flex-1"></span>
		{#if last}
			<button class="btn btn-primary" disabled={busy}>{busy ? busyLabel : submitLabel}</button>
		{:else}
			<button type="button" class="btn btn-primary" onclick={next}>
				Next: {steps[current + 1].label}
				<Icon name="arrow" class="size-4" />
			</button>
		{/if}
	</div>
</form>
