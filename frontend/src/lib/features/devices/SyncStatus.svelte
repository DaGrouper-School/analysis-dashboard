<script lang="ts">
	import type { ObservationTarget, ObservationTemplate } from '#lib/domain/types.ts';
	import { outbox } from '#lib/features/devices/outbox.svelte.ts';
	import { clockMessage, type OutboxStatus } from '#lib/features/devices/outbox.ts';
	import { isMockDataSource } from '#lib/services/index.ts';

	/**
	 * Every observation saved on this device by the signed-in user (AP-16): pending, synced or
	 * failed, each with its own result, clock warnings and a retry.
	 */
	let { templates, targets }: { templates: ObservationTemplate[]; targets: ObservationTarget[] } =
		$props();

	const templateLabel = (id: string) => templates.find((t) => t.id === id)?.label ?? id;
	const labName = (id: string) => targets.find((t) => t.lab.id === id)?.lab.name ?? 'Lab';
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const STATUS: Record<OutboxStatus, { label: string; class: string }> = {
		pending: { label: 'Pending', class: 'border-orange/50 text-orange' },
		synced: { label: 'Synced', class: 'border-lime/50 text-lime' },
		failed: { label: 'Failed', class: 'border-danger/50 text-danger' }
	};

	const pending = $derived(outbox.mine.filter((e) => e.status === 'pending').length);
	const failed = $derived(outbox.mine.filter((e) => e.status === 'failed').length);
</script>

<section id="sync" class="mt-10 scroll-mt-24" aria-labelledby="sync-heading">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h2 id="sync-heading" class="text-xl font-semibold">Sync status</h2>
			<p class="mt-1 text-sm text-muted" aria-live="polite">
				{#if !outbox.online}
					You are offline. Observations are saved on this device and sync when you are back online.
				{:else if outbox.syncing}
					Syncing…
				{:else if pending || failed}
					{[
						pending && `${pending} waiting to sync.`,
						failed && `${failed} failed: retry or discard.`
					]
						.filter(Boolean)
						.join(' ')}
				{:else}
					Everything saved on this device is synced.
				{/if}
			</p>
		</div>
		<div class="flex flex-wrap items-center gap-3">
			{#if isMockDataSource}
				<label class="flex items-center gap-2 text-sm text-muted">
					<input
						type="checkbox"
						class="rounded-sm border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
						checked={outbox.simulatedOffline}
						onchange={(e) => outbox.setSimulatedOffline(e.currentTarget.checked)}
					/>
					Simulate offline
				</label>
			{/if}
			<button
				type="button"
				class="btn btn-ghost"
				disabled={!outbox.online || outbox.syncing || !pending}
				onclick={() => outbox.sync()}>Sync now</button
			>
		</div>
	</div>

	{#if outbox.mine.length}
		<ul class="mt-4 divide-y divide-line rounded-md border border-line">
			{#each outbox.mine as entry (entry.clientId)}
				{@const status = STATUS[entry.status]}
				<li class="p-4">
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div class="min-w-0">
							<p class="font-medium text-ink">{templateLabel(entry.observation.templateId)}</p>
							<p class="text-sm text-muted">
								{labName(entry.observation.labId)} · observed {dateTime.format(
									new Date(entry.observation.observedAt)
								)}
							</p>
						</div>
						<span class={['chip shrink-0', status.class]}>{status.label}</span>
					</div>
					{#if entry.status === 'synced' && entry.lastAttemptAt}
						<p class="mt-2 text-xs text-muted">
							Synced {dateTime.format(new Date(entry.lastAttemptAt))}
						</p>
					{:else if entry.error && !Object.keys(entry.fields).length}
						<p class={['mt-2 text-sm', entry.status === 'failed' ? 'text-danger' : 'text-muted']}>
							{entry.error}
						</p>
					{/if}
					{#if entry.status === 'failed' && Object.keys(entry.fields).length}
						<ul class="mt-2 list-disc pl-5 text-sm text-danger">
							{#each Object.values(entry.fields) as message, i (i)}
								<li>{message}</li>
							{/each}
						</ul>
					{/if}
					{#each entry.clockFlags as flag (flag)}
						<p class="mt-2 text-sm text-orange">{clockMessage(flag, entry.clockOffsetMs)}</p>
					{/each}
					<p class="mt-2 font-mono text-xs break-all text-muted">ID {entry.clientId}</p>
					{#if entry.status === 'failed'}
						<div class="mt-3 flex flex-wrap gap-2">
							<button
								type="button"
								class="btn btn-ghost"
								disabled={!outbox.online || outbox.syncing}
								onclick={() => outbox.retry(entry.clientId)}>Retry</button
							>
							<button
								type="button"
								class="btn btn-ghost"
								onclick={() => outbox.discard(entry.clientId)}>Discard</button
							>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-4 text-sm text-muted">Nothing saved on this device yet.</p>
	{/if}
</section>
