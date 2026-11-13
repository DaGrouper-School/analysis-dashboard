import { resolve } from '$app/paths';
import type { ClassroomSummary } from '#lib/domain/types.ts';
import { services } from '#lib/services/index.ts';

/**
 * Where someone goes after joining a classroom: an institution head whose institution is not set
 * up yet goes to the institution setup (AP-40); everyone else to `otherwise`.
 */
export async function afterJoining(classroom: ClassroomSummary, otherwise: string) {
	if (classroom.myRole !== 'institution_head') return otherwise;
	const setup = await services.classrooms.institutionSetup(classroom.id).catch(() => null);
	return setup && !setup.completedAt
		? resolve('/(app)/classrooms/[id]/setup', { id: classroom.id })
		: otherwise;
}
