import { describe, expect, it } from 'vitest';
import { DEFAULT_SHARING, permissionsFor, publicCutoff, releaseCutoff } from './permissions';
import type { ClassroomRole, ClassroomSharing } from './types';

const member = { platformAdmin: false };
const admin = { platformAdmin: true };
const as = (role: ClassroomRole | null, sharing: Partial<ClassroomSharing> = {}) =>
	permissionsFor(role ? member : null, role, { ...DEFAULT_SHARING, ...sharing });

describe('permissionsFor', () => {
	it('lets a visitor do nothing but look at released data', () => {
		const can = as(null);
		expect(
			Object.entries(can).filter(([, v]) => v === true || (Array.isArray(v) && v.length))
		).toEqual([]);
	});

	it('follows the role grid', () => {
		const head = as('institution_head');
		const facilitator = as('facilitator');
		const scientist = as('community_scientist');
		const researcher = as('researcher');

		for (const can of [head, facilitator, scientist, researcher]) {
			expect(can.view).toBe(true);
			expect(can.seeUnreleased).toBe(true);
			expect(can.openValuePage).toBe(true);
		}
		expect([head, facilitator, scientist, researcher].map((c) => c.seeCode)).toEqual([
			true,
			true,
			false,
			false
		]);
		expect([head, facilitator, scientist, researcher].map((c) => c.recordReadings)).toEqual([
			true,
			true,
			true,
			false
		]);
		expect([head, facilitator, scientist, researcher].map((c) => c.manageDevices)).toEqual([
			true,
			true,
			false,
			false
		]);
		expect(head.invite).toEqual(['facilitator', 'community_scientist', 'researcher']);
		expect(facilitator.invite).toEqual(['community_scientist']);
		expect(scientist.invite).toEqual([]);
		expect(head.manageSharing).toBe(true);
		expect(facilitator.manageSharing).toBe(false);
		expect(facilitator.viewSharing).toBe(true);
		expect(researcher.viewSharing).toBe(false);
	});

	it('lets a platform admin who is not a member manage but not see data in review', () => {
		const can = permissionsFor(admin, null, DEFAULT_SHARING);
		expect(can.view).toBe(true);
		expect(can.manageMembers).toBe(true);
		expect(can.seeUnreleased).toBe(false);
		expect(can.openValuePage).toBe(false);
		expect(can.releaseData).toBe(false);
	});

	it('lets the head and facilitators release data, or the head only', () => {
		expect(as('institution_head').releaseData).toBe(true);
		expect(as('facilitator').releaseData).toBe(true);
		expect(as('community_scientist').releaseData).toBe(false);
		expect(as('facilitator', { releasers: 'head' }).releaseData).toBe(false);
		expect(as('institution_head', { releasers: 'head' }).releaseData).toBe(true);
	});

	it('follows what the head allows facilitators', () => {
		const limited = as('facilitator', {
			facilitatorsInvite: false,
			facilitatorsManageDevices: false
		});
		expect(limited.invite).toEqual([]);
		expect(limited.manageDevices).toBe(false);
		// The head keeps both.
		const head = as('institution_head', {
			facilitatorsInvite: false,
			facilitatorsManageDevices: false
		});
		expect(head.invite.length).toBeGreaterThan(0);
		expect(head.manageDevices).toBe(true);
	});
});

describe('release cutoff', () => {
	const now = Date.parse('2026-11-11T12:00:00.000Z');
	const base = { releasedThrough: '2026-11-01T00:00:00.000Z', sharing: DEFAULT_SHARING };

	it('is what was released by hand', () => {
		expect(releaseCutoff(base, now)).toBe(base.releasedThrough);
	});

	it('moves with time when readings release by themselves', () => {
		const auto = {
			...base,
			sharing: { ...DEFAULT_SHARING, review: 'auto' as const, autoReleaseDays: 2 }
		};
		expect(releaseCutoff(auto, now)).toBe('2026-11-09T12:00:00.000Z');
		// A later release by hand still counts.
		expect(releaseCutoff({ ...auto, releasedThrough: '2026-11-10T00:00:00.000Z' }, now)).toBe(
			'2026-11-10T00:00:00.000Z'
		);
	});

	it('shows visitors nothing while the classroom does not share', () => {
		const hidden = { ...base, sharing: { ...DEFAULT_SHARING, showPublic: false } };
		expect(publicCutoff(hidden, now)).toBeNull();
		expect(releaseCutoff(hidden, now)).toBe(base.releasedThrough);
	});
});
