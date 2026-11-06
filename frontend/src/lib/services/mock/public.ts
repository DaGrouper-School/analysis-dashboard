import { coreReadings } from '#lib/core-readings.ts';
import { readDb, type MockDbState, type StoredClassroom } from '#lib/data/mock-db.ts';
import type {
	PublicClassroom,
	PublicDomainCard,
	PublicSource,
	PublicValue
} from '#lib/domain/types.ts';
import { coarsen, FIELD_SOURCE } from '#lib/raw-data.ts';
import { ServiceError, type PublicService } from '../types';

/** Public positions are never finer than this, whatever a classroom's terms allow. */
const PUBLIC_DECIMALS = 2;

/**
 * A classroom's released records, each with its source (AP-20, AP-21). Records observed after
 * `releasedThrough` are still in review and never reach visitors; with nothing released, none do.
 */
function released(state: MockDbState, classroom: StoredClassroom) {
	const labs = state.labs.filter((l) => l.classroomId === classroom.id);
	const labIds = new Set(labs.map((l) => l.id));
	const devices = state.devices.filter((d) => labIds.has(d.labId));
	const all = [
		...state.readings.flatMap((r) => {
			const device = devices.find((d) => d.id === r.deviceId);
			if (!device) return [];
			const source: PublicSource = { id: device.id, name: device.name, kind: 'device' };
			return [{ record: r, source }];
		}),
		...state.observations
			.filter((o) => labIds.has(o.labId))
			.map((o) => ({
				record: o,
				source: { id: FIELD_SOURCE, name: null, kind: 'field' } as PublicSource
			}))
	];
	const through = classroom.releasedThrough;
	return {
		labs,
		all,
		released: through === null ? [] : all.filter((r) => r.record.observedAt <= through)
	};
}

function toPublicClassroom(state: MockDbState, classroom: StoredClassroom): PublicClassroom {
	const { labs, all } = released(state, classroom);
	const lab = labs[0];
	const decimals = Math.min(classroom.terms.coordinateDecimals ?? PUBLIC_DECIMALS, PUBLIC_DECIMALS);
	return {
		id: classroom.id,
		name: classroom.name,
		institutionName: classroom.institutionName,
		placeName: lab?.placeName ?? null,
		country: lab?.country ?? null,
		location: lab ? coarsen(lab.location, decimals) : null,
		stageId: classroom.stageId,
		core: coreReadings(
			all.map((r) => r.record),
			classroom.releasedThrough,
			state.coreParameters
		)
	};
}

function units(state: MockDbState) {
	const unitOf = (ids: string[]) => state.parameters.find((p) => p.id === ids[0])?.unit ?? '';
	return {
		rain: unitOf(state.coreParameters.rainDaily),
		temperature: unitOf(state.coreParameters.airTemperature)
	};
}

function findClassroom(state: MockDbState, classroomId: string) {
	const classroom = state.classrooms.find((c) => c.id === classroomId);
	if (!classroom) throw new ServiceError('not_found', 'Classroom not found.');
	return classroom;
}

/** Whether the signed-in viewer, if any, belongs to the classroom. Visitors are never members. */
function isMember(state: MockDbState, classroomId: string) {
	const userId = state.session?.userId;
	return (
		!!userId && state.memberships.some((m) => m.classroomId === classroomId && m.userId === userId)
	);
}

function domainOf(state: MockDbState, parameterId: string) {
	const domain = state.domains.find(
		(d) => d.id === state.parameters.find((p) => p.id === parameterId)?.domain
	);
	return domain
		? { id: domain.id, label: domain.label, icon: domain.icon, description: domain.description }
		: null;
}

/** The public site needs no session: everything here is open to visitors (AP-20, AP-21). */
export function createMockPublic(delay: () => Promise<void>): PublicService {
	return {
		async home() {
			await delay();
			const state = readDb();
			return {
				content: structuredClone(state.siteContent),
				units: units(state),
				classrooms: state.classrooms
					.map((c) => toPublicClassroom(state, c))
					.sort((a, b) => a.name.localeCompare(b.name))
			};
		},

		async content() {
			await delay();
			return structuredClone(readDb().siteContent);
		},

		async station(classroomId) {
			await delay();
			const state = readDb();
			const classroom = findClassroom(state, classroomId);
			// The latest released value of each parameter from each source.
			const latest = new Map<string, PublicValue>();
			for (const { record, source } of released(state, classroom).released) {
				for (const [parameterId, value] of Object.entries(record.values)) {
					const key = `${source.id}|${parameterId}`;
					const seen = latest.get(key);
					if (!seen || record.observedAt > seen.observedAt) {
						latest.set(key, { parameterId, value, observedAt: record.observedAt, source });
					}
				}
			}
			const known = new Set(state.parameters.map((p) => p.id));
			const values = [...latest.values()].filter((v) => known.has(v.parameterId));
			const order = (id: string) => state.parameters.findIndex((p) => p.id === id);
			const cards: PublicDomainCard[] = [
				...state.domains.map((d) => ({
					domain: { id: d.id, label: d.label, icon: d.icon, description: d.description },
					values: values.filter((v) => domainOf(state, v.parameterId)?.id === d.id)
				})),
				{ domain: null, values: values.filter((v) => !domainOf(state, v.parameterId)) }
			]
				.map((card) => ({
					...card,
					values: card.values.sort(
						(a, b) =>
							order(a.parameterId) - order(b.parameterId) ||
							(a.source.name ?? '').localeCompare(b.source.name ?? '')
					)
				}))
				.filter((card) => card.values.length);
			const shown = new Set(values.map((v) => v.parameterId));
			return {
				content: structuredClone(state.siteContent),
				classroom: toPublicClassroom(state, classroom),
				units: units(state),
				parameters: structuredClone(state.parameters.filter((p) => shown.has(p.id))),
				domains: structuredClone(cards),
				member: isMember(state, classroom.id)
			};
		},

		async parameter(classroomId, parameterId) {
			await delay();
			const state = readDb();
			const classroom = findClassroom(state, classroomId);
			const parameter = state.parameters.find((p) => p.id === parameterId);
			const records = released(state, classroom).released.filter(
				(r) => r.record.values[parameterId] !== undefined
			);
			if (!parameter || !records.length) {
				throw new ServiceError('not_found', 'This classroom has no released values for that.');
			}
			const bySource = new Map<string, PublicParameterSeries>();
			for (const { record, source } of records) {
				const series = bySource.get(source.id) ?? { source, points: [] };
				series.points.push({ observedAt: record.observedAt, value: record.values[parameterId] });
				bySource.set(source.id, series);
			}
			const domain = domainOf(state, parameterId);
			return {
				content: structuredClone(state.siteContent),
				classroom: toPublicClassroom(state, classroom),
				parameter: structuredClone(parameter),
				domain: domain && { id: domain.id, label: domain.label, icon: domain.icon },
				series: [...bySource.values()]
					.map((s) => ({
						...s,
						points: s.points.sort((a, b) => a.observedAt.localeCompare(b.observedAt))
					}))
					.sort((a, b) => (a.source.name ?? '').localeCompare(b.source.name ?? '')),
				member: isMember(state, classroom.id)
			};
		}
	};
}

type PublicParameterSeries = {
	source: PublicSource;
	points: { observedAt: string; value: number | string }[];
};
