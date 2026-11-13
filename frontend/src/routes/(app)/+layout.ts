import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { contextClassroom } from '#lib/utils/classroom-context.ts';
import { services } from '#lib/services/index.ts';
import { nextRoute } from '#lib/utils/routing.ts';
import type { ClassroomAlert } from '#lib/features/alerts/AlertsList.svelte';
import type { MemberConsent, ReleaseReview } from '#lib/domain/types.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, url, depends }) => {
	const { user } = await parent();
	if (!user?.onboardingCompleted) redirect(307, resolve(nextRoute(user)));
	// A sync, a release or a recorded consent changes what needs the user in the top bar.
	depends('app:classroom-context', 'app:classroom');
	// The shared registry: every page can name device types, domains and values.
	const [deviceTypes, domains, parameters, mine, conditions, invitations] = await Promise.all([
		services.registry.listDeviceTypes(),
		services.registry.listDomains(),
		services.registry.listParameters(),
		services.classrooms.listMine(),
		services.labs.currentConditions(),
		services.classrooms.listMyInvitations()
	]);
	// The classrooms the top bar picks from: ones the user belongs to, as those have data.
	const memberClassrooms = mine.filter((c) => c.myRole !== null);
	// Alerts and messages, in the top bar on every page: what needs the user in each classroom.
	const inbox = (
		await Promise.all(
			memberClassrooms.map(async (classroom) => {
				const [consent, release] = await Promise.all([
					// Only the head and facilitators see guardian consent; everyone else sees none.
					services.classrooms.listConsent(classroom.id).catch((): MemberConsent[] => []),
					// What waits for review, for whoever may release it; nobody else is told.
					services.classrooms.releaseReview(classroom.id).catch((): ReleaseReview | null => null)
				]);
				// Each device's active alerts, evaluated by the backend against its levels (AP-08).
				const alerts: ClassroomAlert[] = conditions
					.filter((c) => c.reading.classroomId === classroom.id)
					.flatMap((c) =>
						c.alerts.map((alert) => ({
							...alert,
							device: c.device,
							observedAt: c.reading.observedAt
						}))
					);
				return {
					classroom: { id: classroom.id, name: classroom.name },
					alerts,
					waiting: consent.filter((m) => m.status === 'pending'),
					// Released by hand and shown publicly: readings waiting for review need someone.
					release:
						release && !release.auto && release.showPublic && release.pending ? release : null
				};
			})
		)
	).filter((c) => c.alerts.length || c.waiting.length || c.release);
	return {
		user,
		deviceTypes,
		domains,
		parameters,
		memberClassrooms,
		inbox,
		invitations,
		// Record is for members who record readings; researchers only read.
		canRecord: memberClassrooms.some((c) => c.canRecord),
		contextClassroom: contextClassroom(memberClassrooms, url)
	};
};
