import type {
	AlertRuleInput,
	ClassroomAlerts,
	ClassroomDetail,
	ClassroomRole,
	ClassroomSummary,
	Invitation,
	InvitationPreview,
	User
} from '#lib/domain/types.ts';
import { isValidEmail, validateClassroomUpdate } from '#lib/domain/validation.ts';
import { checkAlertRule, effectiveRules } from '#lib/domain/alerts.ts';
import {
	newId,
	readDb,
	updateDb,
	type MockDbState,
	type StoredClassroom,
	type StoredInvitation
} from '#lib/data/mock-db.ts';
import { ServiceError, type ClassroomService } from '../types';

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

/** Who may do what in a classroom. This is the rule the backend must enforce too. */
export function permissionsFor(user: User, role: ClassroomRole | null) {
	const isHead = role === 'institution_head';
	return {
		view: user.platformAdmin || role !== null,
		seeCode: user.platformAdmin || isHead || role === 'facilitator',
		seeEmails: user.platformAdmin || isHead || role === 'facilitator',
		invite: (user.platformAdmin
			? ['institution_head', 'facilitator', 'community_scientist', 'researcher']
			: isHead
				? ['facilitator', 'community_scientist', 'researcher']
				: role === 'facilitator'
					? ['community_scientist']
					: []) as ClassroomRole[],
		manageMembers: user.platformAdmin || isHead,
		// The classroom's own focal people. A platform admin sets the defaults, not a classroom's.
		manageAlerts: isHead || role === 'facilitator',
		// Connecting an already registered device and recording its readings. Researchers only read.
		recordReadings:
			user.platformAdmin || isHead || role === 'facilitator' || role === 'community_scientist'
	};
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
	const myRole = roleIn(state, classroom.id, user.id);
	return {
		id: classroom.id,
		name: classroom.name,
		institutionName: classroom.institutionName,
		code: permissionsFor(user, myRole).seeCode ? classroom.code : null,
		setup: classroom.setup,
		createdAt: classroom.createdAt,
		myRole,
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

/** The alert rules in force for a classroom, with who last changed its own. */
export function classroomRules(state: MockDbState, classroomId: string) {
	return effectiveRules(
		state.alertDefaults,
		state.classroomAlerts.filter((r) => r.classroomId === classroomId),
		(id) => state.users.find((u) => u.id === id)?.fullName ?? 'Unknown'
	);
}

/** Loads a classroom the user may see. Hidden classrooms read as not found. */
function visibleClassroom(state: MockDbState, user: User, classroomId: string) {
	const classroom = state.classrooms.find((c) => c.id === classroomId);
	const role = classroom ? roleIn(state, classroom.id, user.id) : null;
	const can = permissionsFor(user, role);
	if (!classroom || !can.view) throw new ServiceError('not_found', 'Classroom not found.');
	return { classroom, role, can };
}

export function createMockClassrooms({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): ClassroomService {
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

	function alertsOf(state: MockDbState, classroomId: string, canEdit: boolean): ClassroomAlerts {
		return structuredClone({ rules: classroomRules(state, classroomId), canEdit });
	}

	/** Loads the classroom for an alert change, refusing anyone who is not a head or facilitator. */
	function alertEditor(state: MockDbState, classroomId: string) {
		const user = currentUser(state);
		const { can } = visibleClassroom(state, user, classroomId);
		if (!can.manageAlerts) {
			throw new ServiceError(
				'forbidden',
				'Only the classroom’s institution head and facilitators can change its alerts.'
			);
		}
		return user;
	}

	/**
	 * The classroom's own rules for a parameter. The first change copies the defaults in, so
	 * changing one level leaves the others as they were. Returns the copies keyed by default ID.
	 */
	function ownRules(state: MockDbState, classroomId: string, parameterId: string, userId: string) {
		const copies = new Map<string, string>();
		const own = state.classroomAlerts.filter(
			(r) => r.classroomId === classroomId && r.parameterId === parameterId
		);
		if (own.length) return copies;
		for (const rule of state.alertDefaults.filter((r) => r.parameterId === parameterId)) {
			const id = newId('alr');
			copies.set(rule.id, id);
			state.classroomAlerts.push({
				...rule,
				id,
				classroomId,
				updatedBy: userId,
				updatedAt: iso()
			});
		}
		return copies;
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
				permissions: {
					invite: can.invite,
					manageMembers: can.manageMembers,
					edit: can.manageMembers,
					manageAlerts: can.manageAlerts
				}
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
				if (!can.manageMembers) {
					throw new ServiceError('forbidden', 'Only the institution head can edit the classroom.');
				}
				// The code is the classroom's identifier in SAM, so it is never editable.
				classroom.name = input.name.trim();
				classroom.institutionName = input.institutionName.trim();
				return summarize(state, user, classroom);
			});
		},

		async create(input) {
			await delay();
			const errors = validateClassroomUpdate(input);
			const headEmail = input.headEmail?.trim().toLowerCase();
			if (headEmail !== undefined && headEmail !== '' && !isValidEmail(headEmail)) {
				errors.headEmail = 'Enter a valid email address.';
			}
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user = currentUser(state);
				const assisted = Boolean(headEmail);
				if (
					assisted ? !user.platformAdmin : user.role !== 'institution_head' && !user.platformAdmin
				) {
					throw new ServiceError(
						'forbidden',
						assisted
							? 'Only platform admins can set up a classroom for someone else.'
							: 'Only institution heads can create a classroom.'
					);
				}
				const classroom: StoredClassroom = {
					id: newId('cls'),
					name: input.name.trim(),
					institutionName: input.institutionName.trim(),
					code: generateCode(state),
					setup: assisted ? 'admin_assisted' : 'self_service',
					createdBy: user.id,
					createdAt: iso()
				};
				state.classrooms.push(classroom);
				// The real backend registers the code with SAM so sources can report against it.
				state.samSources.push({
					sourceId: classroom.code,
					kind: 'classroom',
					registeredAt: classroom.createdAt
				});
				if (assisted) {
					createInvitation(state, classroom.id, headEmail!, 'institution_head', user.id);
				} else {
					state.memberships.push({
						classroomId: classroom.id,
						userId: user.id,
						role: 'institution_head',
						joinedAt: classroom.createdAt
					});
				}
				return summarize(state, user, classroom);
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
		},
		async listAlerts(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const { can } = visibleClassroom(state, user, classroomId);
			return alertsOf(state, classroomId, can.manageAlerts);
		},

		async saveAlert(classroomId, input, ruleId) {
			await delay();
			return updateDb((state) => {
				const user = alertEditor(state, classroomId);
				const rule: AlertRuleInput = {
					parameterId: input.parameterId,
					direction: input.direction,
					value: input.value,
					level: input.level,
					title: input.title?.trim() ?? '',
					enabled: input.enabled !== false
				};
				const errors = checkAlertRule(rule, state.parameters);
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				if (ruleId) {
					const current = classroomRules(state, classroomId).find((r) => r.id === ruleId);
					if (!current) throw new ServiceError('not_found', 'That alert no longer exists.');
					if (current.parameterId !== rule.parameterId) {
						const message = 'An alert stays on its value. Add a new one for another value.';
						throw new ServiceError('invalid_input', message, { parameterId: message });
					}
					const copies = ownRules(state, classroomId, rule.parameterId, user.id);
					const id = copies.get(ruleId) ?? ruleId;
					const stored = state.classroomAlerts.find(
						(r) => r.classroomId === classroomId && r.id === id
					)!;
					Object.assign(stored, rule, { updatedBy: user.id, updatedAt: iso() });
				} else {
					ownRules(state, classroomId, rule.parameterId, user.id);
					state.classroomAlerts.push({
						...rule,
						id: newId('alr'),
						classroomId,
						updatedBy: user.id,
						updatedAt: iso()
					});
				}
				return alertsOf(state, classroomId, true);
			});
		},

		async deleteAlert(classroomId, ruleId) {
			await delay();
			return updateDb((state) => {
				alertEditor(state, classroomId);
				const index = state.classroomAlerts.findIndex(
					(r) => r.classroomId === classroomId && r.id === ruleId
				);
				if (index < 0) {
					throw new ServiceError(
						'not_found',
						'Only the classroom’s own alerts can be deleted. Turn a default off instead.'
					);
				}
				state.classroomAlerts.splice(index, 1);
				return alertsOf(state, classroomId, true);
			});
		},

		async resetAlerts(classroomId, parameterId) {
			await delay();
			return updateDb((state) => {
				alertEditor(state, classroomId);
				state.classroomAlerts = state.classroomAlerts.filter(
					(r) => !(r.classroomId === classroomId && r.parameterId === parameterId)
				);
				return alertsOf(state, classroomId, true);
			});
		}
	};
}
