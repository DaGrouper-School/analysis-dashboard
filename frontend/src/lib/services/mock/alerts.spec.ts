import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import type { AlertRuleInput, DeviceAlerts } from '#lib/domain/types.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });
const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);

async function signInAbebe() {
	const challenge = await services.auth.requestPhoneCode('+251911000001');
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

const STATION = 'dev_shashamane_ws';
const uvWarning: AlertRuleInput = {
	parameterId: 'uv',
	direction: 'above',
	value: 5,
	level: 'warning',
	title: 'Very high UV',
	enabled: true
};
const uv = (alerts: DeviceAlerts) => alerts.rules.filter((r) => r.parameterId === 'uv');

/** A second weather station in Trinity, so two of a type share a classroom. */
async function secondStation() {
	await signInAs('g-tesfaye');
	return services.labs.registerDevice('lab_trinity', {
		name: 'Rooftop Station',
		type: 'weather_station',
		identifier: 'tws-roof-01'
	});
}

beforeEach(() => resetDb());

describe('device alert levels (AP-08)', () => {
	it('start on the platform defaults, readable by every member', async () => {
		await signInAs('g-marcus');
		const alerts = await services.labs.listDeviceAlerts(STATION);
		expect(alerts).toMatchObject({
			deviceType: 'weather_station',
			classroomName: expect.any(String),
			canEditDevice: false,
			canEditType: false
		});
		expect(alerts.rules.every((r) => r.source === 'default')).toBe(true);
	});

	it("let the head set a sensor type's levels for the classroom, keeping the other levels", async () => {
		const roof = await secondStation();
		const before = await services.labs.listDeviceAlerts(STATION);
		const warning = uv(before).find((r) => r.level === 'warning')!;
		const after = await services.labs.saveAlert(STATION, 'type', uvWarning, warning.id);
		expect(uv(after).map((r) => [r.level, r.value, r.source])).toEqual([
			['warning', 5, 'type'],
			['caution', 6, 'type']
		]);
		// Every weather station in Trinity has them; Nassau's keeps the defaults.
		expect(uv(await services.labs.listDeviceAlerts(roof.id))[0].source).toBe('type');
		await signInAs('g-marcus');
		expect(uv(await services.labs.listDeviceAlerts('dev_nassau_ws'))[0].source).toBe('default');
	});

	it('let one device differ from another of the same type', async () => {
		const roof = await secondStation();
		const defaults = await services.labs.listDeviceAlerts(STATION);
		const warning = uv(defaults).find((r) => r.level === 'warning')!;
		await services.labs.saveAlert(STATION, 'type', uvWarning, warning.id);
		const typeRule = uv(await services.labs.listDeviceAlerts(roof.id)).find((r) => r.value === 5)!;
		const own = await services.labs.saveAlert(
			roof.id,
			'device',
			{ ...uvWarning, value: 9 },
			typeRule.id
		);
		expect(uv(own).map((r) => [r.source, r.value])).toEqual([
			['device', 9],
			['device', 6]
		]);
		expect(uv(await services.labs.listDeviceAlerts(STATION)).map((r) => r.value)).toEqual([5, 6]);
		// Reset brings the classroom's levels back to the device.
		const reset = await services.labs.resetAlerts(roof.id, 'device', 'uv');
		expect(uv(reset).every((r) => r.source === 'type')).toBe(true);
	});

	it('evaluate current conditions against each device', async () => {
		await signInAs('g-tesfaye');
		await services.labs.saveAlert(STATION, 'device', { ...uvWarning, title: 'UV over 5' });
		const conditions = await services.labs.currentConditions();
		expect(conditions.find((c) => c.device.id === STATION)!.alerts[0]).toMatchObject({
			title: 'UV over 5',
			message: "UV index 6.8 is at or above this device's warning level of 5."
		});
	});

	it('let facilitators set a device but not a sensor type', async () => {
		await signInAs('g-tesfaye');
		await services.classrooms.setMemberRole('cls_trinity', 'usr_abebe', 'facilitator');
		await signInAbebe();
		const alerts = await services.labs.listDeviceAlerts(STATION);
		expect(alerts).toMatchObject({ canEditDevice: true, canEditType: false });
		const saved = await services.labs.saveAlert(STATION, 'device', uvWarning);
		const mine = uv(saved).find((r) => r.value === 5)!;
		const off = await services.labs.saveAlert(
			STATION,
			'device',
			{ ...uvWarning, enabled: false },
			mine.id
		);
		expect(uv(off).find((r) => r.id === mine.id)!.enabled).toBe(false);
		await services.labs.deleteAlert(STATION, mine.id);
		await expect(services.labs.saveAlert(STATION, 'type', uvWarning)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('refuse community scientists, researchers and admins outside the classroom', async () => {
		await signInAbebe();
		await expect(services.labs.saveAlert(STATION, 'device', uvWarning)).rejects.toMatchObject({
			code: 'forbidden'
		});
		await signInAs('g-marcus');
		await expect(services.labs.resetAlerts(STATION, 'device', 'uv')).rejects.toMatchObject({
			code: 'forbidden'
		});
		await signInAs('g-leila');
		expect(await services.labs.listDeviceAlerts(STATION)).toMatchObject({ canEditDevice: false });
		await expect(services.labs.saveAlert(STATION, 'type', uvWarning)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('checks the level against the registry, and keeps defaults undeletable', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.labs.saveAlert(STATION, 'device', {
				...uvWarning,
				parameterId: 'air_temperature',
				value: 90
			})
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { value: 'Air temperature should be -60 to 60 °C.' }
		});
		const defaults = await services.labs.listDeviceAlerts(STATION);
		await expect(services.labs.deleteAlert(STATION, uv(defaults)[0].id)).rejects.toMatchObject({
			code: 'not_found'
		});
	});
});

describe('platform default alert levels (AP-08)', () => {
	it('let platform admins change them, logged, and every device without its own follows', async () => {
		await signInAs('g-leila');
		const defaults = await services.registry.listAlertDefaults();
		const warning = defaults.find((r) => r.parameterId === 'uv' && r.level === 'warning')!;
		const saved = await services.registry.saveAlertDefault(
			{ ...uvWarning, value: 9, title: 'Extreme UV' },
			warning.id
		);
		expect(saved.find((r) => r.id === warning.id)).toMatchObject({ value: 9, title: 'Extreme UV' });
		expect((await services.registry.listChanges())[0].summary).toBe(
			'Changed the default UV index level Warning at or above 8 (Very high UV) to Warning at or above 9 (Extreme UV).'
		);
		await signInAs('g-marcus');
		const station = await services.labs.listDeviceAlerts(STATION);
		expect(uv(station).find((r) => r.level === 'warning')).toMatchObject({
			value: 9,
			source: 'default'
		});
	});

	it('leave classrooms and devices with their own levels as they were', async () => {
		await signInAs('g-tesfaye');
		const before = await services.labs.listDeviceAlerts(STATION);
		await services.labs.saveAlert(STATION, 'type', uvWarning, uv(before)[0].id);
		await signInAs('g-leila');
		const defaults = await services.registry.listAlertDefaults();
		for (const rule of defaults.filter((r) => r.parameterId === 'uv')) {
			await services.registry.deleteAlertDefault(rule.id);
		}
		expect((await services.registry.listAlertDefaults()).some((r) => r.parameterId === 'uv')).toBe(
			false
		);
		await signInAs('g-tesfaye');
		expect(uv(await services.labs.listDeviceAlerts(STATION))[0]).toMatchObject({
			value: 5,
			source: 'type'
		});
	});

	it('add, turn off and check them, for admins only', async () => {
		await signInAs('g-tesfaye');
		await expect(services.registry.saveAlertDefault(uvWarning)).rejects.toMatchObject({
			code: 'forbidden'
		});
		await signInAs('g-leila');
		const added = await services.registry.saveAlertDefault({
			...uvWarning,
			parameterId: 'wind_gust',
			value: 20,
			title: 'Strong gusts'
		});
		const gust = added.find((r) => r.parameterId === 'wind_gust' && r.value === 20)!;
		const off = await services.registry.saveAlertDefault({ ...gust, enabled: false }, gust.id);
		expect(off.find((r) => r.id === gust.id)?.enabled).toBe(false);
		expect((await services.registry.listChanges())[0].summary).toBe(
			'Turned off the default Wind gust level Warning at or above 20 m/s (Strong gusts).'
		);
		await expect(
			services.registry.saveAlertDefault({ ...uvWarning, parameterId: 'pressure_trend' })
		).rejects.toMatchObject({ code: 'invalid_input' });
	});
});
