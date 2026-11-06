import { resolve } from '$app/paths';
import type { SiteSection } from '#lib/domain/types.ts';

/**
 * A public section's page, or its "coming soon" page while it is not built (AP-20). A section's
 * `href` is a path inside the app, so the base path is put in front of it here.
 */
export function sectionHref(section: SiteSection) {
	if (!section.href) return resolve('/soon/[id]', { id: section.id });
	return resolve('/') + section.href.replace(/^\//, '');
}
