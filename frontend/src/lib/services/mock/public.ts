import { monthCompleteness } from '#lib/completeness.ts';
import { coreReadings, stationCore } from '#lib/core-readings.ts';
import { readDb, type MockDbState, type StoredClassroom } from '#lib/data/mock-db.ts';
import type {
	InstitutionOverview,
	KitItem,
	PublicClassroom,
	PublicClassroomProfile,
	PublicContribution,
	PublicDevice,
	PublicDomainCard,
	PublicParameterSeries,
	PublicSource,
	PublicValue,
	Reading
} from '#lib/domain/types.ts';
import { coarsen, FIELD_SOURCE } from '#lib/raw-data.ts';
import { deviceSummary, last24Hours, nowTiles, readingTiles } from '#lib/station-now.ts';
import { keyValues } from '#lib/key-values.ts';
import { deviceTypeColor, deviceTypeIcon, deviceTypeOf } from '#lib/labels.ts';
import { ServiceError, type PublicService } from '../types';
import { permissionsFor, roleIn } from './classrooms';

/** A contributor profile lists this many of their latest values. */
const CONTRIBUTOR_RECENT = 20;

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

/**
 * Each device with released data (AP-45): its key values from its newest released reading, with
 * their last 24 hours. Colour and icon come from the registry's device types.
 */
function publicDevices(
	state: MockDbState,
	shown: ReturnType<typeof released>['released']
): PublicDevice[] {
	const parameters = new Map(state.parameters.map((p) => [p.id, p]));
	const domainIcons = new Map(state.domains.map((d) => [d.id, d.icon]));
	return state.devices.flatMap((device) => {
		const readings = shown
			.filter((r) => r.source.id === device.id)
			.map((r) => r.record)
			.sort((a, b) => b.observedAt.localeCompare(a.observedAt));
		if (!readings.length) return [];
		const icon = deviceTypeIcon(state.deviceTypes, device.type);
		const type = deviceTypeOf(state.deviceTypes, device.type);
		const values = readings[0].values as Reading['values'];
		return [
			{
				id: device.id,
				name: device.name,
				type: device.type,
				color: deviceTypeColor(state.deviceTypes, device.type),
				icon,
				tiles: readingTiles(
					readings,
					keyValues(type?.tileParameterIds, values, parameters, { domainIcons }),
					icon
				),
				summary: deviceSummary(
					readings,
					keyValues(type?.cardParameterIds, values, parameters, { domainIcons, fallback: false })
				)
			}
		];
	});
}

function toPublicClassroom(state: MockDbState, classroom: StoredClassroom): PublicClassroom {
	const { labs, all } = released(state, classroom);
	const lab = labs[0];
	const decimals = Math.min(classroom.terms.coordinateDecimals ?? PUBLIC_DECIMALS, PUBLIC_DECIMALS);
	return {
		id: classroom.id,
		name: classroom.name,
		// The host is named only once it has signed the host agreement.
		institutionName: classroom.profile.hostAgreementSigned ? classroom.institutionName : null,
		placeName: lab?.placeName ?? null,
		country: lab?.country ?? null,
		location: lab ? coarsen(lab.location, decimals) : null,
		stageId: classroom.stageId,
		regionId: classroom.regionId,
		themes: labs.flatMap((l) => l.siteProfile.researchQuestions),
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

/** The Lab in a Bag, read from the devices registered to the classroom's labs. */
function kitFor(state: MockDbState, labs: { id: string }[], member: boolean): KitItem[] {
	const labelOf = (id: string) => state.parameters.find((p) => p.id === id)?.label ?? id;
	const labIds = new Set(labs.map((l) => l.id));
	const devices = state.devices.filter((d) => labIds.has(d.labId));
	return state.deviceTypes
		.map((type) => ({ type, devices: devices.filter((d) => d.type === type.id) }))
		.filter((k) => k.devices.length)
		.map(({ type, devices }) => {
			// What it measures, by domain: a station sends dozens of values. Values with no
			// domain yet are named one by one.
			const domains = state.domains
				.filter((d) => type.parameterIds.some((id) => domainOf(state, id)?.id === d.id))
				.map((d) => d.label);
			const loose = type.parameterIds.filter((id) => !domainOf(state, id)).map(labelOf);
			return {
				deviceType: { id: type.id, label: type.label, icon: type.icon },
				count: devices.length,
				measures: [...domains, ...loose],
				serials: member ? devices.map((d) => d.identifier) : []
			};
		});
}

/**
 * The institution overview card (AP-43). The host is shown only once it has signed the host
 * agreement, and only when there is a photo or a history to show.
 */
function institutionFor(
	state: MockDbState,
	classroom: StoredClassroom,
	publicClassroom: PublicClassroom,
	kit: KitItem[]
): InstitutionOverview | null {
	const info = classroom.profile.institution;
	// Shown once a host agreement is signed, even before a photo or history is added.
	if (!publicClassroom.institutionName) return null;
	return {
		...structuredClone(info),
		name: publicClassroom.institutionName,
		location: publicClassroom.location,
		deviceTypes: kit.map((k) => k.deviceType.label)
	};
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
			const { all, released: shownRecords } = released(state, classroom);
			// The latest released value of each parameter from each source, and the last reading of
			// each day for its sparkline.
			const latest = new Map<string, PublicValue>();
			const daily = new Map<string, Map<string, { at: string; value: number }>>();
			for (const { record, source } of shownRecords) {
				for (const [parameterId, value] of Object.entries(record.values)) {
					const key = `${source.id}|${parameterId}`;
					const seen = latest.get(key);
					if (!seen || record.observedAt > seen.observedAt) {
						latest.set(key, {
							parameterId,
							value,
							observedAt: record.observedAt,
							source,
							last7Days: []
						});
					}
					if (typeof value !== 'number') continue;
					const days = daily.get(key) ?? new Map<string, { at: string; value: number }>();
					const day = record.observedAt.slice(0, 10);
					const last = days.get(day);
					if (!last || record.observedAt > last.at) days.set(day, { at: record.observedAt, value });
					daily.set(key, days);
				}
			}
			for (const [key, value] of latest) {
				const days = daily.get(key);
				if (!days) continue;
				const end = Date.parse(`${value.observedAt.slice(0, 10)}T00:00:00.000Z`);
				value.last7Days = Array.from({ length: 7 }, (_, i) => {
					const day = new Date(end - (6 - i) * 86_400_000).toISOString().slice(0, 10);
					return days.get(day)?.value ?? null;
				});
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
			const publicClassroom = toPublicClassroom(state, classroom);
			const member = isMember(state, classroom.id);
			const sites = classroom.profile.projects.flatMap((p) => p.sites);
			const expectations = sites.flatMap((site) =>
				site.parameterIds.map((parameterId) => ({
					parameterId,
					readingsPerDay: site.readingsPerDay
				}))
			);
			// Completeness counts every reading that arrived, released or not: it says whether data
			// came in, never what it says.
			const completeness =
				publicClassroom.core.day && expectations.length
					? monthCompleteness(
							all.map((r) => r.record),
							publicClassroom.core.day,
							expectations,
							(id) => {
								const domain = domainOf(state, id);
								return domain ? { id: domain.id, label: domain.label, icon: domain.icon } : null;
							}
						)
					: null;
			return {
				content: structuredClone(state.siteContent),
				classroom: publicClassroom,
				units: units(state),
				parameters: structuredClone(state.parameters.filter((p) => shown.has(p.id))),
				domains: structuredClone(cards),
				core: stationCore(
					all.map((r) => r.record),
					classroom.releasedThrough,
					state.coreParameters,
					(id) =>
						id === FIELD_SOURCE ? null : (state.devices.find((d) => d.id === id)?.name ?? null)
				),
				completeness,
				siteCodes: member ? sites.map((s) => s.code) : [],
				member,
				places: state.classrooms
					.map((c) => toPublicClassroom(state, c))
					.filter((c) => c.location)
					.sort((a, b) => a.name.localeCompare(b.name))
					.map(({ id, name, placeName, country, location, stageId, regionId }) => ({
						id,
						name,
						placeName,
						country,
						location,
						stageId,
						regionId
					})),
				now: nowTiles(state.nowTiles, values, state.parameters, publicClassroom.core).map(
					(tile) => {
						const shown = values.find(
							(v) => v.parameterId === tile.parameterId && v.observedAt === tile.observedAt
						);
						if (!shown || !tile.observedAt) return tile;
						const records = shownRecords
							.filter((r) => r.source.id === shown.source.id)
							.map((r) => r.record);
						return {
							...tile,
							last24Hours: last24Hours(records, [shown.parameterId], tile.observedAt)
						};
					}
				),
				devices: publicDevices(state, shownRecords),
				institution: institutionFor(
					state,
					classroom,
					publicClassroom,
					kitFor(state, released(state, classroom).labs, member)
				)
			};
		},

		async profile(classroomId) {
			await delay();
			const state = readDb();
			const classroom = findClassroom(state, classroomId);
			const member = isMember(state, classroom.id);
			const user = state.session && state.users.find((u) => u.id === state.session?.userId);
			const can = user ? permissionsFor(user, roleIn(state, classroom.id, user.id)) : null;
			const { labs, released: records } = released(state, classroom);
			const labelOf = (id: string) => state.parameters.find((p) => p.id === id)?.label ?? id;

			const kit = kitFor(state, labs, member);

			// Everyone who recorded released data, by participant ID (AP-39). Only those who turned
			// on their public profile are named; a minor never can.
			const recorders = new Set<string>();
			for (const { record } of records) {
				const by = record.recordedByUserId ?? record.creditedToUserId;
				if (by) recorders.add(by);
			}
			const contributors = [...recorders]
				.map((id) => state.users.find((u) => u.id === id))
				.filter((u) => !!u)
				.map((u) => ({
					participantId: u.participantId,
					name: u.publicProfile && !u.minor ? u.fullName : null
				}))
				.sort(
					(a, b) =>
						Number(b.name !== null) - Number(a.name !== null) ||
						(a.name ?? '').localeCompare(b.name ?? '') ||
						a.participantId - b.participantId
				);

			const now = new Date().toISOString();
			const { profile } = classroom;
			const result: PublicClassroomProfile = {
				content: structuredClone(state.siteContent),
				classroom: toPublicClassroom(state, classroom),
				profile: {
					areaCode: profile.areaCode,
					language: profile.language,
					description: profile.description,
					ecosystems: [...profile.ecosystems]
				},
				projects: profile.projects.map((project) => ({
					id: project.id,
					title: project.title,
					question: project.question,
					sites: project.sites.map(({ parameterIds, ...site }) => ({
						...site,
						readings: parameterIds.map(labelOf)
					}))
				})),
				kit,
				institution: institutionFor(state, classroom, toPublicClassroom(state, classroom), kit),
				custodianRole: member ? profile.custodianRole : null,
				sessions: profile.sessions
					.filter((s) => s.startsAt >= now)
					.sort((a, b) => a.startsAt.localeCompare(b.startsAt))
					.map((s) => ({ ...s })),
				contributors,
				member,
				canManage: !!can && (can.manageMembers || can.invite.length > 0)
			};
			return result;
		},

		async contributor(participantId) {
			await delay();
			const state = readDb();
			const user = state.users.find((u) => u.participantId === participantId);
			const missing = () => new ServiceError('not_found', 'Contributor not found.');
			if (!user) throw missing();
			const contributions: PublicContribution[] = [];
			const classrooms = new Map<string, string>();
			for (const classroom of state.classrooms) {
				for (const { record } of released(state, classroom).released) {
					if ((record.recordedByUserId ?? record.creditedToUserId) !== user.id) continue;
					classrooms.set(classroom.id, classroom.name);
					for (const [parameterId, value] of Object.entries(record.values)) {
						const param = state.parameters.find((p) => p.id === parameterId);
						if (!param) continue;
						contributions.push({
							classroomId: classroom.id,
							classroomName: classroom.name,
							parameterId,
							parameterLabel: param.label,
							unit: param.unit,
							valueType: param.valueType,
							value,
							observedAt: record.observedAt
						});
					}
				}
			}
			if (!contributions.length) throw missing();
			contributions.sort((a, b) => b.observedAt.localeCompare(a.observedAt));
			return {
				content: structuredClone(state.siteContent),
				participantId: user.participantId,
				// Shown only by their own choice in Settings, and never for a minor.
				profile:
					user.publicProfile && !user.minor
						? {
								fullName: user.fullName,
								email: user.email,
								role: user.role,
								avatarUrl: user.avatarUrl
							}
						: null,
				classrooms: [...classrooms].map(([id, name]) => ({ id, name })),
				total: contributions.length,
				recent: contributions.slice(0, CONTRIBUTOR_RECENT)
			};
		},

		async parameter(classroomId, parameterId) {
			await delay();
			const state = readDb();
			const classroom = findClassroom(state, classroomId);
			const parameter = state.parameters.find((p) => p.id === parameterId);
			// Rain from a gauge and from a station is the same rain: the page shows them together.
			const rainIds = state.coreParameters.rainDaily;
			const isRain = rainIds.includes(parameterId);
			const ids = isRain
				? [parameterId, ...rainIds.filter((id) => id !== parameterId)]
				: [parameterId];
			const reports = (values: Reading['values']) => ids.find((id) => values[id] !== undefined);
			const { labs, all, released: shown } = released(state, classroom);
			if (!parameter || !shown.some((r) => reports(r.record.values))) {
				throw new ServiceError('not_found', 'This classroom has no released values for that.');
			}

			const bySource = new Map<string, PublicParameterSeries>();
			for (const { record, source } of shown) {
				const id = reports(record.values);
				if (!id) continue;
				const series = bySource.get(source.id) ?? { source, parameterId: id, points: [] };
				series.points.push({ observedAt: record.observedAt, value: record.values[id] });
				bySource.set(source.id, series);
			}
			const manual = (series: PublicParameterSeries) =>
				state.parameters.find((p) => p.id === series.parameterId)?.source === 'manual';
			const reference = isRain ? ([...bySource.values()].find(manual) ?? null) : null;
			const series = [...bySource.values()]
				.map((s) => ({
					...s,
					points: s.points.sort((a, b) => a.observedAt.localeCompare(b.observedAt))
				}))
				.sort(
					(a, b) =>
						Number(b.source.id === reference?.source.id) -
							Number(a.source.id === reference?.source.id) ||
						(a.source.name ?? '').localeCompare(b.source.name ?? '')
				);

			const through = classroom.releasedThrough;
			const deviceIds = new Set(series.map((s) => s.source.id));
			const typeIds = new Set(state.devices.filter((d) => deviceIds.has(d.id)).map((d) => d.type));
			const domain = domainOf(state, parameterId);
			return {
				content: structuredClone(state.siteContent),
				classroom: toPublicClassroom(state, classroom),
				parameter: structuredClone(parameter),
				domain: domain && { id: domain.id, label: domain.label, icon: domain.icon },
				note: labs.map((l) => l.siteProfile.parameterNotes?.[parameterId]).find(Boolean) ?? null,
				series,
				rain: isRain
					? {
							referenceSourceId: reference?.source.id ?? null,
							tolerance: state.coreParameters.rainTolerance
						}
					: null,
				releasedThrough: through,
				inReview: all.some(
					(r) => (through === null || r.record.observedAt > through) && reports(r.record.values)
				),
				compare: state.classrooms
					.filter(
						(c) =>
							c.id !== classroom.id &&
							released(state, c).released.some((r) => reports(r.record.values))
					)
					.map((c) => {
						const { id, name, placeName, country } = toPublicClassroom(state, c);
						return { id, name, placeName, country };
					})
					.sort((a, b) => a.name.localeCompare(b.name)),
				deviceTypes: state.deviceTypes.filter((t) => typeIds.has(t.id)).map((t) => t.label),
				terms: {
					licence: classroom.terms.licence,
					licenceUrl: classroom.terms.licenceUrl,
					attribution: classroom.terms.attribution,
					restrictions: [...classroom.terms.restrictions]
				},
				member: isMember(state, classroom.id)
			};
		}
	};
}
