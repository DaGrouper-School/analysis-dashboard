import { resolve } from '$app/paths';
import type { SiteSection } from '#lib/domain/types.ts';
import { sensorQuery } from '#lib/utils/sensor-selection.ts';

/**
 * A public section's page, or its "coming soon" page while it is not built (AP-20). A section's
 * `href` is a path inside the app, so the base path is put in front of it here.
 */
export function sectionHref(section: SiteSection) {
	if (!section.href) return resolve('/soon/[id]', { id: section.id });
	return resolve('/') + section.href.replace(/^\//, '');
}

/**
 * A classroom's Domains page (members only), on a device and, with `domainId`, one domain open.
 */
export function domainsHref(classroomId: string, deviceId?: string | null, domainId?: string) {
	const query = sensorQuery(classroomId, deviceId ?? undefined);
	return (
		resolve('/(app)/dashboard/domains') +
		query +
		(domainId ? `&domain=${encodeURIComponent(domainId)}` : '')
	);
}

/**
 * A classroom's page on the Classrooms page (members only), open on a section (its tab) when
 * `section` is given: Measurements, Labs, People, Settings and the rest.
 */
export function classroomsHref(classroomId: string, section?: string, deviceId?: string | null) {
	return (
		resolve('/(app)/dashboard/classrooms') +
		sensorQuery(classroomId, deviceId ?? undefined) +
		(section ? `&tab=${encodeURIComponent(section)}` : '')
	);
}

/**
 * A value's own page in a classroom (AP-23), under its Domains page, opened on one device's
 * readings when `deviceId` is given. Tiles, cards, domain readings and alerts all link here, for
 * the classroom's members only: the page is in the app.
 */
export function valueHref(classroomId: string, parameterId: string, deviceId?: string | null) {
	const path = resolve('/(app)/dashboard/domains/[classroom]/[parameter]', {
		classroom: classroomId,
		parameter: parameterId
	});
	return deviceId ? `${path}?device=${encodeURIComponent(deviceId)}` : path;
}
