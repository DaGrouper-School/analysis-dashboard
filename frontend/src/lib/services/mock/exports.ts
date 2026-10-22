import type { RawPage, RawQuery, RawRow } from '#lib/domain/types.ts';
import { readDb, type MockDbState } from '#lib/data/mock-db.ts';
import { FIELD_SOURCE, coarsen, csvFilename, filterRows, sortRows, toCsv } from '#lib/raw-data.ts';
import { ServiceError, type ExportService } from '../types';
import { currentUser, roleIn } from './classrooms';

/**
 * The classroom when the user belongs to it. Membership only, as for current conditions:
 * a platform admin who is not a member exports nothing, and a hidden classroom reads as not found.
 */
function memberClassroom(state: MockDbState, classroomId: string) {
	const user = currentUser(state);
	const classroom = state.classrooms.find((c) => c.id === classroomId);
	if (!classroom || roleIn(state, classroomId, user.id) === null) {
		throw new ServiceError('not_found', 'Classroom not found.');
	}
	return classroom;
}

/** Every value the classroom's devices and field observations sent, one row each. */
function classroomRows(state: MockDbState, classroomId: string): RawRow[] {
	const { terms } = state.classrooms.find((c) => c.id === classroomId)!;
	const labs = new Map(
		state.labs.filter((l) => l.classroomId === classroomId).map((l) => [l.id, l])
	);
	const devices = new Map(state.devices.filter((d) => labs.has(d.labId)).map((d) => [d.id, d]));
	const nameOf = (userId: string | null) =>
		userId ? (state.users.find((u) => u.id === userId)?.fullName ?? null) : null;
	const records = [
		...state.readings.filter((r) => devices.has(r.deviceId)),
		...state.observations.filter((o) => labs.has(o.labId))
	];
	return records.flatMap((record) => {
		const device = record.deviceId ? devices.get(record.deviceId) : undefined;
		const location = coarsen(record.location, terms.coordinateDecimals);
		return Object.entries(record.values).map(([parameterId, value]): RawRow => ({
			id: `${record.id}:${parameterId}`,
			observedAt: record.observedAt,
			receivedAt: record.receivedAt,
			parameterId,
			value,
			sourceId: device?.id ?? FIELD_SOURCE,
			sourceName: device?.name ?? 'Field observation',
			sourceKind: device ? 'device' : 'field',
			labId: record.labId,
			siteName: labs.get(record.labId)!.name,
			location,
			locationSource: record.locationSource,
			creditedTo: nameOf(record.creditedToUserId)
		}));
	});
}

function matching(state: MockDbState, query: RawQuery) {
	const classroom = memberClassroom(state, query.classroomId);
	const all = classroomRows(state, classroom.id);
	const labels = new Map(state.parameters.map((p) => [p.id, p.label]));
	const rows = sortRows(filterRows(all, query), query, (id) => labels.get(id) ?? id);
	return { classroom, all, rows };
}

export function createMockExports({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): ExportService {
	return {
		async rawValues(query, limit = 200) {
			await delay();
			const state = readDb();
			const { classroom, all, rows } = matching(state, query);
			const sources = new Map<string, RawPage['options']['sources'][number]>();
			const sites = new Map<string, string>();
			for (const row of all) {
				sources.set(row.sourceId, { id: row.sourceId, name: row.sourceName, kind: row.sourceKind });
				sites.set(row.labId, row.siteName);
			}
			const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name);
			return {
				classroomId: classroom.id,
				classroomName: classroom.name,
				rows: structuredClone(rows.slice(0, limit)),
				total: rows.length,
				terms: structuredClone(classroom.terms),
				options: {
					parameterIds: [...new Set(all.map((r) => r.parameterId))],
					sources: [...sources.values()].sort(byName),
					sites: [...sites].map(([id, name]) => ({ id, name })).sort(byName)
				}
			};
		},

		async rawCsv(query) {
			await delay();
			const state = readDb();
			const { classroom, rows } = matching(state, query);
			const parameters = new Map(state.parameters.map((p) => [p.id, p]));
			return {
				filename: csvFilename(classroom.name, new Date(now()).toISOString().slice(0, 10)),
				content: toCsv(rows, classroom.name, classroom.terms, parameters),
				rows: rows.length
			};
		}
	};
}
