import type { Device, Lab } from '#lib/domain/types.ts';
import { validateDevice, validateLab } from '#lib/domain/validation.ts';
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
		location: { ...(device.location ?? lab.location) },
		ownLocation: device.location !== null,
		placeName: device.placeName,
		country: device.country,
		samSourceId: showSource ? sourceIdFor(state, device) : null,
		registeredAt: device.registeredAt
	};
}

/** Public view of a device: no SAM source ID, since that contains the classroom code. */
export function toPublicDevice(state: MockDbState, device: StoredDevice): Device {
	return toDevice(state, device, false);
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
			return {
				lab,
				classroomName: classroomOf(state, lab).name,
				devices: state.devices
					.filter((d) => d.labId === lab.id)
					.map((d) => toDevice(state, d, can.seeCode)),
				canManage
			};
		},

		async create(classroomId, input) {
			await delay();
			const errors = validateLab(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
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
					name: input.name.trim(),
					placeName: input.placeName.trim(),
					country: input.country.trim(),
					location: { ...input.location },
					siteProfile: {
						setting: input.siteProfile.setting,
						description: input.siteProfile.description.trim(),
						researchQuestions: input.siteProfile.researchQuestions
							.map((q) => q.trim())
							.filter(Boolean)
					},
					createdAt: iso()
				};
				state.labs.push(lab);
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
					placeName: input.placeName.trim(),
					country: input.country.trim(),
					location: input.location && { ...input.location },
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

		async listMyDevices() {
			await delay();
			const state = readDb();
			const user = currentUser(state);
			return state.devices
				.filter((d) => {
					const lab = state.labs.find((l) => l.id === d.labId)!;
					return roleIn(state, lab.classroomId, user.id) !== null;
				})
				.map((d) => {
					const lab = state.labs.find((l) => l.id === d.labId)!;
					const role = roleIn(state, lab.classroomId, user.id);
					return toDevice(state, d, permissionsFor(user, role).seeCode);
				});
		}
	};
}
