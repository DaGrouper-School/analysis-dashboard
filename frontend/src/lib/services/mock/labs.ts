import type {
	CreateLabInput,
	CurrentConditions,
	Device,
	Lab,
	Observation,
	Reading
} from '#lib/domain/types.ts';
import { alertsFor } from '#lib/domain/alerts.ts';
import {
	isValidGeoPoint,
	validateDevice,
	validateLab,
	validateReading
} from '#lib/domain/validation.ts';
import { checkObservation, withDeviceTemplates } from '#lib/observations.ts';
import { newId, readDb, updateDb, type MockDbState, type StoredDevice } from '#lib/data/mock-db.ts';
import { ServiceError, type LabService } from '../types';
import { currentUser, permissionsFor, roleIn } from './classrooms';

function classroomOf(state: MockDbState, lab: Lab) {
	return state.classrooms.find((c) => c.id === lab.classroomId)!;
}

function sourceIdFor(state: MockDbState, device: StoredDevice): string {
	const lab = state.labs.find((l) => l.id === device.labId)!;
	return `${classroomOf(state, lab).code}:${device.identifier}`;
}

function toDevice(state: MockDbState, device: StoredDevice, showSource: boolean): Device {
	const lab = state.labs.find((l) => l.id === device.labId)!;
	return {
		id: device.id,
		labId: device.labId,
		labName: lab.name,
		name: device.name,
		type: device.type,
		identifier: device.identifier,
		samSourceId: showSource ? sourceIdFor(state, device) : null,
		registeredAt: device.registeredAt
	};
}

/**
 * The stored reading without the SAM source ID (it contains the classroom code), and with the
 * recorder's current name when they opted in to credit.
 */
function toReading(state: MockDbState, stored: MockDbState['readings'][number]): Reading {
	const reading: Partial<MockDbState['readings'][number]> = structuredClone(stored);
	delete reading.samSourceId;
	delete reading.creditedToUserId;
	const credited = stored.creditedToUserId;
	const creditedTo = credited
		? (state.users.find((u) => u.id === credited)?.fullName ?? null)
		: null;
	return { ...(reading as Omit<Reading, 'creditedTo'>), creditedTo };
}

/** A stored field observation, shown the same way as a reading. */
function toObservation(
	state: MockDbState,
	stored: MockDbState['observations'][number]
): Observation {
	const credited = stored.creditedToUserId;
	return {
		id: stored.id,
		templateId: stored.templateId,
		deviceId: null,
		labId: stored.labId,
		classroomId: stored.classroomId,
		observedAt: stored.observedAt,
		receivedAt: stored.receivedAt,
		values: { ...stored.values },
		location: { ...stored.location },
		locationSource: stored.locationSource,
		creditedTo: credited ? (state.users.find((u) => u.id === credited)?.fullName ?? null) : null
	};
}

function newestFirst(a: { observedAt: string }, b: { observedAt: string }) {
	return b.observedAt.localeCompare(a.observedAt);
}

function cleanLab(input: CreateLabInput) {
	return {
		name: input.name.trim(),
		placeName: input.placeName.trim(),
		country: input.country.trim(),
		location: { ...input.location },
		siteProfile: {
			setting: input.siteProfile.setting,
			description: input.siteProfile.description.trim(),
			researchQuestions: input.siteProfile.researchQuestions.map((q) => q.trim()).filter(Boolean)
		}
	};
}

function checkLab(input: CreateLabInput) {
	const errors = validateLab(input);
	if (Object.keys(errors).length) {
		throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
	}
}

/** The lab plus what the user may do with it. Hidden labs read as not found. */
function accessLab(state: MockDbState, labId: string) {
	const user = currentUser(state);
	const lab = state.labs.find((l) => l.id === labId);
	const can = lab && permissionsFor(user, roleIn(state, lab.classroomId, user.id));
	if (!lab || !can?.view) throw new ServiceError('not_found', 'Lab not found.');
	// Focal people (head, facilitator) register devices: the same people who can invite.
	return { user, lab, can, canManage: can.invite.length > 0 };
}

/**
 * Every reading from devices in classrooms the user is a member of, newest first. Membership
 * only: admins and everyone else see nothing from classrooms they are not in.
 */
function memberReadings(state: MockDbState): CurrentConditions[] {
	const user = currentUser(state);
	const result: CurrentConditions[] = [];
	for (const device of state.devices) {
		const lab = state.labs.find((l) => l.id === device.labId)!;
		const role = roleIn(state, lab.classroomId, user.id);
		if (role === null) continue;
		const shown = toDevice(state, device, permissionsFor(user, role).seeCode);
		const classroomName = classroomOf(state, lab).name;
		for (const r of state.readings) {
			if (r.deviceId === device.id) {
				result.push({ device: shown, classroomName, reading: toReading(state, r), alerts: [] });
			}
		}
	}
	return result.sort((a, b) => newestFirst(a.reading, b.reading));
}

export function createMockLabs({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): LabService {
	const iso = () => new Date(now()).toISOString();

	return {
		async listForClassroom(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			if (!permissionsFor(user, roleIn(state, classroomId, user.id)).view) {
				throw new ServiceError('not_found', 'Classroom not found.');
			}
			return state.labs.filter((l) => l.classroomId === classroomId);
		},

		async get(labId) {
			await delay();
			const state = readDb();
			const { lab, can, canManage } = accessLab(state, labId);
			const devices = state.devices.filter((d) => d.labId === lab.id);
			return {
				lab,
				classroomName: classroomOf(state, lab).name,
				devices: devices.map((d) => toDevice(state, d, can.seeCode)),
				// Readings of removed devices stay in SAM but leave the lab page.
				readings: state.readings
					.filter((r) => devices.some((d) => d.id === r.deviceId))
					.sort(newestFirst)
					.slice(0, 50)
					.map((r) => toReading(state, r)),
				observations: state.observations
					.filter((o) => o.labId === lab.id)
					.sort(newestFirst)
					.slice(0, 50)
					.map((o) => toObservation(state, o)),
				canManage,
				canRecord: can.recordReadings
			};
		},

		async create(classroomId, input) {
			await delay();
			checkLab(input);
			return updateDb((state) => {
				const user = currentUser(state);
				const can = permissionsFor(user, roleIn(state, classroomId, user.id));
				if (!state.classrooms.some((c) => c.id === classroomId) || !can.view) {
					throw new ServiceError('not_found', 'Classroom not found.');
				}
				if (!can.invite.length) {
					throw new ServiceError(
						'forbidden',
						'Only facilitators and the institution head can add a lab.'
					);
				}
				const lab: Lab = {
					id: newId('lab'),
					classroomId,
					...cleanLab(input),
					createdAt: iso()
				};
				state.labs.push(lab);
				return lab;
			});
		},

		async update(labId, input) {
			await delay();
			checkLab(input);
			return updateDb((state) => {
				const { lab, canManage } = accessLab(state, labId);
				if (!canManage) {
					throw new ServiceError(
						'forbidden',
						'Only facilitators and the institution head can edit the lab.'
					);
				}
				Object.assign(lab, cleanLab(input));
				return lab;
			});
		},

		async registerDevice(labId, input) {
			await delay();
			const errors = validateDevice(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const { lab, canManage } = accessLab(state, labId);
				if (!canManage) {
					throw new ServiceError(
						'forbidden',
						'Only facilitators and the institution head can register devices.'
					);
				}
				if (!state.deviceTypes.some((t) => t.id === input.type)) {
					const message = 'Pick a device type from the registry.';
					throw new ServiceError('invalid_input', message, { type: message });
				}
				const device: StoredDevice = {
					id: newId('dev'),
					labId: lab.id,
					name: input.name.trim(),
					type: input.type,
					identifier: input.identifier.trim().toUpperCase(),
					registeredAt: iso()
				};
				// SAM source identity is the classroom code plus the device identifier (SAM-06).
				const sourceId = sourceIdFor(state, device);
				if (state.samSources.some((s) => s.sourceId === sourceId)) {
					const message = 'A device with this identifier is already registered for this classroom.';
					throw new ServiceError('conflict', message, { identifier: message });
				}
				state.devices.push(device);
				state.samSources.push({ sourceId, kind: 'device', registeredAt: device.registeredAt });
				return toDevice(state, device, true);
			});
		},

		async removeDevice(deviceId) {
			await delay();
			updateDb((state) => {
				const device = state.devices.find((d) => d.id === deviceId);
				if (!device) throw new ServiceError('not_found', 'Device not found.');
				const { canManage } = accessLab(state, device.labId);
				if (!canManage) throw new ServiceError('forbidden', 'You cannot remove this device.');
				const sourceId = sourceIdFor(state, device);
				state.devices = state.devices.filter((d) => d !== device);
				// SAM keeps past observations; the source just stops accepting new ones.
				state.samSources = state.samSources.filter((s) => s.sourceId !== sourceId);
			});
		},

		async uploadReadings(deviceId, inputs, options = { credit: false }) {
			await delay();
			inputs.forEach((input, i) => {
				const errors = validateReading(input);
				if (Object.keys(errors).length) {
					throw new ServiceError(
						'invalid_input',
						`Reading ${i + 1}: ${Object.values(errors)[0]}`,
						errors
					);
				}
			});
			return updateDb((state) => {
				const device = state.devices.find((d) => d.id === deviceId);
				if (!device) throw new ServiceError('not_found', 'Device not found.');
				const { user, lab, can } = accessLab(state, device.labId);
				if (!can.recordReadings) {
					throw new ServiceError(
						'forbidden',
						'You can read this classroom’s data but not record readings.'
					);
				}
				const samSourceId = sourceIdFor(state, device);
				const receivedAt = iso();
				const stored = inputs.map((input) => ({
					id: newId('rdg'),
					deviceId: device.id,
					labId: lab.id,
					classroomId: lab.classroomId,
					observedAt: new Date(input.observedAt).toISOString(),
					receivedAt,
					values: { ...input.values },
					// No GPS from the device: stand in with the lab's location, and say so.
					location: { ...(input.location ?? lab.location) },
					locationSource: input.location ? ('device' as const) : ('lab' as const),
					samSourceId,
					// Credit goes to the person who recorded, and only if they asked for it.
					creditedToUserId: options.credit ? user.id : null
				}));
				// Stand-in for forwarding to SAM: the reading is stored under the device's source.
				state.readings.push(...stored);
				return stored.map((r) => toReading(state, r));
			});
		},

		async currentConditions() {
			await delay();
			const latest = new Map<string, CurrentConditions>();
			// Entries come newest first, so the first one per device is its latest reading.
			for (const entry of memberReadings(readDb())) {
				if (!latest.has(entry.device.id)) {
					latest.set(entry.device.id, { ...entry, alerts: alertsFor(entry.reading) });
				}
			}
			return [...latest.values()];
		},

		async recentReadings(limit = 20) {
			await delay();
			return memberReadings(readDb()).slice(0, limit);
		},

		async listClassroomDevices(classroomId) {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const can = permissionsFor(user, roleIn(state, classroomId, user.id));
			if (!can.view) throw new ServiceError('not_found', 'Classroom not found.');
			const labIds = new Set(
				state.labs.filter((l) => l.classroomId === classroomId).map((l) => l.id)
			);
			return state.devices
				.filter((d) => labIds.has(d.labId))
				.map((d) => toDevice(state, d, can.seeCode));
		},

		async deviceReadings(deviceId, limit = 100) {
			await delay();
			const state = readDb();
			const device = state.devices.find((d) => d.id === deviceId);
			if (!device) throw new ServiceError('not_found', 'Device not found.');
			accessLab(state, device.labId);
			return state.readings
				.filter((r) => r.deviceId === deviceId)
				.sort(newestFirst)
				.slice(0, limit)
				.map((r) => toReading(state, r));
		},

		async observationTargets() {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			return state.labs
				.filter((lab) => {
					const role = roleIn(state, lab.classroomId, user.id);
					// Membership only: admins record in classrooms they belong to, like everyone.
					return role !== null && permissionsFor(user, role).recordReadings;
				})
				.map((lab) => {
					const role = roleIn(state, lab.classroomId, user.id);
					return {
						lab,
						classroomName: classroomOf(state, lab).name,
						devices: state.devices
							.filter((d) => d.labId === lab.id)
							.map((d) => toDevice(state, d, permissionsFor(user, role).seeCode))
					};
				});
		},

		async submitObservation(input, options = { credit: false }) {
			await delay();
			const errors: Record<string, string> = {};
			const at = Date.parse(input.observedAt);
			if (Number.isNaN(at)) errors.observedAt = 'Enter when you observed it.';
			else if (at > now() + 5 * 60_000) errors.observedAt = 'That time is in the future.';
			if (input.location && !isValidGeoPoint(input.location)) {
				errors.location = 'That GPS position is not valid.';
			}
			return updateDb((state) => {
				const { user, lab, can } = accessLab(state, input.labId);
				if (!can.recordReadings) {
					throw new ServiceError(
						'forbidden',
						'You can read this classroom’s data but not record observations.'
					);
				}
				const templates = withDeviceTemplates(
					state.observationTemplates,
					state.deviceTypes,
					state.parameters
				);
				const template = templates.find((t) => t.id === input.templateId);
				if (!template)
					throw new ServiceError('not_found', 'That observation form no longer exists.');
				Object.assign(errors, checkObservation(template, state.parameters, input.values));
				// A device reading needs one of the lab's registered devices of that type. Devices are
				// never added from here: registering one is for facilitators, on the lab page.
				const device = template.deviceType
					? state.devices.find((d) => d.id === input.deviceId && d.labId === lab.id)
					: null;
				if (template.deviceType && device?.type !== template.deviceType) {
					errors.deviceId = 'Choose one of this lab’s registered devices of that type.';
				}
				if (!template.deviceType && input.deviceId) {
					errors.deviceId = 'A field observation is not read off a device.';
				}
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				const base = {
					id: newId('obs'),
					labId: lab.id,
					classroomId: lab.classroomId,
					observedAt: new Date(at).toISOString(),
					receivedAt: iso(),
					values: { ...input.values },
					// No GPS: stand in with the lab's location, and say so.
					location: { ...(input.location ?? lab.location) },
					locationSource: input.location ? ('device' as const) : ('lab' as const),
					creditedToUserId: options.credit ? user.id : null,
					templateId: template.id
				};
				if (device) {
					// A device reading joins the device's readings and its SAM source.
					const stored = { ...base, deviceId: device.id, samSourceId: sourceIdFor(state, device) };
					state.readings.push(stored);
					return { ...toReading(state, stored), templateId: template.id };
				}
				// A field observation goes to SAM under the classroom's own source.
				const stored = {
					...base,
					deviceId: null,
					samSourceId: classroomOf(state, lab).code
				};
				state.observations.push(stored);
				return toObservation(state, stored);
			});
		}
	};
}
