import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { CSV_COLUMNS } from '#lib/features/measurements/raw-data.ts';
import { createMockServices } from './index';

const services = createMockServices({
	latency: 0,
	now: () => Date.parse('2026-10-22T09:00:00.000Z')
});

beforeEach(() => resetDb());

const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);

async function signInAbebe() {
	const challenge = await services.auth.requestPhoneCode('+251911000001');
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

/** Every row on one page. */
const ALL = { page: 1, pageSize: 10_000 };

describe('raw data export', () => {
	it('only serves classrooms the user belongs to, platform admins included', async () => {
		await signInAs('g-leila');
		await expect(
			services.exports.rawValues({ classroomId: 'cls_trinity' }, ALL)
		).rejects.toMatchObject({
			code: 'not_found'
		});
		await expect(services.exports.rawCsv({ classroomId: 'cls_trinity' })).rejects.toMatchObject({
			code: 'not_found'
		});
		await signInAs('g-marcus');
		const page = await services.exports.rawValues(
			{ classroomId: 'cls_trinity' },
			{ page: 1, pageSize: 5 }
		);
		expect(page.rows).toHaveLength(5);
		expect(page.total).toBeGreaterThan(5);
		expect(page.rows.every((r) => r.labId === 'lab_trinity')).toBe(true);
		expect(page.terms.licence).toBe('CC BY 4.0');
	});

	it('pages the values: each page starts where the last one ended', async () => {
		await signInAs('g-marcus');
		const query = { classroomId: 'cls_trinity' };
		const all = await services.exports.rawValues(query, ALL);
		const second = await services.exports.rawValues(query, { page: 2, pageSize: 5 });
		expect(second.rows.map((r) => r.id)).toEqual(all.rows.slice(5, 10).map((r) => r.id));
		expect([second.page, second.pageSize, second.total]).toEqual([2, 5, all.total]);
	});

	it('includes field observations as their own source', async () => {
		await signInAbebe();
		await services.labs.submitObservation({
			templateId: 'tpl_species_sighting',
			labId: 'lab_trinity',
			deviceId: null,
			observedAt: '2026-10-20T07:00:00.000Z',
			values: { species_name: 'Hooded vulture', species_count: 3 },
			location: { lat: 9.05, lng: 38.76 }
		});
		const page = await services.exports.rawValues(
			{
				classroomId: 'cls_trinity',
				sourceId: 'field'
			},
			ALL
		);
		const mine = page.rows.filter((r) => r.observedAt === '2026-10-20T07:00:00.000Z');
		expect(mine.map((r) => [r.parameterId, r.value, r.sourceName])).toEqual([
			['species_count', 3, 'Field observation'],
			['species_name', 'Hooded vulture', 'Field observation']
		]);
		expect(page.options.sources).toContainEqual({
			id: 'field',
			name: 'Field observation',
			kind: 'field'
		});
	});

	it('rounds positions where the terms ask, in the table and the file', async () => {
		await signInAs('g-marcus');
		const page = await services.exports.rawValues({ classroomId: 'cls_hargeisa' }, ALL);
		expect(page.terms.coordinateDecimals).toBe(2);
		for (const { location } of page.rows) {
			expect(Math.round(location.lat * 100) / 100).toBe(location.lat);
			expect(Math.round(location.lng * 100) / 100).toBe(location.lng);
		}
	});

	it('exports the filtered rows as CSV with the licence, credit and restrictions', async () => {
		await signInAs('g-marcus');
		const query = { classroomId: 'cls_nassau', parameterId: 'air_temperature' };
		const page = await services.exports.rawValues(query, ALL);
		const file = await services.exports.rawCsv(query);
		expect(file.filename).toMatch(
			/^dagrouper-bay-street-classroom-all-sources-20261022-0900-[a-z0-9]{6}\.csv$/
		);
		const again = await services.exports.rawCsv(query);
		expect(again.filename).not.toBe(file.filename);
		const [source] = page.options.sources.filter((s) => s.kind === 'device');
		const one = await services.exports.rawCsv({ ...query, sourceId: source.id });
		expect(one.filename).toContain(`-${source.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-`);
		expect(file.rows).toBe(page.total);
		const lines = file.content.trimEnd().split('\r\n');
		expect(lines[0]).toBe(CSV_COLUMNS.join(','));
		expect(lines).toHaveLength(page.total + 1);
		for (const line of lines.slice(1)) {
			expect(line).toContain(',air_temperature,');
			expect(line).toMatch(/CC BY-NC 4\.0,.*Bay Street Classroom.*,No commercial use\.$/);
		}
	});
});
