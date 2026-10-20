import type { CreateLabInput, CurrentConditions, Device, Lab, Reading } from '#lib/domain/types.ts';
import { alertsFor } from '#lib/domain/alerts.ts';
import { validateDevice, validateLab, validateReading } from '#lib/domain/validation.ts';
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

/** The stored reading minus the SAM source ID, which contains the classroom code. */
function toReading(stored: MockDbState['readings'][number]): Reading {
	const reading: Partial<MockDbState['readings'][number]> = structuredClone(stored);
	delete reading.samSourceId;
	return reading as Reading;
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
					.map(toReading),
				canManage
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

		async uploadReadings(deviceId, inputs) {
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
				const { lab, canManage } = accessLab(state, device.labId);
				if (!canManage) {
					throw new ServiceError(
						'forbidden',
						'Only facilitators and the institution head can connect devices.'
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
					samSourceId
				}));
				// Stand-in for forwarding to SAM: the reading is stored under the device's source.
				state.readings.push(...stored);
				return stored.map(toReading);
			});
		},

		async currentConditions() {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			const result: CurrentConditions[] = [];
			for (const device of state.devices) {
				const lab = state.labs.find((l) => l.id === device.labId)!;
				// Membership only: admins and everyone else see nothing from classrooms they are not in.
				const role = roleIn(state, lab.classroomId, user.id);
				if (role === null) continue;
				const latest = state.readings.filter((r) => r.deviceId === device.id).sort(newestFirst)[0];
				if (!latest) continue;
				result.push({
					device: toDevice(state, device, permissionsFor(user, role).seeCode),
					classroomName: classroomOf(state, lab).name,
					reading: toReading(latest),
					alerts: alertsFor(latest)
				});
			}
			return result.sort((a, b) => newestFirst(a.reading, b.reading));
		}
	};
}
