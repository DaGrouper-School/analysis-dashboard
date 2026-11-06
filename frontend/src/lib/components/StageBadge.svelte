<script lang="ts">
	import type { ClassroomStage } from '#lib/domain/types.ts';

	/** A classroom's stage (AP-20): its registry color as a dot, and its name in words. */
	let { stage, stageId }: { stage: ClassroomStage | undefined; stageId: string } = $props();
</script>

<span class="chip gap-1.5">
	<span
		class="size-2 rounded-full"
		style:background={stage?.color ?? 'currentColor'}
		aria-hidden="true"
	></span>
	{stage?.label ?? stageId}
</span>
