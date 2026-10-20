import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });

beforeEach(() => resetDb());

const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);

async function signUpWithPhone(phone: string, fullName: string, email: string) {
	const challenge = await services.auth.requestPhoneCode(phone);
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
	await services.profile.updateProfile({ fullName, email });
	await services.profile.updateAddress({
		street: '1 Main Street',
		city: 'Shashamane',
		state: '',
		zip: '',
		country: 'Ethiopia'
	});
	return services.profile.completeOnboarding();
}

describe('creating a classroom (AP-03)', () => {
	it('gives an institution head a unique code registered as a SAM source', async () => {
		await signInAs('g-tesfaye');
		const a = await services.classrooms.create({ name: 'Second Lab', institutionName: 'Trinity' });
		const b = await services.classrooms.create({ name: 'Third Lab', institutionName: 'Trinity' });
		expect(a.code).toMatch(/^DGC-[A-Z2-9]{6}$/);
		expect(a.code).not.toBe(b.code);
		expect(a).toMatchObject({ myRole: 'institution_head', setup: 'self_service', memberCount: 1 });
	});

	it('lets a platform admin set one up for an institution head (admin-assisted)', async () => {
		await signInAs('g-leila');
		const classroom = await services.classrooms.create({
			name: 'Nassau Coastal Classroom',
			institutionName: 'Bay Street School',
			headEmail: 'Sarah.K@example.org'
		});
		expect(classroom).toMatchObject({ setup: 'admin_assisted', myRole: null, memberCount: 0 });
		const detail = await services.classrooms.get(classroom.id);
		expect(detail.invitations).toEqual([
			expect.objectContaining({ email: 'sarah.k@example.org', role: 'institution_head' })
		]);

		await signInAs('g-sarah');
		const joined = await services.classrooms.acceptInvitation(detail.invitations[0].token);
		expect(joined.myRole).toBe('institution_head');
		expect((await services.auth.getCurrentUser())?.role).toBe('custodian');
	});

	it('refuses contributors', async () => {
		await signInAs('g-sarah');
		await expect(
			services.classrooms.create({ name: 'Mine', institutionName: 'School' })
		).rejects.toMatchObject({ code: 'forbidden' });
	});
});

describe('classroom code visibility', () => {
	it('hides the code and member emails from community scientists', async () => {
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		const [classroom] = await services.classrooms.listMine();
		expect(classroom.myRole).toBe('community_scientist');
		expect(classroom.code).toBeNull();
		const detail = await services.classrooms.get(classroom.id);
		expect(detail.classroom.code).toBeNull();
		expect(detail.members.every((m) => m.email === null)).toBe(true);
		expect(detail.invitations).toEqual([]);
		expect(detail.permissions).toEqual({ invite: [], manageMembers: false, edit: false });
	});

	it('shows it to the institution head', async () => {
		await signInAs('g-tesfaye');
		const [classroom] = await services.classrooms.listMine();
		expect(classroom.code).toBe('DGC-7K3Q9M');
	});

	it('hides classrooms from non-members', async () => {
		await signInAs('g-amina');
		expect(await services.classrooms.listMine()).toEqual([]);
		await expect(services.classrooms.get('cls_trinity')).rejects.toMatchObject({
			code: 'not_found'
		});
	});
});

describe('invitations', () => {
	it('attaches a new user who signs up from the invite to the classroom', async () => {
		await signInAs('g-tesfaye');
		const [invitation] = await services.classrooms.invite(
			'cls_trinity',
			['New.Scientist@example.org'],
			'community_scientist'
		);
		await services.auth.signOut();

		const preview = await services.classrooms.getInvitation(invitation.token);
		expect(preview).toMatchObject({ classroomName: 'Trinity Lab Classroom', status: 'pending' });

		await signUpWithPhone('+251933000111', 'New Scientist', 'new.scientist@example.org');
		const joined = await services.classrooms.acceptInvitation(invitation.token);
		expect(joined).toMatchObject({ id: 'cls_trinity', myRole: 'community_scientist' });
		await expect(services.classrooms.acceptInvitation(invitation.token)).rejects.toMatchObject({
			code: 'conflict'
		});
	});

	it('only lets the invited email accept', async () => {
		await signInAs('g-amina');
		await expect(services.classrooms.acceptInvitation('demo-hana-invite')).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('limits facilitators to inviting community scientists', async () => {
		await signInAs('g-hana');
		const challenge = await services.profile.startPhoneVerification('+251944000222');
		await services.profile.confirmPhoneVerification(challenge.requestId, challenge.devCode!);
		await services.classrooms.acceptInvitation('demo-hana-invite');

		const detail = await services.classrooms.get('cls_trinity');
		expect(detail.permissions.invite).toEqual(['community_scientist']);
		expect(detail.classroom.code).toBe('DGC-7K3Q9M');
		await expect(
			services.classrooms.invite('cls_trinity', ['x@example.org'], 'facilitator')
		).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('rejects invalid emails and people already in the classroom', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.classrooms.invite('cls_trinity', ['not-an-email'], 'community_scientist')
		).rejects.toMatchObject({ code: 'invalid_input' });
		await expect(
			services.classrooms.invite('cls_trinity', ['abebe.t@example.org'], 'community_scientist')
		).rejects.toMatchObject({ code: 'conflict' });
	});

	it('stops a revoked invitation from working', async () => {
		await signInAs('g-tesfaye');
		const { invitations } = await services.classrooms.get('cls_trinity');
		await services.classrooms.revokeInvitation(invitations[0].id);
		await expect(services.classrooms.getInvitation('demo-hana-invite')).rejects.toMatchObject({
			code: 'not_found'
		});
	});
});

describe('managing members', () => {
	it('lets the institution head promote and remove members, but not themselves', async () => {
		await signInAs('g-tesfaye');
		await services.classrooms.setMemberRole('cls_trinity', 'usr_abebe', 'facilitator');
		let detail = await services.classrooms.get('cls_trinity');
		expect(detail.members.find((m) => m.userId === 'usr_abebe')?.role).toBe('facilitator');

		await expect(
			services.classrooms.setMemberRole('cls_trinity', 'usr_tesfaye', 'facilitator')
		).rejects.toMatchObject({ code: 'forbidden' });

		await services.classrooms.removeMember('cls_trinity', 'usr_abebe');
		detail = await services.classrooms.get('cls_trinity');
		expect(detail.members.map((m) => m.userId)).not.toContain('usr_abebe');
	});
});

describe('researchers', () => {
	it('can be invited by the institution head but not by facilitators', async () => {
		await signInAs('g-tesfaye');
		const detail = await services.classrooms.get('cls_trinity');
		expect(detail.permissions.invite).toContain('researcher');
		const [invitation] = await services.classrooms.invite(
			'cls_trinity',
			['new.researcher@example.org'],
			'researcher'
		);
		expect(invitation.role).toBe('researcher');
		await services.classrooms.setMemberRole('cls_trinity', 'usr_abebe', 'facilitator');
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		await expect(
			services.classrooms.invite('cls_trinity', ['other@example.org'], 'researcher')
		).rejects.toMatchObject({ code: 'forbidden' });
	});
});

describe('editing a classroom', () => {
	it('lets the institution head change everything but the code', async () => {
		await signInAs('g-tesfaye');
		const updated = await services.classrooms.update('cls_trinity', {
			name: '  Trinity Science Classroom ',
			institutionName: 'Trinity International School'
		});
		expect(updated).toMatchObject({
			name: 'Trinity Science Classroom',
			institutionName: 'Trinity International School',
			code: 'DGC-7K3Q9M'
		});
		expect((await services.classrooms.get('cls_trinity')).permissions.edit).toBe(true);
	});

	it('refuses members who are not the head, and blank names', async () => {
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		expect((await services.classrooms.get('cls_trinity')).permissions.edit).toBe(false);
		await expect(
			services.classrooms.update('cls_trinity', { name: 'Renamed', institutionName: 'Trinity' })
		).rejects.toMatchObject({ code: 'forbidden' });

		await signInAs('g-tesfaye');
		await expect(
			services.classrooms.update('cls_trinity', { name: '', institutionName: 'Trinity' })
		).rejects.toMatchObject({ code: 'invalid_input', fields: { name: expect.any(String) } });
	});
});

describe('invitations shared with the invited person', () => {
	it('shows an invite to the person it was sent to when they sign in, and lets them join', async () => {
		await signInAs('g-tesfaye');
		await services.classrooms.invite('cls_trinity', ['sarah.k@example.org'], 'facilitator');

		await signInAs('g-sarah');
		const [invitation] = await services.classrooms.listMyInvitations();
		expect(invitation).toMatchObject({
			classroomName: 'Trinity Lab Classroom',
			role: 'facilitator',
			invitedByName: 'Tesfaye G.'
		});
		await services.classrooms.acceptInvitation(invitation.token);
		expect(await services.classrooms.listMyInvitations()).toEqual([]);
		expect((await services.classrooms.listMine()).map((c) => c.id)).toContain('cls_trinity');

		// The head sees the new member and the invitation as used.
		await signInAs('g-tesfaye');
		const detail = await services.classrooms.get('cls_trinity');
		expect(detail.members.find((m) => m.fullName === 'Sarah K.')?.role).toBe('facilitator');
		expect(detail.invitations.some((i) => i.email === 'sarah.k@example.org')).toBe(false);
	});

	it('does not show invites sent to someone else, or revoked ones', async () => {
		await signInAs('g-tesfaye');
		const [sent] = await services.classrooms.invite(
			'cls_trinity',
			['sarah.k@example.org'],
			'community_scientist'
		);
		await signInAs('g-hana');
		expect((await services.classrooms.listMyInvitations()).map((i) => i.email)).toEqual([
			'hana.bekele@example.org'
		]);
		await signInAs('g-tesfaye');
		await services.classrooms.revokeInvitation(sent.id);
		await signInAs('g-sarah');
		expect(await services.classrooms.listMyInvitations()).toEqual([]);
	});
});

describe('demo accounts', () => {
	it('keeps the researcher and the platform admin as separate people', async () => {
		await signInAs('g-marcus');
		const followed = await services.classrooms.listMine();
		expect(followed.map((c) => c.id).sort()).toEqual(['cls_hargeisa', 'cls_nassau', 'cls_trinity']);
		expect(followed.every((c) => c.myRole === 'researcher')).toBe(true);

		await signInAs('g-leila');
		const all = await services.classrooms.listMine();
		expect(all.length).toBe(3);
		expect(all.every((c) => c.myRole === null)).toBe(true);
	});

	it('puts the demo community scientist in two classrooms', async () => {
		await signInAs('g-sarah');
		const mine = await services.classrooms.listMine();
		expect(mine.map((c) => c.id).sort()).toEqual(['cls_hargeisa', 'cls_nassau']);
		expect(mine.every((c) => c.myRole === 'community_scientist' && c.code === null)).toBe(true);
	});
});
