import type { ClassroomPermissions, ClassroomRole, ClassroomSharing } from './types';

/**
 * Who may do what in a classroom (see "Roles and Permissions"). One rule for every page and the
 * backend: the viewer's role in the classroom, the platform admin flag, and the sharing settings
 * the institution head chose. A visitor (no viewer) may do nothing but look at released data.
 */

/** A new classroom's sharing: public once released by hand, by the head or a facilitator. */
export const DEFAULT_SHARING: ClassroomSharing = {
	showPublic: true,
	review: 'manual',
	autoReleaseDays: 2,
	releasers: 'head_and_facilitators',
	facilitatorsInvite: true,
	facilitatorsManageDevices: true
};

/** The longest a classroom may wait before readings release themselves. */
export const MAX_AUTO_RELEASE_DAYS = 30;

const ALL_ROLES: ClassroomRole[] = [
	'institution_head',
	'facilitator',
	'community_scientist',
	'researcher'
];

export function permissionsFor(
	viewer: { platformAdmin: boolean } | null,
	role: ClassroomRole | null,
	sharing: ClassroomSharing
): ClassroomPermissions {
	const admin = !!viewer?.platformAdmin;
	const member = role !== null;
	const head = role === 'institution_head';
	const facilitator = role === 'facilitator';
	const focal = head || facilitator;
	const manageDevices = admin || head || (facilitator && sharing.facilitatorsManageDevices);
	return {
		view: admin || member,
		seeUnreleased: member,
		openValuePage: member,
		seeCode: admin || focal,
		seeSiteCodes: member,
		seeEmails: admin || focal,
		invite: admin
			? ALL_ROLES
			: head
				? ['facilitator', 'community_scientist', 'researcher']
				: facilitator && sharing.facilitatorsInvite
					? ['community_scientist']
					: [],
		manageMembers: admin || head,
		edit: admin || head,
		manageSharing: admin || head,
		viewSharing: admin || focal,
		// Minors' guardian consent (AP-38): the people who hand out and collect the paper sheets.
		// Not platform admins: they are not at the classroom.
		manageConsent: focal,
		// Alert levels (AP-08): a classroom's focal people set one device's; only the head sets a
		// sensor type's for the whole classroom.
		manageDeviceAlerts: focal,
		manageTypeAlerts: head,
		manageDevices,
		// Researchers only read.
		recordReadings: admin || focal || role === 'community_scientist',
		releaseData: head || (facilitator && sharing.releasers === 'head_and_facilitators'),
		setUpInstitution: head,
		markHostAgreement: admin
	};
}

/**
 * Readings observed up to here are released, by hand or, with auto-release, by age. Null while
 * none are. It ignores whether the classroom shows its data publicly: see `publicCutoff`.
 */
export function releaseCutoff(
	classroom: { releasedThrough: string | null; sharing: ClassroomSharing },
	now: number
): string | null {
	const manual = classroom.releasedThrough;
	if (classroom.sharing.review !== 'auto') return manual;
	const aged = new Date(now - classroom.sharing.autoReleaseDays * 86_400_000).toISOString();
	return manual && manual > aged ? manual : aged;
}

/** What visitors see up to: the release cutoff, or nothing while the classroom does not share. */
export function publicCutoff(
	classroom: { releasedThrough: string | null; sharing: ClassroomSharing },
	now: number
): string | null {
	return classroom.sharing.showPublic ? releaseCutoff(classroom, now) : null;
}
