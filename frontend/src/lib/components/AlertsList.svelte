<script lang="ts" module>
	import type { Device, DeviceAlert, MemberConsent, Parameter } from '#lib/domain/types.ts';

	/** An alert from a device's latest reading, with the device and when it was read. */
	export type ClassroomAlert = DeviceAlert & {
		device: Pick<Device, 'id' | 'name'>;
		observedAt: string;
	};
</script>

<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { valueHref } from '#lib/site.ts';

	/**
	 * What needs a classroom's members (AP-08): each device's active alerts, each opening its
	 * value's page, then members waiting for guardian consent. The same on the dashboard and on
	 * the classroom's Alerts tab.
	 */
	let {
		alerts,
		waiting,
		parameters,
		classroomId,
		onconsent,
		class: className = ''
	}: {
		alerts: ClassroomAlert[];
		waiting: MemberConsent[];
		parameters: Map<string, Pick<Parameter, 'label'>>;
		classroomId: string;
		/** Opens where paper consent is recorded. */
		onconsent: () => void;
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
