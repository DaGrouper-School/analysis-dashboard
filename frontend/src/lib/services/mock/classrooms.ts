import type {
	ClassroomDetail,
	ClassroomRole,
	ClassroomSummary,
	Invitation,
	InvitationPreview,
	MemberConsent,
	ReleaseReview,
	User
} from '#lib/domain/types.ts';
import {
	isValidEmail,
	validateClassroomUpdate,
	validateInstitutionPhoto,
	validateSharing
} from '#lib/domain/validation.ts';
import { DEFAULT_SHARING, permissionsFor, releaseCutoff } from '#lib/domain/permissions.ts';
import { fillConsentSheet, localDay, validatePaperConsent } from '#lib/domain/consent.ts';
import {
	newId,
	readDb,
	updateDb,
	type MockDbState,
	type StoredClassroom,
	type StoredInvitation
} from '#lib/data/mock-db.ts';
import { ServiceError, type ClassroomService } from '../types';
import { setupStatus } from './institution-setup';

const CODE_ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';
const RANK: Record<ClassroomRole, number> = {
	community_scientist: 0,
	researcher: 1,
	facilitator: 2,
	institution_head: 3
};

export function currentUser(state: MockDbState): User {
	const user = state.session && state.users.find((u) => u.id === state.session!.userId);
	if (!user) throw new ServiceError('unauthenticated', 'Please sign in again.');
	return user;
}

export function roleIn(
	state: MockDbState,
	classroomId: string,
	userId: string
): ClassroomRole | null {
	return (
		state.memberships.find((m) => m.classroomId === classroomId && m.userId === userId)?.role ??
		null
	);
}

/**
 * What a viewer (null for a visitor) may do in a classroom: the shared rule in
 * `#lib/domain/permissions.ts`, which the backend must enforce too.
 */
export function canIn(state: MockDbState, user: User | null, classroomId: string) {
	const classroom = state.classrooms.find((c) => c.id === classroomId);
	const role = user ? roleIn(state, classroomId, user.id) : null;
	return {
		role,
		can: permissionsFor(user, role, classroom?.sharing ?? DEFAULT_SHARING)
	};
}

/** The platform's starting terms, credited to the new classroom. */
function termsFor(state: MockDbState, name: string, institution: string) {
	const terms = structuredClone(state.defaultDataTerms);
	terms.attribution = terms.attribution
		.replaceAll('{classroom}', name)
		.replaceAll('{institution}', institution);
	return terms;
}

function generateCode(state: MockDbState): string {
	const taken = new Set(state.samSources.map((s) => s.sourceId));
	for (;;) {
		let code = 'DGC-';
		for (let i = 0; i < 6; i++) {
			code += CODE_ALPHABET[Math.floor(Math.random() * CODE_ALPHABET.length)];
		}
		if (!taken.has(code)) return code;
	}
}

function summarize(state: MockDbState, user: User, classroom: StoredClassroom): ClassroomSummary {
	const { role: myRole, can } = canIn(state, user, classroom.id);
	return {
		id: classroom.id,
		name: classroom.name,
		institutionName: classroom.institutionName,
		code: can.seeCode ? classroom.code : null,
		setup: classroom.setup,
		createdAt: classroom.createdAt,
		myRole,
		canRecord: myRole !== null && can.recordReadings,
		memberCount: state.memberships.filter((m) => m.classroomId === classroom.id).length
	};
}

function toInvitation(state: MockDbState, invitation: StoredInvitation): Invitation {
	return {
		id: invitation.id,
		classroomId: invitation.classroomId,
		email: invitation.email,
		role: invitation.role,
		status: invitation.status,
		invitedByName: state.users.find((u) => u.id === invitation.invitedBy)?.fullName ?? 'Someone',
		createdAt: invitation.createdAt,
		token: invitation.token
	};
}

function toPreview(state: MockDbState, invitation: StoredInvitation): InvitationPreview {
	const classroom = state.classrooms.find((c) => c.id === invitation.classroomId);
	if (!classroom) throw new ServiceError('not_found', 'This invitation is no longer valid.');
	return {
		token: invitation.token,
		classroomName: classroom.name,
		institutionName: classroom.institutionName,
		role: invitation.role,
		email: invitation.email,
		status: invitation.status,
		invitedByName: toInvitation(state, invitation).invitedByName
	};
}

/** A classroom's minor and where their guardian's consent stands (AP-38). */
function toMemberConsent(state: MockDbState, minor: User): MemberConsent {
	const stored = state.guardianConsents.find((c) => c.userId === minor.id);
	return {
		userId: minor.id,
		fullName: minor.fullName,
		status: minor.consent,
		method: stored?.method ?? null,
		guardianName: stored?.guardianName || null,
		signedOn: stored?.signedOn ?? null,
		decidedAt: stored?.decidedAt ?? null,
		recordedByName: stored?.recordedBy
			? (state.users.find((u) => u.id === stored.recordedBy)?.fullName ?? null)
			: null
	};
}

function minorsIn(state: MockDbState, classroomId: string): User[] {
	const ids = new Set(
		state.memberships.filter((m) => m.classroomId === classroomId).map((m) => m.userId)
	);
	return state.users
		.filter((u) => ids.has(u.id) && u.minor)
		.sort((a, b) => a.fullName.localeCompare(b.fullName));
}

/**
 * What waits for review in a classroom: every reading and field observation observed after the
 * release cutoff, by source, oldest first.
 */
export function reviewOf(
	state: MockDbState,
	classroom: StoredClassroom,
	now: number
): ReleaseReview {
	const through = releaseCutoff(classroom, now);
	const labIds = new Set(state.labs.filter((l) => l.classroomId === classroom.id).map((l) => l.id));
	const waiting = [
		...state.readings
			.filter((r) => labIds.has(r.labId))
			.map((r) => ({ at: r.observedAt, sourceId: r.deviceId })),
		...state.observations
			.filter((o) => labIds.has(o.labId))
			.map((o) => ({ at: o.observedAt, sourceId: null }))
	].filter((r) => through === null || r.at > through);
	const sources = new Map<string, ReleaseReview['sources'][number]>();
	for (const r of waiting) {
		const key = r.sourceId ?? 'field';
		const device = r.sourceId ? state.devices.find((d) => d.id === r.sourceId) : null;
		const source = sources.get(key) ?? {
			id: key,
			name: device?.name ?? null,
			kind: r.sourceId ? ('device' as const) : ('field' as const),
			count: 0,
			first: r.at,
			last: r.at
		};
		source.count++;
		if (r.at < source.first) source.first = r.at;
		if (r.at > source.last) source.last = r.at;
		sources.set(key, source);
	}
	return {
		releasedThrough: through,
		auto: classroom.sharing.review === 'auto',
		showPublic: classroom.sharing.showPublic,
		pending: waiting.length,
		sources: [...sources.values()].sort((a, b) => a.first.localeCompare(b.first))
	};
}

/** Loads a classroom the user may see. Hidden classrooms read as not found. */
export function visibleClassroom(state: MockDbState, user: User, classroomId: string) {
	const classroom = state.classrooms.find((c) => c.id === classroomId);
	const { role, can } = canIn(state, user, classroomId);
	if (!classroom || !can.view) throw new ServiceError('not_found', 'Classroom not found.');
	return { classroom, role, can };
}

export function createMockClassrooms({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): Omit<
	ClassroomService,
	'institutionSetupForm' | 'institutionSetup' | 'saveInstitutionSetup' | 'setHostAgreement'
> {
	const iso = () => new Date(now()).toISOString();

	function createInvitation(
		state: MockDbState,
		classroomId: string,
		email: string,
		role: ClassroomRole,
		invitedBy: string
	): StoredInvitation {
		// Re-inviting replaces the pending invitation, like a resend.
		for (const old of state.invitations) {
			if (old.classroomId === classroomId && old.email === email && old.status === 'pending') {
				old.status = 'revoked';
			}
		}
		const invitation: StoredInvitation = {
			id: newId('inv'),
			token: newId('tok'),
			classroomId,
			email,
			role,
			status: 'pending',
			invitedBy,
			createdAt: iso(),
			acceptedBy: null
		};
		state.invitations.push(invitation);
		return invitation;
	}

	return {
		async listMine() {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			return state.classrooms
				.filter((c) => user.platformAdmin || roleIn(state, c.id, user.id))
				.map((c) => summarize(state, user, c));
		},

		async get(classroomId): Promise<ClassroomDetail> {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const { classroom, can } = visibleClassroom(state, user, classroomId);
			const members = state.memberships
				.filter((m) => m.classroomId === classroom.id)
				.map((m) => {
					const member = state.users.find((u) => u.id === m.userId);
					return {
						userId: m.userId,
						fullName: member?.fullName ?? 'Unknown',
						email: can.seeEmails ? (member?.email ?? null) : null,
						role: m.role,
						joinedAt: m.joinedAt
					};
				})
				.sort((a, b) => RANK[b.role] - RANK[a.role] || a.fullName.localeCompare(b.fullName));
			const invitations =
				can.invite.length > 0
					? state.invitations
							.filter((i) => i.classroomId === classroom.id && i.status === 'pending')
							.map((i) => toInvitation(state, i))
					: [];
			return {
				classroom: summarize(state, user, classroom),
				members,
				invitations,
				permissions: can,
				sharing: can.viewSharing
					? { sharing: { ...classroom.sharing }, terms: structuredClone(classroom.terms) }
					: null,
				setup: setupStatus(classroom, can)
			};
		},

		async update(classroomId, input) {
			await delay();
			const errors = validateClassroomUpdate(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.edit) {
					throw new ServiceError('forbidden', 'Only the institution head can edit the classroom.');
				}
				// The code is the classroom's identifier in SAM, so it is never editable.
				classroom.name = input.name.trim();
				classroom.institutionName = input.institutionName.trim();
				return summarize(state, user, classroom);
			});
		},

		async setInstitutionPhoto(classroomId, input) {
			await delay();
			const errors = validateInstitutionPhoto(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', Object.values(errors)[0], errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.edit) {
					throw new ServiceError(
						'forbidden',
						"Only the institution head can change the institution's photo."
					);
				}
				// Stand-in for an upload: the mock keeps the image itself as its URL.
				const info = classroom.profile.institution;
				info.photoUrl = input.image;
				info.photoAlt = input.image ? input.alt.trim() : '';
				return structuredClone(info);
			});
		},

		async create(input) {
			await delay();
			const errors = validateClassroomUpdate(input);
			const headEmail = input.headEmail.trim().toLowerCase();
			if (!headEmail) errors.headEmail = "Enter the institution head's email.";
			else if (!isValidEmail(headEmail)) errors.headEmail = 'Enter a valid email address.';
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				if (!user.platformAdmin) {
					throw new ServiceError('forbidden', 'Only platform admins can create a classroom.');
				}
				// Set up from a host application (AP-30): only once it is accepted, and only once.
				const application = input.applicationId
					? state.hostApplications.find((a) => a.id === input.applicationId)
					: undefined;
				if (input.applicationId && !application) {
					throw new ServiceError('not_found', 'Host application not found.');
				}
				if (application && (application.status !== 'accepted' || application.classroomId)) {
					throw new ServiceError(
						'conflict',
						application.classroomId
							? 'A classroom was already created from this application.'
							: 'Accept the application before creating its classroom.'
					);
				}
				const classroom: StoredClassroom = {
					id: newId('cls'),
					name: input.name.trim(),
					institutionName: input.institutionName.trim(),
					code: generateCode(state),
					setup: 'admin_assisted',
					createdBy: user.id,
					createdAt: iso(),
					terms: termsFor(state, input.name.trim(), input.institutionName.trim()),
					// A new classroom starts at the first stage, with nothing released yet.
					stageId: state.siteContent.stages[0].id,
					regionId: null,
					releasedThrough: null,
					sharing: { ...DEFAULT_SHARING },
					// The profile fills in as the classroom describes itself; no host agreement yet.
					profile: {
						areaCode: '',
						language: '',
						description: '',
						// What the application named; the head confirms them in the institution setup.
						ecosystems: application ? [...application.site.ecosystems] : [],
						hostAgreementSigned: false,
						projects: [],
						sessions: [],
						custodianRole: 'institution_head',
						institution: {
							photoUrl: null,
							photoAlt: '',
							history: '',
							hub: null,
							website: null,
							foundedYear: null,
							showPublicly: null
						}
					},
					// The head answers it after accepting their invitation (AP-40).
					institutionSetup: { step: 0, completedAt: null }
				};
				state.classrooms.push(classroom);
				if (application) application.classroomId = classroom.id;
				// The real backend registers the code with SAM so sources can report against it.
				state.samSources.push({
					sourceId: classroom.code,
					kind: 'classroom',
					registeredAt: classroom.createdAt
				});
				// The head joins by accepting this invitation, then sets up the lab and invites the rest.
				createInvitation(state, classroom.id, headEmail, 'institution_head', user.id);
				return summarize(state, user, classroom);
			});
		},

		async updateSharing(classroomId, input) {
			await delay();
			const errors = validateSharing(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.manageSharing) {
					throw new ServiceError(
						'forbidden',
						'Only the institution head can change how the classroom shares its data.'
					);
				}
				classroom.sharing = { ...input.sharing };
				classroom.terms = {
					...input.terms,
					licence: input.terms.licence.trim(),
					licenceUrl: input.terms.licenceUrl?.trim() || null,
					attribution: input.terms.attribution.trim(),
					restrictions: input.terms.restrictions.map((r) => r.trim()).filter(Boolean)
				};
				return { sharing: { ...classroom.sharing }, terms: structuredClone(classroom.terms) };
			});
		},

		async releaseReview(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const { classroom, can } = visibleClassroom(state, user, classroomId);
			if (!can.releaseData) {
				throw new ServiceError('forbidden', 'You cannot release this classroom’s data.');
			}
			return reviewOf(state, classroom, now());
		},

		async releaseData(classroomId, through) {
			await delay();
			if (Number.isNaN(Date.parse(through))) {
				throw new ServiceError('invalid_input', 'Choose when to release up to.');
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.releaseData) {
					throw new ServiceError('forbidden', 'You cannot release this classroom’s data.');
				}
				// Never later than now, and never back: what is public stays public.
				const at = new Date(Math.min(Date.parse(through), now())).toISOString();
				const current = classroom.releasedThrough;
				if (!current || at > current) classroom.releasedThrough = at;
				return reviewOf(state, classroom, now());
			});
		},

		async invite(classroomId, emails, role) {
			await delay();
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.invite.includes(role)) {
					throw new ServiceError('forbidden', 'You cannot invite people with that role.');
				}
				const unique = [...new Set(emails.map((e) => e.trim().toLowerCase()).filter(Boolean))];
				if (!unique.length) {
					throw new ServiceError('invalid_input', 'Add at least one email address.', {
						emails: 'Add at least one email address.'
					});
				}
				const invalid = unique.filter((e) => !isValidEmail(e));
				if (invalid.length) {
					const message = `Not valid: ${invalid.join(', ')}`;
					throw new ServiceError('invalid_input', message, { emails: message });
				}
				const members = unique.filter((email) => {
					const existing = state.users.find((u) => u.email === email);
					return existing && roleIn(state, classroom.id, existing.id);
				});
				if (members.length) {
					const message = `Already in this classroom: ${members.join(', ')}`;
					throw new ServiceError('conflict', message, { emails: message });
				}
				return unique.map((email) =>
					toInvitation(state, createInvitation(state, classroom.id, email, role, user.id))
				);
			});
		},

		async revokeInvitation(invitationId) {
			await delay();
			updateDb((state) => {
				const user = currentUser(state);
				const invitation = state.invitations.find((i) => i.id === invitationId);
				if (!invitation) throw new ServiceError('not_found', 'Invitation not found.');
				const { can } = visibleClassroom(state, user, invitation.classroomId);
				if (!can.invite.includes(invitation.role)) {
					throw new ServiceError('forbidden', 'You cannot revoke this invitation.');
				}
				invitation.status = 'revoked';
			});
		},

		async setMemberRole(classroomId, userId, role) {
			await delay();
			updateDb((state) => {
				const user = currentUser(state);
				const { can } = visibleClassroom(state, user, classroomId);
				const membership = state.memberships.find(
					(m) => m.classroomId === classroomId && m.userId === userId
				);
				if (!membership) throw new ServiceError('not_found', 'Member not found.');
				if (
					!can.manageMembers ||
					userId === user.id ||
					membership.role === 'institution_head' ||
					role === 'institution_head'
				) {
					throw new ServiceError('forbidden', 'You cannot change this member’s role.');
				}
				membership.role = role;
			});
		},

		async removeMember(classroomId, userId) {
			await delay();
			updateDb((state) => {
				const user = currentUser(state);
				const { can } = visibleClassroom(state, user, classroomId);
				const membership = state.memberships.find(
					(m) => m.classroomId === classroomId && m.userId === userId
				);
				if (!membership) throw new ServiceError('not_found', 'Member not found.');
				if (!can.manageMembers || userId === user.id || membership.role === 'institution_head') {
					throw new ServiceError('forbidden', 'You cannot remove this member.');
				}
				state.memberships = state.memberships.filter((m) => m !== membership);
			});
		},

		async listConsent(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const { classroom, can } = visibleClassroom(state, user, classroomId);
			if (!can.manageConsent) {
				throw new ServiceError(
					'forbidden',
					'Only the institution head and facilitators can see guardian consent.'
				);
			}
			return minorsIn(state, classroom.id).map((m) => toMemberConsent(state, m));
		},

		async recordPaperConsent(classroomId, userId, input) {
			await delay();
			const errors = validatePaperConsent(input, localDay(new Date(now())));
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const { classroom, can } = visibleClassroom(state, user, classroomId);
				if (!can.manageConsent) {
					throw new ServiceError(
						'forbidden',
						'Only the institution head and facilitators can record guardian consent.'
					);
				}
				const minor = minorsIn(state, classroom.id).find((m) => m.id === userId);
				if (!minor) throw new ServiceError('not_found', 'No member under 18 with that name.');
				const at = iso();
				const value = input.give ? 'given' : 'withdrawn';
				let stored = state.guardianConsents.find((c) => c.userId === minor.id);
				if (!stored) {
					stored = {
						userId: minor.id,
						token: null,
						method: 'paper',
						signedOn: null,
						recordedBy: null,
						guardianName: '',
						guardianContact: '',
						status: 'pending',
						requestedAt: at,
						decidedAt: null
					};
					state.guardianConsents.push(stored);
				}
				// Any link already sent keeps working: the guardian can still change their answer.
				Object.assign(stored, {
					method: 'paper',
					status: value,
					guardianName: input.guardianName.trim(),
					signedOn: input.signedOn.trim(),
					recordedBy: user.id,
					decidedAt: at
				});
				minor.consent = value;
				minor.updatedAt = at;
				// Stand-in for the `consent` attribute assertion sent to SAM, as with the link.
				state.samAssertions.push({
					userId: minor.id,
					attribute: 'consent',
					value,
					assertedAt: at
				});
				return toMemberConsent(state, minor);
			});
		},

		async consentSheet(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const { classroom, can } = visibleClassroom(state, user, classroomId);
			if (!can.manageConsent) {
				throw new ServiceError(
					'forbidden',
					'Only the institution head and facilitators can print consent sheets.'
				);
			}
			return {
				classroomName: classroom.name,
				institutionName: classroom.institutionName,
				content: fillConsentSheet(state.siteContent.consentSheet, {
					classroom: classroom.name,
					institution: classroom.institutionName,
					decimals: state.privacy.minorCoordinateDecimals
				}),
				minors: minorsIn(state, classroom.id).map((m) => ({ userId: m.id, fullName: m.fullName }))
			};
		},

		async getInvitation(token) {
			await delay();
			const state = readDb();
			const invitation = state.invitations.find((i) => i.token === token);
			if (!invitation || invitation.status === 'revoked') {
				throw new ServiceError('not_found', 'This invitation is no longer valid.');
			}
			return toPreview(state, invitation);
		},

		async listMyInvitations() {
			await delay();
			const state = readDb();
			const email = currentUser(state).email?.toLowerCase();
			if (!email) return [];
			return state.invitations
				.filter((i) => i.status === 'pending' && i.email === email)
				.sort((a, b) => b.createdAt.localeCompare(a.createdAt))
				.map((i) => toPreview(state, i));
		},

		async acceptInvitation(token) {
			await delay();
			return updateDb((state) => {
				const user = currentUser(state);
				const invitation = state.invitations.find((i) => i.token === token);
				const classroom = state.classrooms.find((c) => c.id === invitation?.classroomId);
				if (!invitation || !classroom || invitation.status === 'revoked') {
					throw new ServiceError('not_found', 'This invitation is no longer valid.');
				}
				if (invitation.status === 'accepted') {
					throw new ServiceError('conflict', 'This invitation has already been used.');
				}
				if (user.email?.toLowerCase() !== invitation.email) {
					throw new ServiceError(
						'forbidden',
						`This invitation was sent to ${invitation.email}. Sign in with that email to accept it.`
					);
				}
				const existing = state.memberships.find(
					(m) => m.classroomId === classroom.id && m.userId === user.id
				);
				if (!existing) {
					state.memberships.push({
						classroomId: classroom.id,
						userId: user.id,
						role: invitation.role,
						joinedAt: iso()
					});
				} else if (RANK[invitation.role] > RANK[existing.role]) {
					existing.role = invitation.role;
				}
				if (invitation.role === 'institution_head' && user.role === 'community_scientist') {
					user.role = 'institution_head';
				}
				invitation.status = 'accepted';
				invitation.acceptedBy = user.id;
				return summarize(state, user, classroom);
			});
		}
	};
}
