<script lang="ts">
	/** Classroom onboarding progress: classroom, then lab and site profile, then devices. */
	let { current }: { current: 1 | 2 | 3 } = $props();

	const steps = ['Classroom', 'Lab and site profile', 'Devices'];
</script>

<ol class="mb-6 flex max-w-xl gap-2" aria-label="Classroom setup">
	{#each steps as label, i (label)}
		<li class="flex-1">
			<div class={['h-1.5 rounded-full', i + 1 <= current ? 'bg-ink' : 'bg-line']}></div>
			<p
				class={['mt-2 text-xs font-medium', i + 1 === current ? 'text-ink' : 'text-muted']}
				aria-current={i + 1 === current ? 'step' : undefined}
			>
				Step {i + 1} · {label}
			</p>
		</li>
	{/each}
</ol>
