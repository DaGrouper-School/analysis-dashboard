<script lang="ts">
	import { invalidate } from '$app/navigation';
	import type { ClassroomSetupStatus } from '#lib/domain/types.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * Whether the host agreement is signed, and with it whether the institution shows publicly. A
	 * platform admin marks it once the signed agreement arrives; the institution head sees where it is.
	 */
	let {
		classroomId,
		status,
		editable
	}: { classroomId: string; status: ClassroomSetupStatus; editable: boolean } = $props();

	let busy = $state(false);
	let error = $state('');

	async function mark(signed: boolean) {
		busy = true;
		error = '';
		try {
			await services.classrooms.setHostAgreement(classroomId, signed);
			await invalidate('app:classroom');
		} catch (e) {
			error = errorMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

<section class="card max-w-2xl space-y-3" aria-labelledby="host-agreement-heading">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="host-agreement-heading" class="text-lg font-semibold">Host agreement</h2>
			<p class="mt-1 text-sm text-muted">
				{status.hostAgreementSigned
					? 'Signed. The institution overview shows on the public site, if the head allowed it.'
					: 'Not signed yet. Until it is, only members see the institution overview.'}
			</p>
		</div>
		<span class={['chip', status.hostAgreementSigned && 'text-lime']}>
			{status.hostAgreementSigned ? 'Signed' : 'Waiting'}
		</span>
	</div>
	{#if !status.institutionDone}
		<p class="text-sm text-muted">The institution head hasn't finished the institution setup.</p>
	{/if}
	{#if editable}
		<button
			type="button"
			class={['btn', status.hostAgreementSigned ? 'btn-ghost' : 'btn-primary']}
			disabled={busy}
			onclick={() => mark(!status.hostAgreementSigned)}
		>
			{busy ? 'Saving…' : status.hostAgreementSigned ? 'Mark as not signed' : 'Mark as signed'}
		</button>
	{/if}
	{#if error}<p class="field-error" role="alert">{error}</p>{/if}
</section>
