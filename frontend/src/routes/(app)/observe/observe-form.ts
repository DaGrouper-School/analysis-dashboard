import type {
	Device,
	DeviceTypeDefinition,
	DomainDefinition,
	ObservationTemplate
} from '#lib/domain/types.ts';
import { FALLBACK_DOMAIN_ICON } from '#lib/utils/icons.ts';
import { deviceTypeLabel } from '#lib/utils/labels.ts';

/** Each step's title and icon in the observation form. */
export const STEP_INFO = {
	what: { label: 'What', icon: 'clipboard' },
	device: { label: 'Device', icon: 'sensor' },
	values: { label: 'Values', icon: 'readings' },
	review: { label: 'Review', icon: 'review' }
};
export type StepId = keyof typeof STEP_INFO;

/**
 * The form's steps. Opened for one sensor, only its values and the review; otherwise the device
 * step shows only when the template is read from a device.
 */
export function observeSteps(locked: boolean, readFromDevice: boolean): StepId[] {
	if (locked) return ['values', 'review'];
	return ['what', ...(readFromDevice ? (['device'] as const) : []), 'values', 'review'];
}

/** Where the person's GPS stands: not asked yet, finding a fix, a fix, or none to be had. */
export type GpsState = 'idle' | 'locating' | 'fix' | 'none';

/** A template group: one domain and the templates under it. */
export interface TemplateGroup {
	id: string | null;
	label: string;
	icon: string;
	templates: ObservationTemplate[];
}

/** Templates by domain, in the registry's domain order; ones with no domain go last. */
export function templateGroups(
	domains: DomainDefinition[],
	templates: ObservationTemplate[]
): TemplateGroup[] {
	const known = new Set(domains.map((d) => d.id));
	return [
		...domains.map((d) => ({ id: d.id as string | null, label: d.label, icon: d.icon })),
		{ id: null, label: 'Other', icon: FALLBACK_DOMAIN_ICON }
	]
		.map((domain) => ({
			...domain,
			templates: templates.filter((t) =>
				domain.id === null ? !t.domain || !known.has(t.domain) : t.domain === domain.id
			)
		}))
		.filter((g) => g.templates.length);
}

/** `now` as a `datetime-local` value in the browser's time zone. */
export function localNow(now = new Date()) {
	const offset = now.getTimezoneOffset() * 60_000;
	return new Date(now.getTime() - offset).toISOString().slice(0, 16);
}

/** What is wrong with the device step: none picked from several, or none of the type in the lab. */
export function deviceProblem(
	template: ObservationTemplate,
	fitting: Device[],
	device: Device | null,
	deviceTypes: DeviceTypeDefinition[]
): string {
	if (!template.deviceType || device) return '';
	return fitting.length
		? 'Choose the device you read this from.'
		: `No ${deviceTypeLabel(deviceTypes, template.deviceType).toLowerCase()} is registered to this lab.`;
}

/** What is wrong with the observation time: missing, or more than five minutes ahead. */
export function timeProblem(observedAt: string, now = Date.now()): string {
	if (!observedAt) return 'Enter when you observed it.';
	return new Date(observedAt).getTime() > now + 5 * 60_000 ? 'That time is in the future.' : '';
}
