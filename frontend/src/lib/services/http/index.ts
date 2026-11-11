import type {
	DialCode,
	InstitutionInfo,
	HostApplication,
	HostApplicationInput,
	HostApplicationReceipt,
	HostQuestion,
	ConsentRequest,
	ConsentSheet,
	GuardianConsent,
	MemberConsent,
	AlertRule,
	Address,
	CalculationDetails,
	DeviceAlerts,
	ClassroomDetail,
	ClassroomSummary,
	ReleaseReview,
	SharingInput,
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
	CsvExport,
	PublicHome,
	PublicClassroomProfile,
	PublicContributor,
	PublicParameterPage,
	PublicStation,
	SiteContent,
	RawPage,
	Device,
	Lab,
	LabDetail,
	Observation,
	SyncBatchResult,
	ObservationTarget,
	ObservationTemplate,
	Reading,
	User
} from '#lib/domain/types.ts';
import { ServiceError, type ServiceErrorCode, type Services } from '../types';

/**
 * The real implementation: same interfaces, backed by the DaGrouper School backend, which
 * calls SAM. Every call is listed in `routes.ts`, which generates `backend/openapi.yaml`.
 * The paths are a proposal for the backend team (BE-01), not an agreed contract yet.
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
			completeOnboarding: () => request<User>('POST', '/me/onboarding/complete'),
			setPublicProfile: (show) => request<User>('PUT', '/me/public-profile', { show }),
			// The backend sends the guardian their link; it is never returned to the minor.
			requestGuardianConsent: (input) =>
				request<GuardianConsent>('POST', '/me/guardian-consent', input),
			markMinor: () => request<User>('POST', '/me/minor'),
			getGuardianConsent: () => request<GuardianConsent | null>('GET', '/me/guardian-consent')
		},
		classrooms: {
			listMine: () => request<ClassroomSummary[]>('GET', '/classrooms'),
			get: (id) => request<ClassroomDetail>('GET', `/classrooms/${encodeURIComponent(id)}`),
			create: (input) => request<ClassroomSummary>('POST', '/classrooms', input),
			update: (id, input) =>
				request<ClassroomSummary>('PATCH', `/classrooms/${encodeURIComponent(id)}`, input),
			setInstitutionPhoto: (id, input) =>
				request<InstitutionInfo>(
					'PUT',
					`/classrooms/${encodeURIComponent(id)}/institution/photo`,
					input
				),
			updateSharing: (id, input) =>
				request<SharingInput>('PUT', `/classrooms/${encodeURIComponent(id)}/sharing`, input),
			releaseReview: (id) =>
				request<ReleaseReview>('GET', `/classrooms/${encodeURIComponent(id)}/release`),
			releaseData: (id, through) =>
				request<ReleaseReview>('POST', `/classrooms/${encodeURIComponent(id)}/release`, {
					through
				}),
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
			// Paper guardian consent (AP-38). Each decision is asserted to SAM as `consent`.
			listConsent: (id) =>
				request<MemberConsent[]>('GET', `/classrooms/${encodeURIComponent(id)}/consent`),
			recordPaperConsent: (id, userId, input) =>
				request<MemberConsent>(
					'PUT',
					`/classrooms/${encodeURIComponent(id)}/consent/${encodeURIComponent(userId)}`,
					input
				),
			consentSheet: (id) =>
				request<ConsentSheet>('GET', `/classrooms/${encodeURIComponent(id)}/consent-sheet`),
			getInvitation: (token) =>
				request<InvitationPreview>('GET', `/invitations/by-token/${encodeURIComponent(token)}`),
			listMyInvitations: () => request<InvitationPreview[]>('GET', '/me/invitations'),
			acceptInvitation: (token) =>
				request<ClassroomSummary>(
					'POST',
					`/invitations/by-token/${encodeURIComponent(token)}/accept`
				)
		},
		// SAM's export service filters, sorts and applies each classroom's terms on the server.
		exports: {
			rawValues: (query, limit = 200) =>
				request<RawPage>('POST', '/exports/raw', { ...query, limit }),
			rawCsv: (query) => request<CsvExport>('POST', '/exports/raw.csv', query)
		},
		// Open to visitors: the backend serves these without a session, released data only.
		// Public: the guardian's link. Each decision is asserted to SAM as `consent`.
		consent: {
			get: (token) => request<ConsentRequest>('GET', `/consent/${encodeURIComponent(token)}`),
			decide: (token, give) =>
				request<ConsentRequest>('POST', `/consent/${encodeURIComponent(token)}`, { give })
		},

		// Applying is public; deciding is for platform admins.
		hosting: {
			questions: () => request<HostQuestion[]>('GET', '/host/questions'),
			apply: (input: HostApplicationInput) =>
				request<HostApplicationReceipt>('POST', '/host/applications', input),
			status: (token) =>
				request<HostApplication>('GET', `/host/status/${encodeURIComponent(token)}`),
			list: () => request<HostApplication[]>('GET', '/admin/host-applications'),
			decide: (id, accept) =>
				request<HostApplication>(
					'POST',
					`/admin/host-applications/${encodeURIComponent(id)}/decision`,
					{ accept }
				)
		},
		publicSite: {
			home: () => request<PublicHome>('GET', '/public/home'),
			content: () => request<SiteContent>('GET', '/public/content'),
			station: (classroomId) =>
				request<PublicStation>('GET', `/public/classrooms/${encodeURIComponent(classroomId)}`),
			parameter: (classroomId, parameterId) =>
				request<PublicParameterPage>(
					'GET',
					`/public/classrooms/${encodeURIComponent(classroomId)}/parameters/${encodeURIComponent(parameterId)}`
				),
			profile: (classroomId) =>
				request<PublicClassroomProfile>(
					'GET',
					`/public/classrooms/${encodeURIComponent(classroomId)}/profile`
				),
			contributor: (participantId) =>
				request<PublicContributor>('GET', `/public/contributors/${participantId}`)
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
				}),
			// The backend stores what is new, matches retries by `clientId`, and forwards to SAM.
			syncObservations: (batch) => request<SyncBatchResult>('POST', '/observations/batch', batch),
			listDeviceAlerts: (deviceId) =>
				request<DeviceAlerts>('GET', `/devices/${encodeURIComponent(deviceId)}/alerts`),
			// `scope=type` writes the classroom's levels for the device's sensor type.
			saveAlert: (deviceId, scope, input, ruleId) =>
				request<DeviceAlerts>(
					ruleId ? 'PUT' : 'POST',
					`/devices/${encodeURIComponent(deviceId)}/alerts${ruleId ? `/${encodeURIComponent(ruleId)}` : ''}?scope=${scope}`,
					input
				),
			deleteAlert: (deviceId, ruleId) =>
				request<DeviceAlerts>(
					'DELETE',
					`/devices/${encodeURIComponent(deviceId)}/alerts/${encodeURIComponent(ruleId)}`
				),
			resetAlerts: (deviceId, scope, parameterId) =>
				request<DeviceAlerts>(
					'DELETE',
					`/devices/${encodeURIComponent(deviceId)}/alerts?scope=${scope}&parameter=${encodeURIComponent(parameterId)}`
				)
		},
		places: {
			autocomplete: (q) =>
				request<PlaceSuggestion[]>('GET', `/places/autocomplete?q=${encodeURIComponent(q)}`),
			getAddress: (placeId) => request<Address>('GET', `/places/${encodeURIComponent(placeId)}`),
			dialCodes: () => request<DialCode[]>('GET', '/places/dial-codes')
		},
		registry: {
			listParameters: () => request<Parameter[]>('GET', '/registry/parameters'),
			listDomains: () => request<DomainDefinition[]>('GET', '/registry/domains'),
			listDeviceTypes: () => request<DeviceTypeDefinition[]>('GET', '/registry/device-types'),
			listObservationTemplates: () =>
				request<ObservationTemplate[]>('GET', '/registry/observation-templates'),
			getCalculation: (id) =>
				request<CalculationDetails>(
					'GET',
					`/registry/parameters/${encodeURIComponent(id)}/calculation`
				),
			listChanges: () => request<RegistryChange[]>('GET', '/registry/changes'),
			createDomain: (input) => request<DomainDefinition>('POST', '/registry/domains', input),
			updateDomain: (id, input) =>
				request<DomainDefinition>('PATCH', `/registry/domains/${encodeURIComponent(id)}`, input),
			deleteDomain: (id) => request<void>('DELETE', `/registry/domains/${encodeURIComponent(id)}`),
			createDeviceType: (input) =>
				request<DeviceTypeDefinition>('POST', '/registry/device-types', input),
			updateDeviceType: (id, input) =>
				request<DeviceTypeDefinition>(
					'PATCH',
					`/registry/device-types/${encodeURIComponent(id)}`,
					input
				),
			mapParameter: (parameterId, domain) =>
				request<Parameter>('PATCH', `/registry/parameters/${encodeURIComponent(parameterId)}`, {
					domain
				}),
			explainParameter: (parameterId, input) =>
				request<Parameter>(
					'PUT',
					`/registry/parameters/${encodeURIComponent(parameterId)}/explanation`,
					input
				),
			listAlertDefaults: () => request<AlertRule[]>('GET', '/registry/alert-defaults'),
			saveAlertDefault: (input, ruleId) =>
				ruleId
					? request<AlertRule[]>(
							'PUT',
							`/registry/alert-defaults/${encodeURIComponent(ruleId)}`,
							input
						)
					: request<AlertRule[]>('POST', '/registry/alert-defaults', input),
			deleteAlertDefault: (ruleId) =>
				request<AlertRule[]>('DELETE', `/registry/alert-defaults/${encodeURIComponent(ruleId)}`)
		}
	};
}
