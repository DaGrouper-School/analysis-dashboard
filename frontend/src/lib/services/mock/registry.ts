import type {
	AlertRuleInput,
	DeviceTypeDefinition,
	DomainDefinition,
	ExpectedValueInput,
	Parameter
} from '#lib/domain/types.ts';
import { checkAlertRule, ruleText } from '#lib/domain/alerts.ts';
import { PARAMETER_ID, validateDeviceType, validateDomain } from '#lib/domain/validation.ts';
import { newId, readDb, updateDb, type MockDbState } from '#lib/data/mock-db.ts';
import { ServiceError, type RegistryService } from '../types';
import { currentUser } from './classrooms';
import { DEVICE_TYPE_PALETTE } from '#lib/labels.ts';
import { FALLBACK_DOMAIN_ICON, FALLBACK_TYPE_ICON } from '#lib/icons.ts';
import { withDeviceTemplates } from '#lib/observations.ts';
import { SOURCE_CALCULATION } from '#lib/data/seed.ts';

/**
 * The shared registry. Reading it needs a signed-in user; changing it needs a platform admin,
 * and every change is logged with who made it (SAM-23).
 */
export function createMockRegistry({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): RegistryService {
	const iso = () => new Date(now()).toISOString();

	function asAdmin(state: MockDbState) {
		const user = currentUser(state);
		if (!user.platformAdmin) {
			throw new ServiceError('forbidden', 'Only platform admins can change the registry.');
		}
		return user;
	}

	function log(state: MockDbState, byName: string, summary: string) {
		state.registryChanges.unshift({ id: newId('chg'), at: iso(), byName, summary });
	}

	function domainOrThrow(state: MockDbState, id: string | null): DomainDefinition | null {
		if (id === null) return null;
		const domain = state.domains.find((d) => d.id === id);
		if (!domain) throw new ServiceError('not_found', 'That domain no longer exists.');
		return domain;
	}

	function invalid(errors: Record<string, string>) {
		if (Object.keys(errors).length) {
			throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
		}
	}

	function labelFree(state: MockDbState, label: string, exceptId: string | null) {
		const taken = state.deviceTypes.some(
			(t) => t.id !== exceptId && t.label.toLowerCase() === label.toLowerCase()
		);
		if (taken) {
			const message = `There is already a ${label} device type.`;
			throw new ServiceError('conflict', message, { label: message });
		}
	}

	/** Adds the values that are new to the registry, and returns their IDs. */
	function addValues(state: MockDbState, values: ExpectedValueInput[]) {
		const added: string[] = [];
		for (const value of values) {
			const id = value.id.trim();
			if (state.parameters.some((p) => p.id === id)) continue;
			domainOrThrow(state, value.domain);
			state.parameters.push({
				id,
				label: value.label.trim(),
				unit: value.unit.trim(),
				definition: '',
				domain: value.domain,
				source: 'device',
				valueType: value.valueType,
				calculation: { ...SOURCE_CALCULATION.device, inputs: [], ...state.registryVersion }
			});
			added.push(id);
		}
		return added;
	}

	/** Moves values that were already in the registry to the domain picked for them, and logs it. */
	function moveValues(
		state: MockDbState,
		values: ExpectedValueInput[],
		added: string[],
		by: string
	) {
		for (const value of values) {
			const parameter = state.parameters.find((p) => p.id === value.id.trim());
			if (!parameter || added.includes(parameter.id) || parameter.domain === value.domain) continue;
			move(state, parameter, value.domain, by);
		}
	}

	function move(state: MockDbState, parameter: Parameter, domainId: string | null, by: string) {
		const domain = domainOrThrow(state, domainId);
		const from = state.domains.find((d) => d.id === parameter.domain)?.label ?? 'no domain';
		parameter.domain = domainId;
		log(
			state,
			by,
			`Moved ${parameter.label} (${parameter.id}) from ${from} to ${domain?.label ?? 'no domain'}.`
		);
	}

	/** `Soil probe v2` becomes `soil_probe_v2`, numbered if it is taken. */
	function typeId(state: MockDbState, label: string) {
		const base = label
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '_')
			.replace(/^_+|_+$/g, '')
			.slice(0, 40);
		const start = /^[a-z]/.test(base) ? base : `type_${base}`;
		let id = start;
		for (let n = 2; state.deviceTypes.some((t) => t.id === id); n++) id = `${start}_${n}`;
		return id;
	}

	return {
		// Copies, so a page never holds the stored objects and sees every change as new data.
		async listParameters() {
			return structuredClone(readDb().parameters);
		},

		async listDomains() {
			return structuredClone(readDb().domains);
		},

		async listDeviceTypes() {
			return structuredClone(readDb().deviceTypes);
		},

		async listObservationTemplates() {
			const state = readDb();
			return structuredClone(
				withDeviceTemplates(state.observationTemplates, state.deviceTypes, state.parameters)
			);
		},

		async getCalculation(parameterId) {
			const state = readDb();
			currentUser(state);
			const parameter = state.parameters.find((p) => p.id === parameterId);
			if (!parameter) throw new ServiceError('not_found', 'That value is not in the registry.');
			const calculation = parameter.calculation ?? {
				...SOURCE_CALCULATION[parameter.source],
				inputs: [],
				...state.registryVersion
			};
			const inputDetails = calculation.inputs.map((id) => {
				const input = state.parameters.find((p) => p.id === id);
				return { id, label: input?.label ?? id, unit: input?.unit ?? '' };
			});
			const { id, label, unit, source, plain } = parameter;
			return structuredClone({
				...calculation,
				parameter: { id, label, unit, source, plain },
				inputDetails
			});
		},

		async listChanges() {
			await delay();
			const state = readDb();
			asAdmin(state);
			return structuredClone(state.registryChanges);
		},

		async createDomain(input) {
			await delay();
			invalid(validateDomain(input));
			return updateDb((state) => {
				const user = asAdmin(state);
				const id = input.id.trim().toUpperCase();
				if (state.domains.some((d) => d.id === id)) {
					const message = `There is already a ${id} domain.`;
					throw new ServiceError('conflict', message, { id: message });
				}
				const domain: DomainDefinition = {
					id,
					label: input.label.trim(),
					icon: input.icon.trim() || FALLBACK_DOMAIN_ICON,
					description: input.description.trim()
				};
				state.domains.push(domain);
				log(state, user.fullName, `Added the ${domain.label} domain (${id}).`);
				return { ...domain };
			});
		},

		async updateDomain(id, input) {
			await delay();
			invalid(validateDomain({ id, ...input }));
			return updateDb((state) => {
				const user = asAdmin(state);
				const domain = domainOrThrow(state, id)!;
				domain.label = input.label.trim();
				domain.icon = input.icon.trim() || domain.icon;
				domain.description = input.description.trim();
				log(state, user.fullName, `Edited the ${domain.label} domain (${id}).`);
				return domain;
			});
		},

		async deleteDomain(id) {
			await delay();
			updateDb((state) => {
				const user = asAdmin(state);
				const domain = domainOrThrow(state, id)!;
				const mapped = state.parameters.filter((p) => p.domain === id).length;
				if (mapped) {
					throw new ServiceError(
						'conflict',
						`${mapped} value${mapped === 1 ? ' is' : 's are'} mapped to ${domain.label}. Move ${mapped === 1 ? 'it' : 'them'} to another domain first.`
					);
				}
				state.domains = state.domains.filter((d) => d.id !== id);
				log(state, user.fullName, `Removed the ${domain.label} domain (${id}).`);
			});
		},

		async createDeviceType(input) {
			await delay();
			invalid(validateDeviceType(input));
			return updateDb((state) => {
				const user = asAdmin(state);
				const label = input.label.trim();
				labelFree(state, label, null);
				const added = addValues(state, input.values);
				moveValues(state, input.values, added, user.fullName);
				const type: DeviceTypeDefinition = {
					id: typeId(state, label),
					label,
					description: input.description.trim(),
					color: input.color || DEVICE_TYPE_PALETTE.at(-1)!,
					icon: input.icon?.trim() || FALLBACK_TYPE_ICON,
					parameterIds: input.values.map((v) => v.id.trim())
				};
				state.deviceTypes.push(type);
				const count = type.parameterIds.length;
				log(
					state,
					user.fullName,
					`Registered the ${label} device type with ${count} value${count === 1 ? '' : 's'}` +
						(added.length ? ` (new: ${added.join(', ')}).` : '.')
				);
				return type;
			});
		},

		async updateDeviceType(id, input) {
			await delay();
			invalid(validateDeviceType(input));
			return updateDb((state) => {
				const user = asAdmin(state);
				const type = state.deviceTypes.find((t) => t.id === id);
				if (!type) throw new ServiceError('not_found', 'That device type no longer exists.');
				const label = input.label.trim();
				labelFree(state, label, id);
				const added = addValues(state, input.values);
				moveValues(state, input.values, added, user.fullName);
				// Values device types brought in can be corrected; station values stay as registered.
				const edited: string[] = [];
				for (const value of input.values) {
					const parameter = state.parameters.find((p) => p.id === value.id.trim())!;
					if (parameter.source !== 'device' || added.includes(parameter.id)) continue;
					const next = {
						label: value.label.trim(),
						unit: value.unit.trim(),
						valueType: value.valueType
					};
					if (
						next.label !== parameter.label ||
						next.unit !== parameter.unit ||
						next.valueType !== (parameter.valueType ?? 'number')
					) {
						Object.assign(parameter, next);
						edited.push(parameter.id);
					}
				}
				const before = type.parameterIds;
				type.label = label;
				type.description = input.description.trim();
				type.color = input.color || type.color;
				type.icon = input.icon?.trim() || type.icon;
				type.parameterIds = input.values.map((v) => v.id.trim());
				const removed = before.filter((p) => !type.parameterIds.includes(p));
				const parts = [
					added.length && `added ${added.join(', ')}`,
					edited.length && `edited ${edited.join(', ')}`,
					removed.length && `dropped ${removed.join(', ')}`
				].filter(Boolean);
				log(
					state,
					user.fullName,
					`Updated the ${label} device type${parts.length ? `: ${parts.join('; ')}` : ''}.`
				);
				return type;
			});
		},

		async mapParameter(parameterId, domainId) {
			await delay();
			if (!PARAMETER_ID.test(parameterId)) {
				throw new ServiceError('not_found', 'That value is not in the registry.');
			}
			return updateDb((state) => {
				const user = asAdmin(state);
				const parameter = state.parameters.find((p) => p.id === parameterId);
				if (!parameter) throw new ServiceError('not_found', 'That value is not in the registry.');
				if (parameter.domain === domainId) return parameter;
				move(state, parameter, domainId, user.fullName);
				return parameter;
			});
		},

		async listAlertDefaults() {
			await delay();
			return structuredClone(readDb().alertDefaults);
		},

		async saveAlertDefault(input, ruleId) {
			await delay();
			return updateDb((state) => {
				const user = asAdmin(state);
				const rule: AlertRuleInput = {
					parameterId: input.parameterId,
					direction: input.direction,
					value: input.value,
					level: input.level,
					title: input.title?.trim() ?? '',
					enabled: input.enabled !== false
				};
				invalid(checkAlertRule(rule, state.parameters));
				const parameter = state.parameters.find((p) => p.id === rule.parameterId)!;
				const what = (r: AlertRuleInput) => `${ruleText(r, parameter.unit)} (${r.title})`;
				if (ruleId) {
					const stored = state.alertDefaults.find((r) => r.id === ruleId);
					if (!stored) throw new ServiceError('not_found', 'That alert no longer exists.');
					if (stored.parameterId !== rule.parameterId) {
						const message = 'An alert stays on its value. Add a new one for another value.';
						throw new ServiceError('invalid_input', message, { parameterId: message });
					}
					const before = what(stored);
					const toggled = stored.enabled !== rule.enabled;
					Object.assign(stored, rule);
					log(
						state,
						user.fullName,
						toggled && before === what(rule)
							? `Turned ${rule.enabled ? 'on' : 'off'} the default ${parameter.label} level ${before}.`
							: `Changed the default ${parameter.label} level ${before} to ${what(rule)}.`
					);
				} else {
					state.alertDefaults.push({ ...rule, id: newId('alr') });
					log(state, user.fullName, `Added the default ${parameter.label} level ${what(rule)}.`);
				}
				return structuredClone(state.alertDefaults);
			});
		},

		async deleteAlertDefault(ruleId) {
			await delay();
			return updateDb((state) => {
				const user = asAdmin(state);
				const stored = state.alertDefaults.find((r) => r.id === ruleId);
				if (!stored) throw new ServiceError('not_found', 'That alert no longer exists.');
				const parameter = state.parameters.find((p) => p.id === stored.parameterId);
				state.alertDefaults = state.alertDefaults.filter((r) => r !== stored);
				log(
					state,
					user.fullName,
					`Removed the default ${parameter?.label ?? stored.parameterId} level ${ruleText(stored, parameter?.unit ?? '')} (${stored.title}).`
				);
				return structuredClone(state.alertDefaults);
			});
		}
	};
}
