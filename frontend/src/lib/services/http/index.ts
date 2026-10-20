import type {
	Address,
	ClassroomDetail,
	ClassroomSummary,
	Invitation,
	InvitationPreview,
	AuthResult,
	Parameter,
	PhoneChallenge,
	PlaceSuggestion,
	ProfileInput,
	CurrentConditions,
	Device,
	Lab,
	LabDetail,
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
			acceptInvitation: (token) =>
				request<ClassroomSummary>(
					'POST',
					`/invitations/by-token/${encodeURIComponent(token)}/accept`
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
			uploadReadings: (deviceId, readings) =>
				request<Reading[]>('POST', `/devices/${encodeURIComponent(deviceId)}/readings`, {
					readings
				}),
			currentConditions: () => request<CurrentConditions[]>('GET', '/me/conditions')
		},
		places: {
			autocomplete: (q) =>
				request<PlaceSuggestion[]>('GET', `/places/autocomplete?q=${encodeURIComponent(q)}`),
			getAddress: (placeId) => request<Address>('GET', `/places/${encodeURIComponent(placeId)}`)
		},
		registry: {
			listParameters: () => request<Parameter[]>('GET', '/sam/parameters')
		}
	};
}
