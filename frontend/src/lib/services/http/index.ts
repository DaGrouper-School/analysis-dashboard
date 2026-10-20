import type {
	Address,
	AuthResult,
	Parameter,
	PhoneChallenge,
	PlaceSuggestion,
	ProfileInput,
	Station,
	StationSnapshot,
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
		places: {
			autocomplete: (q) =>
				request<PlaceSuggestion[]>('GET', `/places/autocomplete?q=${encodeURIComponent(q)}`),
			getAddress: (placeId) => request<Address>('GET', `/places/${encodeURIComponent(placeId)}`)
		},
		registry: {
			listParameters: () => request<Parameter[]>('GET', '/sam/parameters'),
			listStations: () => request<Station[]>('GET', '/sam/stations'),
			getLatestSnapshot: (stationId) =>
				request<StationSnapshot | null>(
					'GET',
					`/sam/stations/${encodeURIComponent(stationId)}/latest`
				)
		}
	};
}
