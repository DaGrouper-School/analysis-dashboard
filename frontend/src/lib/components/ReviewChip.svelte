<script lang="ts">
	import type { ReviewState } from '#lib/domain/types.ts';

	/** Where a core reading stands in review (AP-20), as a chip with a coloured dot. */
	let { state }: { state: ReviewState } = $props();

	const LABEL: Record<ReviewState, string> = {
		released: 'Released',
		in_review: 'Newer data in review',
		not_released: 'Not released yet'
	};
	const TONE: Record<ReviewState, string> = {
		released: 'bg-lime',
		in_review: 'bg-orange',
		not_released: 'bg-muted'
	};
</script>

<span class="chip shrink-0 whitespace-nowrap">
	<span class={['size-2 rounded-full', TONE[state]]} aria-hidden="true"></span>
	{LABEL[state]}
</span>
