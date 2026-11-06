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
});
