<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * Reminds an institution head who chose "Finish later" that their institution setup (AP-40) is
	 * not done, with a button back to the step they left.
	 */
	let { classroomId, institutionName }: { classroomId: string; institutionName: string } = $props();
</script>

<section
	class="card flex flex-wrap items-center justify-between gap-4 border-teal/40"
	aria-labelledby="setup-reminder-heading"
>
	<div class="flex min-w-0 items-start gap-3">
		<Icon name="info" class="mt-0.5 size-5 shrink-0 text-teal" />
		<div>
			<h2 id="setup-reminder-heading" class="font-semibold">Finish setting up {institutionName}</h2>
			<p class="text-sm text-muted">
				Add your institution's photo and story so members know who they are joining.
			</p>
		</div>
	</div>
	<a href={resolve('/(app)/classrooms/[id]/setup', { id: classroomId })} class="btn btn-primary">
		Continue setup <Icon name="arrow" class="size-4" />
	</a>
</section>
