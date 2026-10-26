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
