<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { invalidate } from '$app/navigation';
	import Select from '#lib/components/Select.svelte';
	import { MAX_AUTO_RELEASE_DAYS } from '#lib/domain/permissions.ts';
	import type { ClassroomSharing, SharingInput } from '#lib/domain/types.ts';
	import { validateSharing } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * How a classroom shares its data and what its facilitators may do (Settings, AP-45): public
	 * or not, how data is released, who releases it, what facilitators manage, how precise public
	 * positions are, and the data terms every download carries (AP-14). The permissions follow
	 * these settings. The institution head changes them; facilitators see them read-only.
	 */
	let {
		classroomId,
		initial,
		editable
	}: { classroomId: string; initial: SharingInput; editable: boolean } = $props();

	// svelte-ignore state_referenced_locally
	let sharing = $state<ClassroomSharing>({ ...initial.sharing });
	// svelte-ignore state_referenced_locally
	let licence = $state(initial.terms.licence);
	// svelte-ignore state_referenced_locally
	let licenceUrl = $state(initial.terms.licenceUrl ?? '');
	// svelte-ignore state_referenced_locally
	let attribution = $state(initial.terms.attribution);
	// svelte-ignore state_referenced_locally
	let restrictions = $state(initial.terms.restrictions.join('\n'));
	// svelte-ignore state_referenced_locally
	let decimals = $state(String(initial.terms.coordinateDecimals ?? 'full'));
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);
	let saved = $state(false);

	const releaserOptions = [
		{ value: 'head_and_facilitators', label: 'The institution head and facilitators' },
		{ value: 'head', label: 'The institution head only' }
	];
	// Rounding a position to fewer decimal places blurs it: about 1 km at 2, 10 km at 1.
	const decimalOptions = [
		{ value: 'full', label: 'Full precision', hint: 'Public pages still round to 2 places' },
		{ value: '4', label: '4 decimal places', hint: 'About 10 m' },
		{ value: '3', label: '3 decimal places', hint: 'About 100 m' },
		{ value: '2', label: '2 decimal places', hint: 'About 1 km' },
		{ value: '1', label: '1 decimal place', hint: 'About 10 km' },
		{ value: '0', label: 'Whole degrees', hint: 'About 100 km' }
	];

	function input(): SharingInput {
		return {
			sharing: { ...sharing, autoReleaseDays: Number(sharing.autoReleaseDays) },
			terms: {
				licence,
				licenceUrl: licenceUrl.trim() || null,
				attribution,
				restrictions: restrictions.split('\n'),
				coordinateDecimals: decimals === 'full' ? null : Number(decimals)
			}
		};
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		const next = input();
		errors = validateSharing(next);
		error = '';
		saved = false;
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await services.classrooms.updateSharing(classroomId, next);
			saved = true;
			await invalidate('app:classroom');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<form class="card max-w-2xl" onsubmit={save} novalidate>
	<h2 class="text-lg font-semibold">Sharing and permissions</h2>
	<p class="mt-1 text-sm text-muted">
		Who sees this classroom's data, and what facilitators can do.
		{#if !editable}Only the institution head can change these.{/if}
	</p>
	<fieldset class="mt-5 space-y-5" disabled={!editable}>
		<label class="flex items-start gap-3">
			<input type="checkbox" class="mt-0.5" bind:checked={sharing.showPublic} />
			<span>
				<span class="font-medium text-ink">Show data to the public</span>
				<span class="block text-sm text-muted"
					>Released readings appear on the public classroom page and the map. Off, visitors see the
					classroom and its devices with no values.</span
				>
			</span>
		</label>

		<div role="radiogroup" aria-labelledby="sharing-review">
			<p id="sharing-review" class="label">Before data is public</p>
			<label class="mt-2 flex items-start gap-3">
				<input type="radio" class="mt-0.5" value="manual" bind:group={sharing.review} />
				<span>
					<span class="font-medium text-ink">Review and release by hand</span>
					<span class="block text-sm text-muted"
						>Whoever releases data is told when readings wait for review.</span
					>
				</span>
			</label>
			<label class="mt-2 flex items-start gap-3">
				<input type="radio" class="mt-0.5" value="auto" bind:group={sharing.review} />
				<span class="flex flex-wrap items-center gap-2">
					<span class="font-medium text-ink">Release by themselves after</span>
					<input
						type="number"
						class={['input w-20', errors.autoReleaseDays && 'input-error']}
						min="1"
						max={MAX_AUTO_RELEASE_DAYS}
						aria-label="Days before readings are released"
						bind:value={sharing.autoReleaseDays}
						disabled={!editable || sharing.review !== 'auto'}
					/>
					<span class="text-muted">days</span>
				</span>
			</label>
			{#if errors.autoReleaseDays}<p class="field-error">{errors.autoReleaseDays}</p>{/if}
		</div>

		<div>
			<label class="label" for="sharing-releasers">Who can release data</label>
			<Select
				id="sharing-releasers"
				options={releaserOptions}
				bind:value={sharing.releasers}
				disabled={!editable}
			/>
		</div>

		<div class="space-y-3">
			<p class="label">Facilitators can</p>
			<label class="flex items-start gap-3">
				<input type="checkbox" class="mt-0.5" bind:checked={sharing.facilitatorsInvite} />
				<span class="text-ink">Invite community scientists</span>
			</label>
			<label class="flex items-start gap-3">
				<input type="checkbox" class="mt-0.5" bind:checked={sharing.facilitatorsManageDevices} />
				<span class="text-ink">Add labs, and register and remove devices</span>
			</label>
		</div>

		<div>
			<label class="label" for="sharing-decimals">Location precision</label>
			<Select
				id="sharing-decimals"
				options={decimalOptions}
				bind:value={decimals}
				error={!!errors.coordinateDecimals}
				disabled={!editable}
			/>
			<p class="mt-1 text-sm text-muted">How far positions are rounded in downloads.</p>
			{#if errors.coordinateDecimals}<p class="field-error">{errors.coordinateDecimals}</p>{/if}
		</div>

		<div class="grid gap-4 sm:grid-cols-2">
			<div>
				<label class="label" for="sharing-licence">Licence</label>
				<input
					id="sharing-licence"
					class={['input', errors.licence && 'input-error']}
					bind:value={licence}
				/>
				{#if errors.licence}<p class="field-error">{errors.licence}</p>{/if}
			</div>
			<div>
				<label class="label" for="sharing-licence-url">Licence link</label>
				<input
					id="sharing-licence-url"
					type="url"
					class={['input', errors.licenceUrl && 'input-error']}
					placeholder="https://"
					bind:value={licenceUrl}
				/>
				{#if errors.licenceUrl}<p class="field-error">{errors.licenceUrl}</p>{/if}
			</div>
		</div>
		<div>
			<label class="label" for="sharing-attribution">How to credit the data</label>
			<textarea
				id="sharing-attribution"
				rows="2"
				class={['input', errors.attribution && 'input-error']}
				bind:value={attribution}></textarea>
			{#if errors.attribution}<p class="field-error">{errors.attribution}</p>{/if}
		</div>
		<div>
			<label class="label" for="sharing-restrictions">Limits on use</label>
			<textarea
				id="sharing-restrictions"
				rows="3"
				class="input"
				placeholder="One per line, or leave empty"
				bind:value={restrictions}></textarea>
		</div>
	</fieldset>

	{#if editable}
		{#if error}<p class="field-error mt-4" role="alert">{error}</p>{/if}
		<div class="mt-5 flex items-center gap-3">
			<button class="btn btn-primary" disabled={saving}>
				<Icon name="check" class="size-4" />
				{saving ? 'Saving…' : 'Save sharing'}
			</button>
			{#if saved}<p class="text-sm text-muted" role="status">Saved.</p>{/if}
		</div>
	{/if}
</form>
