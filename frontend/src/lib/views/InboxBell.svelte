<script lang="ts" module>
	import type { InvitationPreview, MemberConsent, ReleaseReview } from '#lib/domain/types.ts';
	import type { ClassroomAlert } from '#lib/features/alerts/AlertsList.svelte';

	/** What needs the user in one of their classrooms. */
	export interface ClassroomInbox {
		classroom: { id: string; name: string };
		alerts: ClassroomAlert[];
		waiting: MemberConsent[];
		release: ReleaseReview | null;
	}
</script>

<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import Icon from '#lib/components/Icon.svelte';
	import type { Parameter } from '#lib/domain/types.ts';
	import AlertsList from '#lib/features/alerts/AlertsList.svelte';
	import PendingInvitations from '#lib/features/classroom/PendingInvitations.svelte';
	import ReleaseModal from '#lib/features/classroom/ReleaseModal.svelte';
	import { classroomsHref } from '#lib/utils/site.ts';

	/**
	 * Alerts and messages, in the top bar on every page: a bell with how many things need the
	 * user, opening a panel with each classroom's alerts, data waiting for release and members
	 * waiting for consent, then invitations. A dropdown on a wide screen, the full width on a phone.
	 */
	let {
		inbox,
		invitations,
		parameters
	}: {
		inbox: ClassroomInbox[];
		invitations: InvitationPreview[];
		parameters: Parameter[];
	} = $props();

	const labels = $derived(new Map(parameters.map((p) => [p.id, p])));
	const count = $derived(
		inbox.reduce((n, c) => n + c.alerts.length + c.waiting.length + (c.release ? 1 : 0), 0) +
			invitations.length
	);

	let open = $state(false);
	let root = $state<HTMLElement>();
	// Closes on a click outside, on Escape, and when a link in it opens another page.
	function outside(event: MouseEvent) {
		if (open && root && !root.contains(event.target as Node)) open = false;
	}
	function escape(event: KeyboardEvent) {
		if (open && event.key === 'Escape') open = false;
	}
	afterNavigate(() => (open = false));

	// Reviewing and releasing one classroom's data, from its message.
	let reviewing = $state(false);
	let releaseFor = $state<ClassroomInbox>();
	function release(item: ClassroomInbox) {
		releaseFor = item;
		reviewing = true;
		open = false;
	}
	function consent(classroomId: string) {
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(classroomsHref(classroomId, 'people'));
	}
</script>

<svelte:window onclick={outside} onkeydown={escape} />

<div class="sm:relative" bind:this={root}>
	<button
		type="button"
		class="relative grid size-8 shrink-0 place-items-center rounded-full border border-line-strong text-muted transition hover:border-ink hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none sm:size-9"
		aria-expanded={open}
		aria-controls="inbox-panel"
		aria-label={count ? `Alerts and messages, ${count} need you` : 'Alerts and messages'}
		onclick={() => (open = !open)}
	>
		<Icon name="bell" class="size-4" />
		{#if count}
			<span
				class="absolute -top-1 -right-1 grid h-4.5 min-w-4.5 place-items-center rounded-full bg-orange px-1 text-[0.65rem] font-semibold text-canvas"
				aria-hidden="true">{count}</span
			>
		{/if}
	</button>
	{#if open}
		<div
			id="inbox-panel"
			class="card absolute top-full right-0 z-30 mt-2 max-h-[75dvh] w-[30rem] overflow-y-auto p-4 shadow-xl max-sm:inset-x-3 max-sm:w-auto"
			role="region"
			aria-label="Alerts and messages"
		>
			<h2 class="flex items-center gap-2 font-semibold">
				<Icon name="alert" class="size-5 text-orange" /> Alerts and messages
			</h2>
			{#if !count}
				<p class="mt-3 text-sm text-muted">Nothing needs you right now.</p>
			{/if}
			{#each inbox as item (item.classroom.id)}
				<section class="mt-5" aria-label={item.classroom.name}>
					<h3 class="text-sm font-medium text-muted">{item.classroom.name}</h3>
					<AlertsList
						class="mt-2"
						stacked
						alerts={item.alerts}
						waiting={item.waiting}
						release={item.release}
						parameters={labels}
						classroomId={item.classroom.id}
						onconsent={() => consent(item.classroom.id)}
						onrelease={() => release(item)}
					/>
				</section>
			{/each}
			{#if invitations.length}
				<div class="mt-5"><PendingInvitations {invitations} /></div>
			{/if}
		</div>
	{/if}
</div>

{#if releaseFor?.release}
	<ReleaseModal
		bind:open={reviewing}
		classroomId={releaseFor.classroom.id}
		review={releaseFor.release}
	/>
{/if}
