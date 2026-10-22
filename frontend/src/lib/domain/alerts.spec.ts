import { describe, expect, it } from 'vitest';
import { createSeed } from '#lib/data/seed.ts';
import type { AlertRule, AlertRuleInput } from './types';
import {
	alertsFor,
	checkAlertRule,
	effectiveRules,
	thresholdStatus,
	thresholdsFor,
	type ClassroomRuleRecord
} from './alerts';

const { alertDefaults: defaults, parameters } = createSeed();
const platform = effectiveRules(defaults, []);

const own = (rule: Partial<AlertRule> & Pick<AlertRule, 'parameterId'>): ClassroomRuleRecord => ({
	id: `alr_${rule.parameterId}_${rule.level ?? 'warning'}`,
	direction: 'above',
	value: 5,
	level: 'warning',
	title: 'UV',
	enabled: true,
	classroomId: 'cls_trinity',
	updatedBy: 'usr_tesfaye',
	updatedAt: '2026-10-21T09:00:00.000Z',
	...rule
});

describe('platform default levels', () => {
	it('raises one alert per parameter, the most severe one, as a plain statement', () => {
		expect(alertsFor({ values: { uv: 9.1 } }, platform, parameters)).toEqual([
			{
				parameterId: 'uv',
				level: 'warning',
				title: 'Very high UV',
				message: 'UV index 9.1 is at or above the default warning level of 8.'
			}
		]);
		expect(alertsFor({ values: { uv: 6.8 } }, platform, parameters)[0]).toMatchObject({
			level: 'caution'
		});
	});

	it('stays quiet for normal readings and ignores text values', () => {
		const values = { uv: 3.4, air_temperature: 27, pressure_trend: 'steady' };
		expect(alertsFor({ values }, platform, parameters)).toEqual([]);
	});

	it('checks below as well as above, most severe first', () => {
		const titles = alertsFor(
			{ values: { soil_moisture: 12, rain_gauge_daily_accumulation: 61 } },
			platform,
			parameters
		).map((a) => a.title);
		expect(titles).toEqual(['Heavy rain', 'Dry soil']);
	});

	it('rates a value against its levels for a badge', () => {
		expect(thresholdStatus(platform, 'uv', 9.1)).toEqual({
			level: 'warning',
			title: 'Very high UV'
		});
		expect(thresholdStatus(platform, 'uv', 3)).toEqual({ level: 'normal', title: 'No alert' });
		expect(thresholdStatus(platform, 'relative_humidity', 60)).toBeNull();
		expect(thresholdStatus(platform, 'uv', undefined)).toBeNull();
	});
});

describe("a classroom's own levels", () => {
	it('replace the defaults for that parameter only', () => {
		const rules = effectiveRules(
			defaults,
			[own({ parameterId: 'uv', value: 5 })],
			() => 'Tesfaye G.'
		);
		expect(thresholdsFor(rules, 'uv')).toEqual([
			expect.objectContaining({ value: 5, source: 'classroom', updatedBy: 'Tesfaye G.' })
		]);
		expect(thresholdsFor(rules, 'heat_index')[0]).toMatchObject({ source: 'default' });
		expect(alertsFor({ values: { uv: 6 } }, rules, parameters)[0].message).toBe(
			"UV index 6 is at or above this classroom's warning level of 5."
		);
	});

	it('raise nothing while turned off, and a parameter with all off is silent', () => {
		const rules = effectiveRules(defaults, [own({ parameterId: 'uv', enabled: false })]);
		expect(alertsFor({ values: { uv: 11 } }, rules, parameters)).toEqual([]);
		expect(thresholdStatus(rules, 'uv', 11)).toBeNull();
		expect(thresholdsFor(rules, 'uv')).toEqual([]);
	});
});

describe('checking a level', () => {
	const input: AlertRuleInput = {
		parameterId: 'air_temperature',
		direction: 'above',
		value: 30,
		level: 'caution',
		title: 'Hot',
		enabled: true
	};

	it('accepts a level in range', () => {
		expect(checkAlertRule(input, parameters)).toEqual({});
	});

	it('refuses text values, out-of-range levels and missing names', () => {
		expect(checkAlertRule({ ...input, parameterId: 'pressure_trend' }, parameters)).toEqual({
			parameterId: 'Pressure trend is not a number, so it cannot have a level.'
		});
		expect(checkAlertRule({ ...input, value: 80, title: ' ' }, parameters)).toEqual({
			value: 'Air temperature should be -60 to 60 °C.',
			title: 'Give the alert a short name.'
		});
		expect(checkAlertRule({ ...input, value: Number.NaN }, parameters).value).toBe(
			'The level must be a number.'
		);
	});
});
