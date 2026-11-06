import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import type { ObservationInput, SyncRecord } from '#lib/domain/types.ts';
import { createMockServices } from './index';
import { SYNC_BATCH_LIMIT } from './labs';

const NOW = Date.parse('2026-10-20T09:00:00.000Z');
let clock = NOW;
const services = createMockServices({ latency: 0, now: () => clock });

beforeEach(() => {
	resetDb();
	clock = NOW;
});

async function signInAbebe() {
	const challenge = await services.auth.requestPhoneCode('+251911000001');
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

const rain = (mm: number, observedAt = '2026-10-20T07:00:00.000Z'): ObservationInput => ({
	templateId: 'tpl_rain_gauge',
	labId: 'lab_trinity',
	deviceId: 'dev_shashamane_gauge',
	observedAt,
	values: { rain_gauge_daily_accumulation: mm },
	location: null
});

const record = (clientId: string, observation: ObservationInput): SyncRecord => ({
	clientId,
	observation,
	credit: false
});

const sentAt = (offsetMs = 0) => new Date(NOW + offsetMs).toISOString();

describe('syncing observations saved offline (AP-16)', () => {
	it('gives each record its own result', async () => {
		await signInAbebe();
		const result = await services.labs.syncObservations({
			sentAt: sentAt(),
			records: [record('c1', rain(4)), record('c2', rain(900)), record('c3', rain(2))]
		});
		expect(result.clockOffsetMs).toBe(0);
		expect(result.results.map((r) => [r.clientId, r.outcome])).toEqual([
			['c1', 'stored'],
			['c2', 'rejected'],
			['c3', 'stored']
		]);
		expect(result.results[1]).toMatchObject({
			observation: null,
			error: 'Check the highlighted fields.',
			fields: { rain_gauge_daily_accumulation: expect.any(String) }
		});
		const readings = await services.labs.deviceReadings('dev_shashamane_gauge');
		const ids = result.results.flatMap((r) => (r.observation ? [r.observation.id] : []));
		expect(readings.map((r) => r.id)).toEqual(expect.arrayContaining(ids));
	});

	it('never stores a retried record twice', async () => {
		await signInAbebe();
		const batch = { sentAt: sentAt(), records: [record('c1', rain(4))] };
		const first = await services.labs.syncObservations(batch);
		const before = (await services.labs.deviceReadings('dev_shashamane_gauge', 1000)).length;
		const again = await services.labs.syncObservations(batch);
		expect(again.results[0].outcome).toBe('duplicate');
		expect(again.results[0].observation?.id).toBe(first.results[0].observation?.id);
		expect((await services.labs.deviceReadings('dev_shashamane_gauge', 1000)).length).toBe(before);
	});

	it('dedupes field observations too, and keeps the device ID off what pages see', async () => {
		await signInAbebe();
		const field = record('f1', {
			templateId: 'tpl_species_sighting',
			labId: 'lab_trinity',
			deviceId: null,
			observedAt: '2026-10-20T07:00:00.000Z',
			values: { species_name: 'Hooded vulture', species_count: 3 },
			location: null
		});
		await services.labs.syncObservations({ sentAt: sentAt(), records: [field] });
		const again = await services.labs.syncObservations({ sentAt: sentAt(), records: [field] });
		expect(again.results[0]).toMatchObject({
			outcome: 'duplicate',
			observation: { deviceId: null }
		});
		const detail = await services.labs.get('lab_trinity');
		// Stored once, beside the seeded observations.
		expect(
			detail.observations.filter((o) => o.observedAt === '2026-10-20T07:00:00.000Z')
		).toHaveLength(1);
		const [reading] = await services.labs.deviceReadings('dev_shashamane_gauge');
		expect(reading).not.toHaveProperty('clientId');
	});

	it('flags a device clock that is off, without refusing the records', async () => {
		await signInAbebe();
		// The device thinks it is 40 minutes later than it is.
		const result = await services.labs.syncObservations({
			sentAt: sentAt(40 * 60_000),
			records: [record('c1', rain(4, sentAt(30 * 60_000)))]
		});
		expect(result.clockOffsetMs).toBe(-40 * 60_000);
		expect(result.results[0]).toMatchObject({
			outcome: 'stored',
			clockFlags: ['device_clock_off']
		});
	});

	it('flags an observation dated after the device sent it', async () => {
		await signInAbebe();
		const result = await services.labs.syncObservations({
			sentAt: sentAt(),
			records: [record('c1', rain(4, sentAt(3 * 3_600_000)))]
		});
		expect(result.results[0]).toMatchObject({
			outcome: 'stored',
			clockFlags: ['observed_after_sent']
		});
		// A retry reports the same flags.
		const again = await services.labs.syncObservations({
			sentAt: sentAt(),
			records: [record('c1', rain(4, sentAt(3 * 3_600_000)))]
		});
		expect(again.results[0].clockFlags).toEqual(['observed_after_sent']);
	});

	it('refuses a record per record when the user may not record there', async () => {
		await services.auth.signInWithGoogle('g-marcus');
		const result = await services.labs.syncObservations({
			sentAt: sentAt(),
			records: [record('c1', rain(4))]
		});
		expect(result.results[0]).toMatchObject({ outcome: 'rejected', error: expect.any(String) });
	});

	it('needs a session, and caps the batch size', async () => {
		await expect(
			services.labs.syncObservations({ sentAt: sentAt(), records: [record('c1', rain(4))] })
		).rejects.toMatchObject({ code: 'unauthenticated' });
		await signInAbebe();
		const records = Array.from({ length: SYNC_BATCH_LIMIT + 1 }, (_, i) =>
			record(`c${i}`, rain(1))
		);
		await expect(
			services.labs.syncObservations({ sentAt: sentAt(), records })
		).rejects.toMatchObject({ code: 'invalid_input' });
	});
});
