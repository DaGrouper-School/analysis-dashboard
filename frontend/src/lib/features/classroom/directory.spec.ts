import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { createMockServices } from '#lib/services/mock/index.ts';
import {
	filterClassrooms,
	filterOptions,
	filterQuery,
	NO_FILTER,
	readFilter
} from '#lib/features/classroom/directory.ts';

const services = createMockServices({ latency: 0 });

beforeEach(() => resetDb());

describe('Global Classrooms directory (AP-28)', () => {
	it('filters by region, country and stage, offering only values the classrooms use', async () => {
		const { classrooms, content } = await services.publicSite.home();
		expect(filterClassrooms(classrooms, NO_FILTER)).toHaveLength(classrooms.length);
		expect(
			filterClassrooms(classrooms, { ...NO_FILTER, region: 'east_africa' }).map((c) => c.id)
		).toEqual(expect.arrayContaining(['cls_trinity', 'cls_hargeisa']));
		expect(
			filterClassrooms(classrooms, { region: 'east_africa', country: '', stage: 'operating' }).map(
				(c) => c.id
			)
		).toEqual(['cls_trinity']);
		const options = filterOptions(classrooms, content);
		expect(options.regions.map((r) => r.id)).toEqual(['caribbean', 'east_africa']);
		expect(options.stages.map((s) => s.id)).toEqual(['preparing', 'pilot', 'operating']);
	});

	it('keeps the filter in the URL', () => {
		const query = filterQuery({ region: 'caribbean', country: '', stage: 'pilot' });
		expect(query).toBe('?region=caribbean&stage=pilot');
		expect(readFilter(new URLSearchParams(query))).toEqual({
			region: 'caribbean',
			country: '',
			stage: 'pilot'
		});
		expect(filterQuery(NO_FILTER)).toBe('');
	});

	it('shows themes, never codes or people', async () => {
		const { classrooms } = await services.publicSite.home();
		const trinity = classrooms.find((c) => c.id === 'cls_trinity')!;
		expect(trinity.themes.length).toBeGreaterThan(0);
		const text = JSON.stringify(classrooms);
		expect(text).not.toContain('DGC-');
		expect(text).not.toMatch(/Liya|Abebe|creditedTo/);
	});
});
