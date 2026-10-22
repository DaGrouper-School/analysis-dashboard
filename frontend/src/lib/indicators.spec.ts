import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { createSeed } from '#lib/data/seed.ts';
import type { Parameter } from '#lib/domain/types.ts';
import { createMockServices } from '#lib/services/mock/index.ts';
import { spanText, trendOf, trendText } from './indicators';

const { parameters, alertDefaults, observationTemplates } = createSeed();
const byId = (id: string) => parameters.find((p) => p.id === id)!;

const DAY = 86_400_000;
const series = (values: (number | undefined)[]) =>
	values.map((v, i): { observedAt: string; values: Record<string, number> } => ({
		observedAt: new Date(Date.UTC(2026, 9, 1) + i * DAY).toISOString(),
		values: v === undefined ? {} : { air_temperature: v }
	}));

describe('trends', () => {
	const air = byId('air_temperature');

	it('reads rising and falling across the readings, in any order', () => {
		const rising = trendOf(series([20, 21, 22, 23]).reverse(), air)!;
		expect(rising.direction).toBe('rising');
		expect(rising.change).toBeCloseTo(3);
		expect(trendText(rising, '°C')).toBe('Rising, up 3°C over 3 days');
		const falling = trendOf(series([24, 22, 20]), air)!;
		expect(trendText(falling, '°C')).toBe('Falling, down 4°C over 2 days');
	});

	it('reads a change inside the sensor noise as steady', () => {
		const steady = trendOf(series([20, 20.2, 19.9, 20.1]), air)!;
		expect(steady.direction).toBe('steady');
		expect(trendText(steady, '°C')).toBe('Steady over 3 days');
	});

	it('needs two numeric readings', () => {
		expect(trendOf(series([20, undefined]), air)).toBeNull();
		expect(trendOf([], air)).toBeNull();
	});

	it('words the span', () => {
		expect(spanText(DAY)).toBe('1 day');
		expect(spanText(5 * 3_600_000)).toBe('5 hours');
		expect(spanText(40 * 60_000)).toBe('40 minutes');
	});
});

describe('registry text', () => {
	const ADVICE =
		/\b(should|must|avoid|drink|wear|cover up|stay (inside|indoors)|limit|seek|sunscreen|recommend\w*|take care|be careful|protect)\b/i;
	const CAUSE = /\b(because|due to|caus(e|es|ed|ing)|leads? to|results? in|so that)\b/i;

	const texts = [
		...parameters.flatMap((p) => [
			[`${p.id} plain`, p.plain ?? ''],
			[`${p.id} definition`, p.definition]
		]),
		...observationTemplates.map((t) => [`${t.id} description`, t.description])
	];

	it('gives no advice and claims no cause', () => {
		const flagged = texts.filter(([, text]) => ADVICE.test(text) || CAUSE.test(text));
		expect(flagged).toEqual([]);
	});

	it('keeps alert titles to what was measured', () => {
		expect(alertDefaults.filter((r) => ADVICE.test(r.title) || CAUSE.test(r.title))).toEqual([]);
	});

	it('has plain text for the dashboard values', () => {
		for (const id of [
			'air_temperature',
			'relative_humidity',
			'wind_avg',
			'precip_accum_local_day',
			'uv',
			'station_pressure',
			'heat_index',
			'feels_like',
			'dew_point'
		]) {
			const parameter: Parameter = byId(id);
			expect(parameter.plain, id).toBeTruthy();
		}
	});
});

describe('how a value is calculated', () => {
	const services = createMockServices({ latency: 0 });
	beforeEach(() => resetDb());

	it('names the formula, inputs and SAM version, for anyone signed in', async () => {
		await services.auth.signInWithGoogle('g-marcus');
		const heat = await services.registry.getCalculation('heat_index');
		expect(heat).toMatchObject({
			method: 'derived',
			version: 'SAM v1.4',
			publishedAt: '2026-10-01',
			parameter: { id: 'heat_index', label: 'Heat index', source: 'weather_station' },
			inputDetails: [
				{ id: 'air_temperature', label: 'Air temperature', unit: '°C' },
				{ id: 'relative_humidity', label: 'Relative humidity', unit: '%' }
			]
		});
		expect(await services.registry.getCalculation('rain_gauge_daily_accumulation')).toMatchObject({
			method: 'manual',
			inputDetails: []
		});
		await expect(services.registry.getCalculation('nope')).rejects.toMatchObject({
			code: 'not_found'
		});
	});
});
