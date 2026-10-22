import { describe, expect, it } from 'vitest';
import type { DataTerms, RawRow } from '#lib/domain/types.ts';
import {
	CSV_COLUMNS,
	coarsen,
	csvField,
	csvFilename,
	filterRows,
	sortRows,
	toCsv
} from './raw-data';

const row = (over: Partial<RawRow>): RawRow => ({
	id: 'r1:air_temperature',
	observedAt: '2026-10-20T08:00:00.000Z',
	receivedAt: '2026-10-20T08:00:05.000Z',
	parameterId: 'air_temperature',
	value: 21.5,
	sourceId: 'dev_a',
	sourceName: 'Station A',
	sourceKind: 'device',
	labId: 'lab_a',
	siteName: 'North lab',
	location: { lat: 9.03, lng: 38.74 },
	locationSource: 'device',
	creditedTo: null,
	...over
});

const rows = [
	row({ id: 'a' }),
	row({
		id: 'b',
		observedAt: '2026-10-18T12:00:00.000Z',
		parameterId: 'uv',
		sourceId: 'field',
		sourceName: 'Field observation',
		sourceKind: 'field',
		labId: 'lab_b',
		siteName: 'Annex'
	}),
	row({ id: 'c', observedAt: '2026-10-21T23:59:00.000Z', parameterId: 'uv' })
];
const labels: Record<string, string> = { air_temperature: 'Air temperature', uv: 'UV index' };
const labelOf = (id: string) => labels[id] ?? id;

describe('raw data filters', () => {
	it('includes whole days at both ends', () => {
		const ids = (q: Parameters<typeof filterRows>[1]) => filterRows(rows, q).map((r) => r.id);
		expect(ids({ classroomId: 'c', from: '2026-10-20', to: '2026-10-21' })).toEqual(['a', 'c']);
		expect(ids({ classroomId: 'c', to: '2026-10-18' })).toEqual(['b']);
	});

	it('filters by parameter, source and site together', () => {
		expect(filterRows(rows, { classroomId: 'c', parameterId: 'uv' }).map((r) => r.id)).toEqual([
			'b',
			'c'
		]);
		expect(
			filterRows(rows, { classroomId: 'c', parameterId: 'uv', sourceId: 'dev_a' }).map((r) => r.id)
		).toEqual(['c']);
		expect(filterRows(rows, { classroomId: 'c', labId: 'lab_b' }).map((r) => r.id)).toEqual(['b']);
	});
});

describe('raw data sorting', () => {
	it('shows newest first by default and flips with the order', () => {
		expect(sortRows(rows, {}, labelOf).map((r) => r.id)).toEqual(['c', 'a', 'b']);
		expect(sortRows(rows, { order: 'asc' }, labelOf).map((r) => r.id)).toEqual(['b', 'a', 'c']);
	});

	it('sorts by parameter label, source and site, A to Z first, newest first on ties', () => {
		expect(sortRows(rows, { sort: 'parameter' }, labelOf).map((r) => r.id)).toEqual([
			'a',
			'c',
			'b'
		]);
		expect(sortRows(rows, { sort: 'source', order: 'desc' }, labelOf).map((r) => r.id)).toEqual([
			'c',
			'a',
			'b'
		]);
		expect(sortRows(rows, { sort: 'site' }, labelOf).map((r) => r.id)).toEqual(['b', 'c', 'a']);
	});
});

describe('CSV', () => {
	it('quotes what needs it and defuses spreadsheet formulas', () => {
		expect(csvField(-3.5)).toBe('-3.5');
		expect(csvField('-3.5')).toBe('-3.5');
		expect(csvField('a, "b"')).toBe('"a, ""b"""');
		expect(csvField('=HYPERLINK("x")')).toBe(`"'=HYPERLINK(""x"")"`);
		expect(csvField(null)).toBe('');
	});

	it('writes one value per line with the terms on every line', () => {
		const terms: DataTerms = {
			licence: 'CC BY-NC 4.0',
			licenceUrl: 'https://creativecommons.org/licenses/by-nc/4.0/',
			attribution: 'Bay Street Classroom, Nassau.',
			restrictions: ['No commercial use.', 'Ask before publishing.'],
			coordinateDecimals: null
		};
		const params = new Map([['air_temperature', { label: 'Air temperature', unit: '°C' }]]);
		const lines = toCsv([rows[0], rows[1]], 'Bay Street Classroom', terms, params)
			.trimEnd()
			.split('\r\n');
		expect(lines[0]).toBe(CSV_COLUMNS.join(','));
		expect(lines).toHaveLength(3);
		const first = lines[1].split(',');
		expect(first.slice(2, 6)).toEqual(['air_temperature', 'Air temperature', '21.5', '°C']);
		for (const line of lines.slice(1)) {
			expect(line).toContain(
				'CC BY-NC 4.0,https://creativecommons.org/licenses/by-nc/4.0/,"Bay Street Classroom, Nassau.",No commercial use. Ask before publishing.'
			);
		}
		expect(lines[2]).toContain('Field observation,field,Annex');
	});

	it('rounds positions when the terms ask, and names the file after the classroom', () => {
		expect(coarsen({ lat: 9.5627, lng: 44.0651 }, 2)).toEqual({ lat: 9.56, lng: 44.07 });
		expect(coarsen({ lat: 9.5627, lng: 44.0651 }, null)).toEqual({ lat: 9.5627, lng: 44.0651 });
		const at = new Date('2026-10-22T09:30:15.000Z');
		expect(csvFilename('Bay Street Classroom!', null, at, 'k3f9q2')).toBe(
			'dagrouper-bay-street-classroom-all-sources-20261022-0930-k3f9q2.csv'
		);
		expect(csvFilename('Bay Street Classroom', 'Bay St. Rain Gauge', at, 'a1b2c3')).toBe(
			'dagrouper-bay-street-classroom-bay-st-rain-gauge-20261022-0930-a1b2c3.csv'
		);
	});
});
