<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/** The steps of a modal flow, with the current one lit and the ones before it checked. */
	let {
		steps,
		current,
		failed = false
	}: {
		steps: { label: string; icon: string }[];
		current: number;
		/** The current step stopped on an error. */
		failed?: boolean;
	} = $props();
</script>

<ol class="flex items-center gap-1.5" aria-label="Steps">
	{#each steps as step, i (step.label)}
		{@const state = i < current ? 'done' : i === current ? (failed ? 'failed' : 'now') : 'next'}
		<li
			class="flex min-w-0 items-center gap-1.5 {i < steps.length - 1 ? 'flex-1' : ''}"
			aria-current={state === 'now' ? 'step' : undefined}
		>
			<span
				class={[
					'grid size-7 shrink-0 place-items-center rounded-full border transition-colors duration-300',
					state === 'done' && 'border-(--accent) bg-(--accent) text-canvas',
					state === 'now' && 'border-(--accent) text-(--accent)',
					state === 'failed' && 'border-danger text-danger',
					state === 'next' && 'border-line-strong text-muted'
				]}
			>
				<Icon
					name={state === 'done' ? 'check' : state === 'failed' ? 'alert' : step.icon}
					class="size-3.5"
				/>
			</span>
			<span
				class={[
					'hidden truncate text-xs sm:block',
					state === 'now' || state === 'failed' ? 'font-medium text-ink' : 'text-muted'
				]}>{step.label}</span
			>
			<span class="sr-only"
				>{state === 'done' ? '(done)' : state === 'failed' ? '(failed)' : ''}</span
			>
			{#if i < steps.length - 1}
				<span
					class={[
						'h-px min-w-3 flex-1 transition-colors duration-300',
						i < current ? 'bg-(--accent)' : 'bg-line-strong'
					]}
					aria-hidden="true"
				></span>
			{/if}
		</li>
	{/each}
</ol>
