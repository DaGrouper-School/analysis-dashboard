<script lang="ts">
	let { inverted = false, compact = false }: { inverted?: boolean; compact?: boolean } = $props();
</script>

<span class="inline-flex items-center gap-2.5">
	<span class="grid size-9 place-items-center rounded-full bg-lime text-canvas" aria-hidden="true">
		<svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2">
			<path d="M3 13c3-6 9-8 18-7-1 9-5 14-12 14a6 6 0 0 1-6-7Z" />
			<path d="M8 16c2-3 5-5 9-6" />
		</svg>
	</span>
	<span
		class={[
			'text-lg font-semibold tracking-tight',
			inverted ? 'text-white' : 'text-ink',
			compact && 'hidden sm:inline'
		]}
	>
		DaGrouper
		<span class={inverted ? 'text-white/60' : 'text-muted'}>Analysis</span>
	</span>
</span>
