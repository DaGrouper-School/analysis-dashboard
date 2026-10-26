import { resolve } from '$app/paths';
import type { SiteSection } from '#lib/domain/types.ts';

/** A public section's page, or its "coming soon" page while it is not built (AP-20). */
export function sectionHref(section: SiteSection) {
	return section.href ?? resolve('/soon/[id]', { id: section.id });
}
