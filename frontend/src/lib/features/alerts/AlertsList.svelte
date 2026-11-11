<script lang="ts" module>
	import type {
		Device,
		DeviceAlert,
		MemberConsent,
		Parameter,
		ReleaseReview
	} from '#lib/domain/types.ts';

	/** An alert from a device's latest reading, with the device and when it was read. */
	export type ClassroomAlert = DeviceAlert & {
		device: Pick<Device, 'id' | 'name'>;
		observedAt: string;
	};
</script>

<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { valueHref } from '#lib/utils/site.ts';

	/**
	 * What needs a classroom's members (AP-08): each device's active alerts, each opening its
	 * value's page, then data waiting for review before it is public, then members waiting for
	 * guardian consent. The same on the dashboard and on the classroom's Alerts tab.
	 */
	let {
		alerts,
		waiting,
		release = null,
		parameters,
		classroomId,
		onconsent,
		onrelease,
		class: className = ''
	}: {
		alerts: ClassroomAlert[];
		waiting: MemberConsent[];
		/** Readings waiting for review, for whoever may release them; null for everyone else. */
		release?: ReleaseReview | null;
		parameters: Map<string, Pick<Parameter, 'label'>>;
		classroomId: string;
		/** Opens where paper consent is recorded. */
		onconsent: () => void;
		/** Opens the review of what waits, and its release. */
		onrelease?: () => void;
		class?: string;
	} = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<ul class={['space-y-3', className]}>
	{#each alerts as alert (`${alert.device.id}|${alert.parameterId}`)}
		{@const label = parameters.get(alert.parameterId)?.label ?? alert.parameterId}
		<li class="card flex flex-wrap items-center gap-4 p-5">
			<div class="min-w-0 flex-1">
				<p class="flex flex-wrap items-center gap-3 text-xs text-muted">
					<span
						class={[
							'chip',
							alert.level === 'warning'
								? 'border-danger/60 text-danger'
								: 'border-orange/60 text-orange'
						]}
						><Icon name="alert" class="size-3.5" />
						{alert.level === 'warning' ? 'Warning' : 'Caution'}</span
					>
					{dateTime.format(new Date(alert.observedAt))}
				</p>
				<p class="mt-2 font-medium text-ink">{alert.title} at {alert.device.name}</p>
				<p class="mt-1 text-sm">{alert.message}</p>
			</div>
			<!-- Opens the value's page on this device, with its alert levels. valueHref resolves. -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a class="btn btn-ghost" href={valueHref(classroomId, alert.parameterId, alert.device.id)}
				>Open {label}</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		</li>
	{/each}
	{#if release && onrelease}
		<li class="card flex flex-wrap items-center gap-4 p-5">
			<div class="min-w-0 flex-1">
				<p class="text-xs text-muted">
					<span class="chip"><Icon name="clipboard" class="size-3.5" /> Review</span>
				</p>
				<p class="mt-2 font-medium text-ink">
					{release.pending}
					{release.pending === 1 ? 'reading is' : 'readings are'} waiting for review
				</p>
				<p class="mt-1 text-sm">
					The public sees them only once they are released{release.releasedThrough
						? `. Released so far up to ${dateTime.format(new Date(release.releasedThrough))}.`
						: '. Nothing is released yet.'}
				</p>
			</div>
			<button type="button" class="btn btn-ghost" aria-haspopup="dialog" onclick={onrelease}
				>Review and release</button
			>
		</li>
	{/if}
	{#each waiting as member (member.userId)}
		<li class="card flex flex-wrap items-center gap-4 p-5">
			<div class="min-w-0 flex-1">
				<p class="text-xs text-muted">
					<span class="chip"><Icon name="user" class="size-3.5" /> Consent</span>
				</p>
				<p class="mt-2 font-medium text-ink">
					{member.fullName} is waiting for guardian consent
				</p>
				<p class="mt-1 text-sm">
					They can't send measurements until a parent or guardian consents{member.guardianName
						? `. The link went to ${member.guardianName.replace(/\.$/, '')}.`
						: '.'}
				</p>
			</div>
			<button type="button" class="btn btn-ghost" onclick={onconsent}>Record paper consent</button>
		</li>
	{/each}
</ul>
