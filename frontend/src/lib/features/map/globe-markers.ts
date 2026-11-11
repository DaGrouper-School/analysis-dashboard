import type { GlobeMarker } from '#lib/features/map/Globe.svelte';
import type { ClassroomStage, PublicClassroom } from '#lib/domain/types.ts';
import { stageColor } from '#lib/utils/labels.ts';
import { services } from '#lib/services/index.ts';

/** Every Global Classroom with a place, as a globe marker coloured by its stage (AP-20). */
export function classroomMarkers(
	classrooms: Pick<PublicClassroom, 'id' | 'name' | 'location' | 'stageId'>[],
	stages: ClassroomStage[]
): GlobeMarker[] {
	const byId = new Map(stages.map((s) => [s.id, s]));
	return classrooms
		.filter((c) => c.location)
		.map((c) => ({
			id: c.id,
			lat: c.location!.lat,
			lng: c.location!.lng,
			color: stageColor(stages, c.stageId),
			label: `${c.name} · ${byId.get(c.stageId)?.label ?? c.stageId}`
		}));
}

/** The classrooms' markers from the public site, for a page's backdrop. Empty if it fails. */
export function loadClassroomMarkers(): Promise<GlobeMarker[]> {
	return services.publicSite
		.home()
		.then((home) => classroomMarkers(home.classrooms, home.content.stages))
		.catch(() => []);
}
