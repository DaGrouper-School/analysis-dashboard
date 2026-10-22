import type { AlertRuleInput, AlertScope, DeviceAlerts } from '#lib/domain/types.ts';
import { checkAlertRule, effectiveRules, type StoredAlertRule } from '#lib/domain/alerts.ts';
import { newId, readDb, updateDb, type MockDbState, type StoredDevice } from '#lib/data/mock-db.ts';
import { ServiceError, type LabService } from '../types';
import { currentUser, permissionsFor, roleIn } from './classrooms';

/**
 * Alert levels per device (AP-08). For each parameter the first layer with rules is in force:
 * the device's own, its classroom's for its sensor type, then the platform defaults.
 */

function classroomIdOf(state: MockDbState, device: StoredDevice) {
	return state.labs.find((l) => l.id === device.labId)!.classroomId;
}

/** Does a stored rule belong to this device's layer at `scope`? */
function inLayer(state: MockDbState, device: StoredDevice, scope: AlertScope) {
	const classroomId = classroomIdOf(state, device);
	return (rule: StoredAlertRule) =>
		scope === 'device'
			? rule.scope === 'device' && rule.deviceId === device.id
			: rule.scope === 'type' &&
				rule.classroomId === classroomId &&
				rule.deviceType === device.type;
}

/** The rules in force for a device, with who last changed each. */
export function deviceRules(state: MockDbState, device: StoredDevice) {
	return effectiveRules(
		state.alertDefaults,
		state.alertRules.filter(inLayer(state, device, 'type')),
		state.alertRules.filter(inLayer(state, device, 'device')),
		(id) => state.users.find((u) => u.id === id)?.fullName ?? 'Unknown'
	);
}

export function createMockAlerts({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): Pick<LabService, 'listDeviceAlerts' | 'saveAlert' | 'deleteAlert' | 'resetAlerts'> {
	const iso = () => new Date(now()).toISOString();

	/** The device, if the user may see it, and what they may change. Hidden devices read as not found. */
	function access(state: MockDbState, deviceId: string) {
		const user = currentUser(state);
		const device = state.devices.find((d) => d.id === deviceId);
		const role = device ? roleIn(state, classroomIdOf(state, device), user.id) : null;
		const can = permissionsFor(user, role);
		if (!device || !can.view) throw new ServiceError('not_found', 'Device not found.');
		return { user, device, can };
	}

	function editor(state: MockDbState, deviceId: string, scope: AlertScope) {
		const found = access(state, deviceId);
		if (scope === 'device' && !found.can.manageDeviceAlerts) {
			throw new ServiceError(
				'forbidden',
				'Only the classroom’s institution head and facilitators can change a device’s alerts.'
			);
		}
		if (scope === 'type' && !found.can.manageTypeAlerts) {
			throw new ServiceError(
				'forbidden',
				'Only the institution head sets alerts for every sensor of a type in the classroom.'
			);
		}
		return found;
	}

	function result(state: MockDbState, device: StoredDevice): DeviceAlerts {
		const user = currentUser(state);
		const lab = state.labs.find((l) => l.id === device.labId)!;
		const can = permissionsFor(user, roleIn(state, lab.classroomId, user.id));
		return structuredClone({
			deviceId: device.id,
			deviceName: device.name,
			deviceType: device.type,
			classroomName: state.classrooms.find((c) => c.id === lab.classroomId)?.name ?? '',
			rules: deviceRules(state, device),
			canEditDevice: can.manageDeviceAlerts,
			canEditType: can.manageTypeAlerts
		});
	}

	/**
	 * A layer's rules for a parameter. The first change copies the levels the layer replaces (the
	 * type's for a device, the defaults for a type), so changing one leaves the others as they
	 * were. Returns the copies' IDs keyed by the IDs they were copied from.
	 */
	function layerRules(
		state: MockDbState,
		device: StoredDevice,
		scope: AlertScope,
		parameterId: string,
		userId: string
	) {
		const copies = new Map<string, string>();
		const mine = inLayer(state, device, scope);
		if (state.alertRules.some((r) => mine(r) && r.parameterId === parameterId)) return copies;
		const below =
			scope === 'device'
				? deviceRules(state, device).filter((r) => r.source !== 'device')
				: state.alertDefaults;
		for (const rule of below.filter((r) => r.parameterId === parameterId)) {
			const id = newId('alr');
			copies.set(rule.id, id);
			state.alertRules.push({
				id,
				parameterId,
				direction: rule.direction,
				value: rule.value,
				level: rule.level,
				title: rule.title,
				enabled: rule.enabled,
				updatedBy: userId,
				updatedAt: iso(),
				...(scope === 'device'
					? { scope, deviceId: device.id }
					: { scope, classroomId: classroomIdOf(state, device), deviceType: device.type })
			});
		}
		return copies;
	}

	return {
		async listDeviceAlerts(deviceId) {
			await delay();
			const state = readDb();
			const { device } = access(state, deviceId);
			return result(state, device);
		},

		async saveAlert(deviceId, scope, input, ruleId) {
			await delay();
			return updateDb((state) => {
				const { user, device } = editor(state, deviceId, scope);
				const rule: AlertRuleInput = {
					parameterId: input.parameterId,
					direction: input.direction,
					value: input.value,
					level: input.level,
					title: input.title?.trim() ?? '',
					enabled: input.enabled !== false
				};
				const errors = checkAlertRule(rule, state.parameters);
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				const copies = layerRules(state, device, scope, rule.parameterId, user.id);
				const stamp = { updatedBy: user.id, updatedAt: iso() };
				if (ruleId) {
					const mine = inLayer(state, device, scope);
					const id = copies.get(ruleId) ?? ruleId;
					const stored = state.alertRules.find((r) => mine(r) && r.id === id);
					if (!stored) throw new ServiceError('not_found', 'That alert no longer exists.');
					if (stored.parameterId !== rule.parameterId) {
						const message = 'An alert stays on its value. Add a new one for another value.';
						throw new ServiceError('invalid_input', message, { parameterId: message });
					}
					Object.assign(stored, rule, stamp);
				} else {
					state.alertRules.push({
						...rule,
						...stamp,
						id: newId('alr'),
						...(scope === 'device'
							? { scope, deviceId: device.id }
							: { scope, classroomId: classroomIdOf(state, device), deviceType: device.type })
					});
				}
				return result(state, device);
			});
		},

		async deleteAlert(deviceId, ruleId) {
			await delay();
			return updateDb((state) => {
				const { device } = access(state, deviceId);
				const stored = state.alertRules.find(
					(r) =>
						r.id === ruleId &&
						(inLayer(state, device, 'device')(r) || inLayer(state, device, 'type')(r))
				);
				if (!stored) {
					throw new ServiceError(
						'not_found',
						'Platform defaults can’t be deleted. Turn one off instead.'
					);
				}
				editor(state, deviceId, stored.scope);
				state.alertRules = state.alertRules.filter((r) => r !== stored);
				return result(state, device);
			});
		},

		async resetAlerts(deviceId, scope, parameterId) {
			await delay();
			return updateDb((state) => {
				const { device } = editor(state, deviceId, scope);
				const mine = inLayer(state, device, scope);
				state.alertRules = state.alertRules.filter(
					(r) => !(mine(r) && r.parameterId === parameterId)
				);
				return result(state, device);
			});
		}
	};
}
