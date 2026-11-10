import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });

beforeEach(() => resetDb());

describe('public home', () => {
	it('needs no sign-in and shows every classroom with its stage, released data only', async () => {
		await services.auth.signOut();
		const home = await services.publicSite.home();
		expect(home.content.sections.map((s) => s.id)).toEqual([
			'explore',
			'classrooms',
			'learn',
			'train',
			'cop32',
			'give'
		]);
		expect(home.units).toEqual({ rain: 'mm', temperature: '°C' });
		const byId = new Map(home.classrooms.map((c) => [c.id, c]));
		expect(byId.get('cls_trinity')).toMatchObject({ stageId: 'operating' });
		// Trinity is released up to 06:00; its 08:00 readings are still in review.
		expect(byId.get('cls_trinity')!.core).toMatchObject({
			day: '2026-10-20',
			review: 'in_review'
		});
		expect(byId.get('cls_trinity')!.core.lastSync! <= '2026-10-20T06:00:00.000Z').toBe(true);
		expect(byId.get('cls_hargeisa')!.core).toMatchObject({
			review: 'not_released',
			airTemperature: null,
			rainToday: null
		});
		expect(home.classrooms.every((c) => !('code' in c))).toBe(true);
	});
});

describe('Explore data (AP-21)', () => {
	it('shows a station with released values only, grouped by domain, without codes or names', async () => {
		await services.auth.signOut();
		const station = await services.publicSite.station('cls_trinity');
		expect(station.member).toBe(false);
		expect(station.content.sampleData).toBe(true);
		const values = station.domains.flatMap((d) => d.values);
		expect(values.length).toBeGreaterThan(0);
		expect(values.every((v) => v.observedAt <= '2026-10-20T06:00:00.000Z')).toBe(true);
		// Every value shown has its registry entry, and domains come in registry order.
		const ids = new Set(station.parameters.map((p) => p.id));
		expect(values.every((v) => ids.has(v.parameterId))).toBe(true);
		const text = JSON.stringify(station);
		expect(text).not.toContain('samSourceId');
		expect(text).not.toContain('creditedTo');
		expect(text).not.toContain('DGC-');
	});

	it('shows nothing from a classroom with no released data', async () => {
		const station = await services.publicSite.station('cls_hargeisa');
		expect(station.domains).toEqual([]);
		expect(station.classroom.core.review).toBe('not_released');
		await expect(
			services.publicSite.parameter('cls_hargeisa', 'air_temperature')
		).rejects.toMatchObject({ code: 'not_found' });
	});

	it('gives a parameter its meaning and released values over time, per source', async () => {
		const page = await services.publicSite.parameter('cls_trinity', 'air_temperature');
		expect(page.parameter.id).toBe('air_temperature');
		expect(page.domain?.id).toBe('AIR');
		expect(page.series.length).toBeGreaterThan(0);
		for (const series of page.series) {
			const times = series.points.map((p) => p.observedAt);
			expect(times).toEqual([...times].sort());
			expect(times.every((t) => t <= '2026-10-20T06:00:00.000Z')).toBe(true);
		}
		await expect(services.publicSite.station('cls_nope')).rejects.toMatchObject({
			code: 'not_found'
		});
	});

	it('tells members they can open the classroom on their dashboard', async () => {
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		expect((await services.publicSite.station('cls_trinity')).member).toBe(true);
		expect((await services.publicSite.station('cls_nassau')).member).toBe(false);
	});

	it('shows site codes to members only', async () => {
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		expect((await services.publicSite.station('cls_trinity')).siteCodes).toContain('TRN-S1');
		await services.auth.signOut();
		expect((await services.publicSite.station('cls_trinity')).siteCodes).toEqual([]);
	});

	it('gives each number a 7-day sparkline and the month its completeness (AP-22)', async () => {
		await services.auth.signOut();
		const station = await services.publicSite.station('cls_trinity');
		const air = station.domains
			.flatMap((d) => d.values)
			.find((v) => v.parameterId === 'air_temperature')!;
		expect(air.last7Days).toHaveLength(7);
		expect(air.last7Days.at(-1)).toBe(air.value);
		expect(station.core.temperature.review).toBe('in_review');
		// The seeded gauge skipped three days this month: rain shows them as missing.
		const rain = station.completeness!.domains.find((d) => d.domain?.id === 'RAIN')!;
		expect(station.completeness!.month).toBe('2026-10');
		expect(rain.days).toHaveLength(20);
		expect(rain.days.filter((d) => d.status !== 'complete').map((d) => d.day)).toEqual([
			'2026-10-09',
			'2026-10-16',
			'2026-10-17'
		]);
	});

	it('has no completeness before anything is released', async () => {
		expect((await services.publicSite.station('cls_hargeisa')).completeness).toBeNull();
	});
});

describe('Classroom profile (AP-29)', () => {
	const signIn = async (phone: string) => {
		const challenge = await services.auth.requestPhoneCode(phone);
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
	};

	it('shows who the classroom is, its projects, kit and next sessions to visitors', async () => {
		await services.auth.signOut();
		const page = await services.publicSite.profile('cls_trinity');
		expect(page.profile).toMatchObject({ areaCode: 'ET-AA', language: 'Amharic and English' });
		expect(page.classroom.institutionName).toBe('Trinity School, Addis Ababa');
		expect(page.projects[0].sites[0]).toMatchObject({ code: 'TRN-S1', frequency: 'Daily' });
		// Readings are shown by their registry labels, not IDs.
		expect(page.projects[0].sites[0].readings).not.toContain('air_temperature');
		expect(page.kit.map((k) => k.deviceType.id).sort()).toEqual(
			['rain_gauge', 'soil_sensor', 'weather_station'].sort()
		);
		// Serials and who looks after the kit are for members only.
		expect(page.kit.every((k) => k.serials.length === 0)).toBe(true);
		expect(page.custodianRole).toBeNull();
		expect(page.member).toBe(false);
		expect(page.canManage).toBe(false);
		const now = new Date().toISOString();
		expect(page.sessions.every((s) => s.startsAt >= now)).toBe(true);
		const text = JSON.stringify(page);
		expect(text).not.toContain('DGC-');
		expect(text).not.toContain('usr_');
	});

	it('names only people who took credit and counts the rest', async () => {
		const page = await services.publicSite.profile('cls_trinity');
		expect(page.contributors).toEqual({ named: ['Abebe T.'], anonymous: 1 });
	});

	it('never names a minor, even one who asked for credit', async () => {
		const { readDb, writeDb } = await import('#lib/data/mock-db.ts');
		const state = readDb();
		state.observations.push({
			...state.observations[0],
			id: 'obs_liya',
			creditedToUserId: 'usr_liya',
			recordedByUserId: 'usr_liya'
		});
		writeDb(state);
		const page = await services.publicSite.profile('cls_trinity');
		expect(page.contributors.named).not.toContain('Liya A.');
		expect(page.contributors.anonymous).toBe(2);
	});

	it('hides the host institution until a host agreement is signed', async () => {
		const page = await services.publicSite.profile('cls_hargeisa');
		expect(page.classroom.institutionName).toBeNull();
		const home = await services.publicSite.home();
		expect(home.classrooms.find((c) => c.id === 'cls_hargeisa')!.institutionName).toBeNull();
	});

	it('shows members the kit serials, and people with rights a way to manage it', async () => {
		await signIn('+251911000001');
		const member = await services.publicSite.profile('cls_trinity');
		expect(member.member).toBe(true);
		expect(member.kit.some((k) => k.serials.includes('TWS-2041-SH'))).toBe(true);
		expect(member.custodianRole).toBe('facilitator');
		expect(member.canManage).toBe(false);
		await services.auth.signOut();
		await signIn('+251911000002');
		expect((await services.publicSite.profile('cls_trinity')).canManage).toBe(true);
	});

	it('says when a classroom does not exist', async () => {
		await expect(services.publicSite.profile('cls_nope')).rejects.toMatchObject({
			code: 'not_found'
		});
	});
});
