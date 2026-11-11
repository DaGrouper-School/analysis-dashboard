import { resolve } from '$app/paths';
import { page } from '$app/state';
import type { SiteSection } from '#lib/domain/types.ts';

/**
 * A public section's page, or its "coming soon" page while it is not built (AP-20). A section's
 * `href` is a path inside the app, so the base path is put in front of it here.
 */
export function sectionHref(section: SiteSection) {
	if (!section.href) return resolve('/soon/[id]', { id: section.id });
	return resolve('/') + section.href.replace(/^\//, '');
}

/**
 * A value's own page in a classroom (AP-23), opened on one device's readings when `deviceId` is
 * given. Tiles, cards, domain readings and alerts all link here. Anyone signed in opens it in the
 * app, visitors on the public site.
 */
export function valueHref(classroomId: string, parameterId: string, deviceId?: string | null) {
	const params = { classroom: classroomId, parameter: parameterId };
	const path = page.data.user
		? resolve('/(app)/dashboard/[classroom]/[parameter]', params)
		: resolve('/classrooms/[classroom]/[parameter]', params);
	return deviceId ? `${path}?device=${encodeURIComponent(deviceId)}` : path;
}
