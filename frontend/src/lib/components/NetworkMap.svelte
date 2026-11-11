<script lang="ts">
	import { resolve } from '$app/paths';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import type { ClassroomStage, PublicClassroom } from '#lib/domain/types.ts';

	/**
	 * Every Global Classroom on the globe, colored by stage, with a list beside it (AP-20, AP-21).
	 * Picking one shows it on the globe and offers its station.
	 */
	let { classrooms, stages }: { classrooms: PublicClassroom[]; stages: ClassroomStage[] } =
		$props();

	const byId = $derived(new Map(stages.map((s) => [s.id, s])));
	let selectedId = $state('');
	const selected = $derived(classrooms.find((c) => c.id === selectedId) ?? null);
	const markers = $derived<GlobeMarker[]>(
		classrooms
			.filter((c) => c.location)
			.map((c) => ({
				id: c.id,
				lat: c.location!.lat,
				lng: c.location!.lng,
				color: byId.get(c.stageId)?.color ?? '#8e98a3',
				label: `${c.name} · ${byId.get(c.stageId)?.label ?? c.stageId}`
			}))
	);
</script>

<div class="grid gap-6 lg:grid-cols-[1.4fr_1fr]">
	<div class="card relative h-96 overflow-hidden p-0">
		<Globe {markers} bind:selectedId ariaLabel="Globe of Global Classrooms by stage" />
	</div>
	<div class="space-y-4">
		<ul class="space-y-2" aria-label="Global Classrooms">
			{#each classrooms as classroom (classroom.id)}
				<li>
					<button
						type="button"
						class={[
							'card flex w-full items-start justify-between gap-3 p-4 text-left transition',
							selectedId === classroom.id ? 'border-ink' : 'hover:border-line-strong'
						]}
						aria-pressed={selectedId === classroom.id}
						onclick={() => (selectedId = classroom.id)}
					>
						<span class="min-w-0">
							<span class="block font-medium text-ink">{classroom.name}</span>
							<span class="block text-sm text-muted">
								{[classroom.placeName, classroom.country].filter(Boolean).join(', ') ||
									classroom.institutionName}
							</span>
						</span>
						<StageBadge stage={byId.get(classroom.stageId)} stageId={classroom.stageId} />
					</button>
				</li>
			{/each}
		</ul>
		{#if selected}
			<a
				class="btn btn-primary w-full"
				href={resolve('/classrooms/[classroom]', { classroom: selected.id })}
				>Open {selected.name}'s station</a
			>
		{/if}
		<dl class="space-y-2 text-sm" aria-label="Stages">
			{#each stages as stage (stage.id)}
				<div class="flex gap-2">
					<dt class="shrink-0"><StageBadge {stage} stageId={stage.id} /></dt>
					<dd class="text-muted">{stage.description}</dd>
				</div>
			{/each}
		</dl>
	</div>
</div>
