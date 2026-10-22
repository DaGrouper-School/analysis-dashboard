import type {
	Address,
	CalculationDetails,
	ClassroomAlerts,
	ClassroomDetail,
	ClassroomSummary,
	Invitation,
	InvitationPreview,
	AuthResult,
	DeviceTypeDefinition,
	DomainDefinition,
	Parameter,
	RegistryChange,
	PhoneChallenge,
	PlaceSuggestion,
	ProfileInput,
	CurrentConditions,
	Device,
	Lab,
	LabDetail,
	Observation,
	ObservationTarget,
	ObservationTemplate,
	Reading,
	User
} from '#lib/domain/types.ts';
import { ServiceError, type ServiceErrorCode, type Services } from '../types';

/**
 * Sketch of the real implementation: same interfaces, backed by the auth API and SAM.
 * Endpoint paths are proposals for the backend team, not an agreed contract yet.
 * The session lives in an HTTP-only cookie, so requests send credentials and the
 * front end never handles tokens.
 */
export function createHttpServices(baseUrl: string): Services {
	async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
		let response: Response;
		try {
			response = await fetch(`${baseUrl}${path}`, {
				method,
				credentials: 'include',
				headers: body === undefined ? undefined : { 'content-type': 'application/json' },
				body: body === undefined ? undefined : JSON.stringify(body)
			});
		} catch {
			throw new ServiceError('network', 'Could not reach the server. Check your connection.');
		}
		if (response.status === 204) return undefined as T;
		const data = await response.json().catch(() => ({}));
		if (!response.ok) {
			const code: ServiceErrorCode =
				data.code ?? (response.status === 401 ? 'unauthenticated' : 'invalid_input');
			throw new ServiceError(code, data.message ?? 'Request failed.', data.fields);
		}
		return data as T;
	}

	return {
		auth: {
			getCurrentUser: () =>
				request<User>('GET', '/auth/me').catch((e) => {
					if (e instanceof ServiceError && e.code === 'unauthenticated') return null;
					throw e;
				}),
			signInWithGoogle: (idToken) => request<AuthResult>('POST', '/auth/google', { idToken }),
			requestPhoneCode: (phone) => request<PhoneChallenge>('POST', '/auth/phone/start', { phone }),
			verifyPhoneCode: (requestId, code) =>
				request<AuthResult>('POST', '/auth/phone/verify', { requestId, code }),
			signOut: () => request<void>('POST', '/auth/sign-out')
		},
		profile: {
			updateProfile: (input: ProfileInput) => request<User>('PATCH', '/me/profile', input),
			startPhoneVerification: (phone) =>
				request<PhoneChallenge>('POST', '/me/phone/start', { phone }),
			confirmPhoneVerification: (requestId, code) =>
				request<User>('POST', '/me/phone/verify', { requestId, code }),
			updateAddress: (address: Address) => request<User>('PUT', '/me/address', address),
			completeOnboarding: () => request<User>('POST', '/me/onboarding/complete')
		},
		classrooms: {
			listMine: () => request<ClassroomSummary[]>('GET', '/classrooms'),
			get: (id) => request<ClassroomDetail>('GET', `/classrooms/${encodeURIComponent(id)}`),
			create: (input) => request<ClassroomSummary>('POST', '/classrooms', input),
			update: (id, input) =>
				request<ClassroomSummary>('PATCH', `/classrooms/${encodeURIComponent(id)}`, input),
			invite: (id, emails, role) =>
				request<Invitation[]>('POST', `/classrooms/${encodeURIComponent(id)}/invitations`, {
					emails,
					role
				}),
			revokeInvitation: (invitationId) =>
				request<void>('DELETE', `/invitations/${encodeURIComponent(invitationId)}`),
			setMemberRole: (id, userId, role) =>
				request<void>(
					'PATCH',
					`/classrooms/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
					{ role }
				),
			removeMember: (id, userId) =>
				request<void>(
					'DELETE',
					`/classrooms/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`
				),
			getInvitation: (token) =>
				request<InvitationPreview>('GET', `/invitations/by-token/${encodeURIComponent(token)}`),
			listMyInvitations: () => request<InvitationPreview[]>('GET', '/me/invitations'),
			acceptInvitation: (token) =>
				request<ClassroomSummary>(
					'POST',
					`/invitations/by-token/${encodeURIComponent(token)}/accept`
				),
			listAlerts: (id) =>
				request<ClassroomAlerts>('GET', `/classrooms/${encodeURIComponent(id)}/alerts`),
			saveAlert: (id, input, ruleId) =>
				ruleId
					? request<ClassroomAlerts>(
							'PUT',
							`/classrooms/${encodeURIComponent(id)}/alerts/${encodeURIComponent(ruleId)}`,
							input
						)
					: request<ClassroomAlerts>('POST', `/classrooms/${encodeURIComponent(id)}/alerts`, input),
			deleteAlert: (id, ruleId) =>
				request<ClassroomAlerts>(
					'DELETE',
					`/classrooms/${encodeURIComponent(id)}/alerts/${encodeURIComponent(ruleId)}`
				),
			resetAlerts: (id, parameterId) =>
				request<ClassroomAlerts>(
					'DELETE',
					`/classrooms/${encodeURIComponent(id)}/alerts?parameter=${encodeURIComponent(parameterId)}`
				)
		},
		labs: {
			listForClassroom: (classroomId) =>
				request<Lab[]>('GET', `/classrooms/${encodeURIComponent(classroomId)}/labs`),
			get: (labId) => request<LabDetail>('GET', `/labs/${encodeURIComponent(labId)}`),
			create: (classroomId, input) =>
				request<Lab>('POST', `/classrooms/${encodeURIComponent(classroomId)}/labs`, input),
			update: (labId, input) => request<Lab>('PUT', `/labs/${encodeURIComponent(labId)}`, input),
			registerDevice: (labId, input) =>
				request<Device>('POST', `/labs/${encodeURIComponent(labId)}/devices`, input),
			removeDevice: (deviceId) =>
				request<void>('DELETE', `/devices/${encodeURIComponent(deviceId)}`),
			// The backend stores the readings, tags them with the classroom and forwards them to SAM.
			uploadReadings: (deviceId, readings, options = { credit: false }) =>
				request<Reading[]>('POST', `/devices/${encodeURIComponent(deviceId)}/readings`, {
					readings,
					credit: options.credit
				}),
			currentConditions: () => request<CurrentConditions[]>('GET', '/me/conditions'),
			recentReadings: (limit = 20) =>
				request<CurrentConditions[]>('GET', `/me/readings?limit=${limit}`),
			listClassroomDevices: (classroomId) =>
				request<Device[]>('GET', `/classrooms/${encodeURIComponent(classroomId)}/devices`),
			deviceReadings: (deviceId, limit = 100) =>
				request<Reading[]>(
					'GET',
					`/devices/${encodeURIComponent(deviceId)}/readings?limit=${limit}`
				),
			observationTargets: () => request<ObservationTarget[]>('GET', '/me/observation-targets'),
			submitObservation: (input, options = { credit: false }) =>
				request<Observation>('POST', `/labs/${encodeURIComponent(input.labId)}/observations`, {
					...input,
					credit: options.credit
				})
		},
		places: {
			autocomplete: (q) =>
				request<PlaceSuggestion[]>('GET', `/places/autocomplete?q=${encodeURIComponent(q)}`),
			getAddress: (placeId) => request<Address>('GET', `/places/${encodeURIComponent(placeId)}`)
		},
		registry: {
			listParameters: () => request<Parameter[]>('GET', '/sam/parameters'),
			listDomains: () => request<DomainDefinition[]>('GET', '/sam/domains'),
			listDeviceTypes: () => request<DeviceTypeDefinition[]>('GET', '/sam/device-types'),
			listObservationTemplates: () =>
				request<ObservationTemplate[]>('GET', '/sam/observation-templates'),
			getCalculation: (id) =>
				request<CalculationDetails>('GET', `/sam/parameters/${encodeURIComponent(id)}/calculation`),
			listChanges: () => request<RegistryChange[]>('GET', '/sam/changes'),
			createDomain: (input) => request<DomainDefinition>('POST', '/sam/domains', input),
			updateDomain: (id, input) =>
				request<DomainDefinition>('PATCH', `/sam/domains/${encodeURIComponent(id)}`, input),
			deleteDomain: (id) => request<void>('DELETE', `/sam/domains/${encodeURIComponent(id)}`),
			createDeviceType: (input) =>
				request<DeviceTypeDefinition>('POST', '/sam/device-types', input),
			updateDeviceType: (id, input) =>
				request<DeviceTypeDefinition>(
					'PATCH',
					`/sam/device-types/${encodeURIComponent(id)}`,
					input
				),
			mapParameter: (parameterId, domain) =>
				request<Parameter>('PATCH', `/sam/parameters/${encodeURIComponent(parameterId)}`, {
					domain
				})
		}
	};
}
