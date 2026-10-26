import { coreReadings } from '#lib/core-readings.ts';
import { readDb } from '#lib/data/mock-db.ts';
import type { PublicClassroom } from '#lib/domain/types.ts';
import { coarsen } from '#lib/raw-data.ts';
import type { PublicService } from '../types';

/** Public positions are never finer than this, whatever a classroom's terms allow. */
const PUBLIC_DECIMALS = 2;

/** The public site needs no session: everything here is open to visitors (AP-20). */
export function createMockPublic(delay: () => Promise<void>): PublicService {
	return {
		async home() {
			await delay();
			const state = readDb();
			const classrooms = state.classrooms.map((classroom): PublicClassroom => {
				const labs = state.labs.filter((l) => l.classroomId === classroom.id);
				const labIds = new Set(labs.map((l) => l.id));
				const deviceIds = new Set(
					state.devices.filter((d) => labIds.has(d.labId)).map((d) => d.id)
				);
				const records = [
					...state.readings.filter((r) => deviceIds.has(r.deviceId)),
					...state.observations.filter((o) => labIds.has(o.labId))
				];
				const lab = labs[0];
				const decimals = Math.min(
					classroom.terms.coordinateDecimals ?? PUBLIC_DECIMALS,
					PUBLIC_DECIMALS
				);
				return {
					id: classroom.id,
					name: classroom.name,
					institutionName: classroom.institutionName,
					placeName: lab?.placeName ?? null,
					country: lab?.country ?? null,
					location: lab ? coarsen(lab.location, decimals) : null,
					stageId: classroom.stageId,
					core: coreReadings(records, classroom.releasedThrough, state.coreParameters)
				};
			});
			const unitOf = (ids: string[]) => state.parameters.find((p) => p.id === ids[0])?.unit ?? '';
			return {
				content: structuredClone(state.siteContent),
				units: {
					rain: unitOf(state.coreParameters.rainDaily),
					temperature: unitOf(state.coreParameters.airTemperature)
				},
				classrooms: classrooms.sort((a, b) => a.name.localeCompare(b.name))
			};
		},

		async content() {
			await delay();
			return structuredClone(readDb().siteContent);
		}
	};
}
