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
		expect(station.can.seeUnreleased).toBe(false);
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

	it('lays out a value in a fixed order: the classroom’s note, chart type and compare list (AP-23)', async () => {
		const page = await services.publicSite.parameter('cls_trinity', 'air_temperature');
		expect(page.parameter.chart).toBe('line_band');
		expect(page.note).toContain('Shashamane');
		expect(page.rain).toBeNull();
		expect(page.inReview).toBe(true);
		expect(page.releasedThrough).toBe('2026-10-20T06:00:00.000Z');
		expect(page.deviceTypes).toEqual(['Weather station']);
		expect(page.terms.licence).toBeTruthy();
		// Hargeisa has nothing released yet, so it is not offered.
		expect(page.compare.map((c) => c.id)).toEqual(['cls_nassau']);
		const other = await services.publicSite.parameter('cls_trinity', 'wind_avg');
		expect(other.note).toBeNull();
	});

	it('shows every rain source together, with the manual gauge as the reference (AP-23)', async () => {
		const page = await services.publicSite.parameter('cls_trinity', 'precip_accum_local_day');
		expect(page.parameter.chart).toBe('daily_bars');
		expect(page.series.map((s) => [s.source.id, s.parameterId])).toEqual([
			['dev_shashamane_gauge', 'rain_gauge_daily_accumulation'],
			['dev_shashamane_ws', 'precip_accum_local_day']
		]);
		expect(page.rain).toEqual({ referenceSourceId: 'dev_shashamane_gauge', tolerance: 2 });
		// Nassau's station measures rain too, so the gauge page can be compared with it.
		const gauge = await services.publicSite.parameter(
			'cls_trinity',
			'rain_gauge_daily_accumulation'
		);
		expect(gauge.compare.map((c) => c.id)).toEqual(['cls_nassau']);
		const nassau = await services.publicSite.parameter(
			'cls_nassau',
			'rain_gauge_daily_accumulation'
		);
		expect(nassau.rain?.referenceSourceId).toBeNull();
	});

	it('tells members they can open the classroom on their dashboard', async () => {
		const challenge = await services.auth.requestPhoneCode('+251911000001');
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
		expect((await services.publicSite.station('cls_trinity')).can.seeUnreleased).toBe(true);
		expect((await services.publicSite.station('cls_nassau')).can.seeUnreleased).toBe(false);
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

describe('Explore station page (AP-43)', () => {
	it('gives the location tabs every public classroom with a place on the map', async () => {
		const station = await services.publicSite.station('cls_trinity');
		expect(station.places.length).toBeGreaterThan(1);
		expect(station.places.every((p) => p.location)).toBe(true);
		expect(station.places.map((p) => p.id)).toContain('cls_trinity');
		expect(JSON.stringify(station.places)).not.toContain('code');
	});

	it('shows what is happening now from released values, and no data as null', async () => {
		const trinity = await services.publicSite.station('cls_trinity');
		expect(trinity.now.map((t) => t.id)).toEqual(['temperature', 'wind', 'uv', 'pressure']);
		const temp = trinity.now.find((t) => t.id === 'temperature')!;
		expect(typeof temp.value).toBe('number');
		expect(temp.unit).toBe('°C');
		expect(temp.observedAt! <= '2026-10-20T06:00:00.000Z').toBe(true);
		// Each tile carries its last 24 hours, one point an hour.
		expect(temp.last24Hours).toHaveLength(24);
		expect(temp.last24Hours.at(-1)).toBe(temp.value);
		const hargeisa = await services.publicSite.station('cls_hargeisa');
		expect(hargeisa.now.every((t) => t.value === null && t.observedAt === null)).toBe(true);
	});

	it('shows the institution overview once a host agreement is signed', async () => {
		const trinity = await services.publicSite.station('cls_trinity');
		expect(trinity.institution).toMatchObject({ name: 'Trinity School, Addis Ababa' });
		expect(trinity.institution!.history).not.toBe('');
		expect(trinity.institution!.deviceTypes.length).toBeGreaterThan(0);
		expect((await services.publicSite.station('cls_hargeisa')).institution).toBeNull();
		expect((await services.publicSite.profile('cls_trinity')).institution).toEqual(
			trinity.institution
		);
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

	it('lists every contributor by participant ID, named only with a public profile', async () => {
		const page = await services.publicSite.profile('cls_trinity');
		expect(page.contributors).toEqual([
			{ participantId: 100002, name: 'Abebe T.' },
			{ participantId: 100005, name: null }
		]);
	});

	it('never names a minor', async () => {
		const { readDb, writeDb } = await import('#lib/data/mock-db.ts');
		const state = readDb();
		state.users.find((u) => u.id === 'usr_liya')!.publicProfile = true;
		state.observations.push({
			...state.observations[0],
			id: 'obs_liya',
			creditedToUserId: 'usr_liya',
			recordedByUserId: 'usr_liya'
		});
		writeDb(state);
		const page = await services.publicSite.profile('cls_trinity');
		expect(page.contributors).toContainEqual({ participantId: 100006, name: null });
		expect((await services.publicSite.contributor(100006)).profile).toBeNull();
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

describe('Contributor pages (AP-39)', () => {
	const signIn = async (phone: string) => {
		const challenge = await services.auth.requestPhoneCode(phone);
		await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
	};

	it('shows a public profile with the released values the participant recorded', async () => {
		const page = await services.publicSite.contributor(100002);
		expect(page.profile).toMatchObject({ fullName: 'Abebe T.', role: 'community_scientist' });
		expect(page.classrooms.map((c) => c.id)).toEqual(['cls_trinity']);
		expect(page.total).toBeGreaterThan(0);
		expect(page.recent.every((c) => c.observedAt <= '2026-10-20T06:00:00.000Z')).toBe(true);
	});

	it('shows only the ID and contributions for a private profile', async () => {
		const page = await services.publicSite.contributor(100005);
		expect(page.profile).toBeNull();
		expect(JSON.stringify(page)).not.toContain('Tesfaye');
		expect(page.total).toBeGreaterThan(0);
	});

	it('has a page for every participant, with no released values yet too', async () => {
		const page = await services.publicSite.contributor(100004);
		expect([page.participantId, page.total, page.recent]).toEqual([100004, 0, []]);
	});

	it('has no page for a participant ID nobody has', async () => {
		await expect(services.publicSite.contributor(999999)).rejects.toMatchObject({
			code: 'not_found'
		});
	});

	it('lets adults turn their public profile on and off, and never minors', async () => {
		await signIn('+251911000002');
		expect((await services.profile.setPublicProfile(true)).publicProfile).toBe(true);
		expect((await services.publicSite.contributor(100005)).profile?.fullName).toBe('Tesfaye G.');
		expect((await services.profile.setPublicProfile(false)).publicProfile).toBe(false);
		expect((await services.publicSite.contributor(100005)).profile).toBeNull();
		await services.auth.signOut();
		await signIn('+251911000003');
		await expect(services.profile.setPublicProfile(true)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});
});

describe('the same devices on the public page and the dashboard (AP-45)', () => {
	it('lists every device, with no values while nothing is released', async () => {
		await services.auth.signOut();
		const station = await services.publicSite.station('cls_hargeisa');
		expect(station.devices.map((d) => d.id)).toEqual(['dev_hargeisa_ws']);
		const [device] = station.devices;
		expect(device.tiles.length).toBeGreaterThan(0);
		expect(device.tiles.every((t) => t.value === null && t.observedAt === null)).toBe(true);
		expect(device.summary.every((c) => c.latest === null)).toBe(true);
	});

	it('gives members the readings in review, from the same service', async () => {
		await services.auth.signInWithGoogle('g-tesfaye');
		const station = await services.publicSite.station('cls_trinity');
		expect(station.can.seeUnreleased).toBe(true);
		const newest = station.devices
			.flatMap((d) => d.tiles)
			.map((t) => t.observedAt!)
			.sort()
			.at(-1);
		expect(newest).toBe('2026-10-20T08:00:00.000Z');
	});
});

describe('releasing data (AP-20)', () => {
	it('tells the head what waits, and shows visitors what they release', async () => {
		await services.auth.signInWithGoogle('g-tesfaye');
		const review = await services.classrooms.releaseReview('cls_trinity');
		expect(review.pending).toBeGreaterThan(0);
		const through = review.sources
			.map((s) => s.last)
			.sort()
			.at(-1)!;
		const after = await services.classrooms.releaseData('cls_trinity', through);
		expect(after.pending).toBe(0);
		expect(after.releasedThrough).toBe(through);

		await services.auth.signOut();
		const station = await services.publicSite.station('cls_trinity');
		const newest = station.devices
			.flatMap((d) => d.tiles)
			.map((t) => t.observedAt ?? '')
			.sort()
			.at(-1);
		expect(newest).toBe(through);
	});

	it('refuses researchers and community scientists', async () => {
		await services.auth.signInWithGoogle('g-marcus');
		await expect(services.classrooms.releaseReview('cls_trinity')).rejects.toMatchObject({
			code: 'forbidden'
		});
		await expect(
			services.classrooms.releaseData('cls_trinity', '2026-10-20T08:00:00.000Z')
		).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('hides everything from visitors while the classroom does not share', async () => {
		await services.auth.signInWithGoogle('g-tesfaye');
		const { sharing } = (await services.classrooms.get('cls_trinity')).sharing!;
		const { terms } = (await services.classrooms.get('cls_trinity')).sharing!;
		await services.classrooms.updateSharing('cls_trinity', {
			sharing: { ...sharing, showPublic: false },
			terms
		});
		await services.auth.signOut();
		const station = await services.publicSite.station('cls_trinity');
		expect(station.shared).toBe(false);
		expect(station.devices.length).toBeGreaterThan(0);
		expect(station.devices.flatMap((d) => d.tiles).every((t) => t.value === null)).toBe(true);
	});
});
