import type { Services } from '../types';

/**
 * The backend API contract: one entry per HTTP call that `createHttpServices` makes.
 * `scripts/openapi.ts` turns this table, the JSDoc in `services/types.ts` and the types in
 * `domain/types.ts` into `backend/openapi.yaml`. `routes.spec.ts` checks that the HTTP client
 * calls exactly these routes.
 *
 * Type expressions (`body`, `response`) are TypeScript over the names exported from
 * `domain/types.ts`. A `response` of null means 204 No Content.
 */

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

/** `public`: no session needed. `user`: any signed-in user. `admin`: platform admins only. */
export type Access = 'public' | 'user' | 'admin';

type ServiceMethod = {
	[S in keyof Services]: `${S & string}.${keyof Services[S] & string}`;
}[keyof Services];

export interface QueryParam {
	type: 'string' | 'integer';
	required?: boolean;
	/** A TypeScript type to use instead of `type`, such as a union of literals. */
	ts?: string;
	description?: string;
}

export interface Route {
	/** The service method this route implements. */
	method: ServiceMethod;
	/** Set when one service method maps to several routes. */
	operationId?: string;
	http: HttpMethod;
	/** Path with `{param}` placeholders. */
	path: string;
	access: Access;
	body?: string;
	query?: Record<string, QueryParam>;
	response: string | null;
	/** Statuses beyond the ones every route can return. */
	errors?: (400 | 404 | 409 | 429)[];
	/** Extra notes for this route, after the method's own JSDoc. */
	notes?: string;
	/** Arguments the contract test calls the method with. Default: `x1`, `x2`, ... */
	args?: unknown[];
}

const scope: QueryParam = {
	type: 'string',
	required: true,
	ts: 'AlertScope',
	description: 'Which layer to change.'
};

export const routes: Route[] = [
	// Auth
	{
		method: 'auth.getCurrentUser',
		http: 'GET',
		path: '/auth/me',
		access: 'public',
		response: 'User',
		notes: 'Returns 401 when there is no session. The front end treats that as signed out.'
	},
	{
		method: 'auth.signInWithGoogle',
		http: 'POST',
		path: '/auth/google',
		access: 'public',
		body: '{ idToken: string }',
		response: 'AuthResult',
		errors: [400],
		notes: 'Sets the HTTP-only session cookie.'
	},
	{
		method: 'auth.requestPhoneCode',
		http: 'POST',
		path: '/auth/phone/start',
		access: 'public',
		body: '{ phone: string }',
		response: 'PhoneChallenge',
		errors: [400, 429]
	},
	{
		method: 'auth.verifyPhoneCode',
		http: 'POST',
		path: '/auth/phone/verify',
		access: 'public',
		body: '{ requestId: string; code: string }',
		response: 'AuthResult',
		errors: [400, 429],
		notes:
			'Sets the HTTP-only session cookie. Wrong or old codes return `code_invalid` or `code_expired`.'
	},
	{
		method: 'auth.signOut',
		http: 'POST',
		path: '/auth/sign-out',
		access: 'user',
		response: null,
		notes: 'Clears the session cookie.'
	},

	// Profile
	{
		method: 'profile.updateProfile',
		http: 'PATCH',
		path: '/me/profile',
		access: 'user',
		body: 'ProfileInput',
		response: 'User',
		errors: [400, 409]
	},
	{
		method: 'profile.startPhoneVerification',
		http: 'POST',
		path: '/me/phone/start',
		access: 'user',
		body: '{ phone: string }',
		response: 'PhoneChallenge',
		errors: [400, 409, 429]
	},
	{
		method: 'profile.confirmPhoneVerification',
		http: 'POST',
		path: '/me/phone/verify',
		access: 'user',
		body: '{ requestId: string; code: string }',
		response: 'User',
		errors: [400, 429]
	},
	{
		method: 'profile.updateAddress',
		http: 'PUT',
		path: '/me/address',
		access: 'user',
		body: 'Address',
		response: 'User',
		errors: [400]
	},
	{
		method: 'profile.completeOnboarding',
		http: 'POST',
		path: '/me/onboarding/complete',
		access: 'user',
		response: 'User',
		errors: [400]
	},
	{
		method: 'profile.requestGuardianConsent',
		http: 'POST',
		path: '/me/guardian-consent',
		access: 'user',
		body: 'GuardianInput',
		response: 'GuardianConsent',
		errors: [400],
		notes: 'The backend emails the guardian their link. The link is never returned to the minor.'
	},
	{
		method: 'profile.markMinor',
		http: 'POST',
		path: '/me/minor',
		access: 'user',
		response: 'User'
	},
	{
		method: 'profile.getGuardianConsent',
		http: 'GET',
		path: '/me/guardian-consent',
		access: 'user',
		response: 'GuardianConsent | null'
	},

	// Classrooms
	{
		method: 'classrooms.listMine',
		http: 'GET',
		path: '/classrooms',
		access: 'user',
		response: 'ClassroomSummary[]'
	},
	{
		method: 'classrooms.get',
		http: 'GET',
		path: '/classrooms/{classroomId}',
		access: 'user',
		response: 'ClassroomDetail'
	},
	{
		method: 'classrooms.create',
		http: 'POST',
		path: '/classrooms',
		access: 'user',
		body: 'CreateClassroomInput',
		response: 'ClassroomSummary',
		errors: [400, 409]
	},
	{
		method: 'classrooms.update',
		http: 'PATCH',
		path: '/classrooms/{classroomId}',
		access: 'user',
		body: 'UpdateClassroomInput',
		response: 'ClassroomSummary',
		errors: [400]
	},
	{
		method: 'classrooms.invite',
		http: 'POST',
		path: '/classrooms/{classroomId}/invitations',
		access: 'user',
		body: '{ emails: string[]; role: ClassroomRole }',
		response: 'Invitation[]',
		errors: [400, 409],
		args: ['x1', ['a@example.org'], 'student']
	},
	{
		method: 'classrooms.revokeInvitation',
		http: 'DELETE',
		path: '/invitations/{invitationId}',
		access: 'user',
		response: null
	},
	{
		method: 'classrooms.setMemberRole',
		http: 'PATCH',
		path: '/classrooms/{classroomId}/members/{userId}',
		access: 'user',
		body: '{ role: ClassroomRole }',
		response: null,
		errors: [400]
	},
	{
		method: 'classrooms.removeMember',
		http: 'DELETE',
		path: '/classrooms/{classroomId}/members/{userId}',
		access: 'user',
		response: null
	},
	{
		method: 'classrooms.listConsent',
		http: 'GET',
		path: '/classrooms/{classroomId}/consent',
		access: 'user',
		response: 'MemberConsent[]',
		errors: [404]
	},
	{
		method: 'classrooms.recordPaperConsent',
		http: 'PUT',
		path: '/classrooms/{classroomId}/consent/{userId}',
		access: 'user',
		body: 'PaperConsentInput',
		response: 'MemberConsent',
		errors: [400, 404],
		notes: 'The recorder is the signed-in user. The decision is asserted to SAM as `consent`.'
	},
	{
		method: 'classrooms.consentSheet',
		http: 'GET',
		path: '/classrooms/{classroomId}/consent-sheet',
		access: 'user',
		response: 'ConsentSheet',
		errors: [404]
	},
	{
		method: 'classrooms.getInvitation',
		http: 'GET',
		path: '/invitations/by-token/{token}',
		access: 'public',
		response: 'InvitationPreview'
	},
	{
		method: 'classrooms.listMyInvitations',
		http: 'GET',
		path: '/me/invitations',
		access: 'user',
		response: 'InvitationPreview[]'
	},
	{
		method: 'classrooms.acceptInvitation',
		http: 'POST',
		path: '/invitations/by-token/{token}/accept',
		access: 'user',
		response: 'ClassroomSummary',
		errors: [409]
	},

	// Labs, devices, readings and observations
	{
		method: 'labs.listForClassroom',
		http: 'GET',
		path: '/classrooms/{classroomId}/labs',
		access: 'user',
		response: 'Lab[]'
	},
	{
		method: 'labs.get',
		http: 'GET',
		path: '/labs/{labId}',
		access: 'user',
		response: 'LabDetail'
	},
	{
		method: 'labs.create',
		http: 'POST',
		path: '/classrooms/{classroomId}/labs',
		access: 'user',
		body: 'CreateLabInput',
		response: 'Lab',
		errors: [400]
	},
	{
		method: 'labs.update',
		http: 'PUT',
		path: '/labs/{labId}',
		access: 'user',
		body: 'CreateLabInput',
		response: 'Lab',
		errors: [400]
	},
	{
		method: 'labs.registerDevice',
		http: 'POST',
		path: '/labs/{labId}/devices',
		access: 'user',
		body: 'RegisterDeviceInput',
		response: 'Device',
		errors: [400, 409]
	},
	{
		method: 'labs.removeDevice',
		http: 'DELETE',
		path: '/devices/{deviceId}',
		access: 'user',
		response: null
	},
	{
		method: 'labs.uploadReadings',
		http: 'POST',
		path: '/devices/{deviceId}/readings',
		access: 'user',
		body: '{ readings: ReadingInput[]; credit: boolean }',
		response: 'Reading[]',
		errors: [400],
		notes:
			'`credit` names the uploader on public pages; minors are never named. The backend stores the readings, tags them with the classroom and forwards them to SAM.',
		args: ['x1', []]
	},
	{
		method: 'labs.currentConditions',
		http: 'GET',
		path: '/me/conditions',
		access: 'user',
		response: 'CurrentConditions[]'
	},
	{
		method: 'labs.recentReadings',
		http: 'GET',
		path: '/me/readings',
		access: 'user',
		query: { limit: { type: 'integer', description: 'Default 20.' } },
		response: 'CurrentConditions[]',
		args: [20]
	},
	{
		method: 'labs.listClassroomDevices',
		http: 'GET',
		path: '/classrooms/{classroomId}/devices',
		access: 'user',
		response: 'Device[]'
	},
	{
		method: 'labs.deviceReadings',
		http: 'GET',
		path: '/devices/{deviceId}/readings',
		access: 'user',
		query: { limit: { type: 'integer', description: 'Default 100.' } },
		response: 'Reading[]',
		args: ['x1', 100]
	},
	{
		method: 'labs.observationTargets',
		http: 'GET',
		path: '/me/observation-targets',
		access: 'user',
		response: 'ObservationTarget[]'
	},
	{
		method: 'labs.submitObservation',
		http: 'POST',
		path: '/labs/{labId}/observations',
		access: 'user',
		body: 'ObservationInput & { credit: boolean }',
		response: 'Observation',
		errors: [400],
		args: [{ labId: 'x1' }]
	},
	{
		method: 'labs.syncObservations',
		http: 'POST',
		path: '/observations/batch',
		access: 'user',
		body: 'SyncBatch',
		response: 'SyncBatchResult',
		errors: [400],
		notes:
			'Returns 200 with a result per record, even when some records are refused. Retries match on `clientId`.',
		args: [{ records: [] }]
	},
	{
		method: 'labs.listDeviceAlerts',
		http: 'GET',
		path: '/devices/{deviceId}/alerts',
		access: 'user',
		response: 'DeviceAlerts'
	},
	{
		method: 'labs.saveAlert',
		operationId: 'labs.createAlert',
		http: 'POST',
		path: '/devices/{deviceId}/alerts',
		access: 'user',
		query: { scope },
		body: 'AlertRuleInput',
		response: 'DeviceAlerts',
		errors: [400],
		args: ['x1', 'device', {}]
	},
	{
		method: 'labs.saveAlert',
		operationId: 'labs.updateAlert',
		http: 'PUT',
		path: '/devices/{deviceId}/alerts/{ruleId}',
		access: 'user',
		query: { scope },
		body: 'AlertRuleInput',
		response: 'DeviceAlerts',
		errors: [400],
		args: ['x1', 'device', {}, 'x2']
	},
	{
		method: 'labs.deleteAlert',
		http: 'DELETE',
		path: '/devices/{deviceId}/alerts/{ruleId}',
		access: 'user',
		response: 'DeviceAlerts'
	},
	{
		method: 'labs.resetAlerts',
		http: 'DELETE',
		path: '/devices/{deviceId}/alerts',
		access: 'user',
		query: {
			scope,
			parameter: { type: 'string', required: true, description: 'The parameter to reset.' }
		},
		response: 'DeviceAlerts',
		args: ['x1', 'device', 'x2']
	},

	// Exports
	{
		method: 'exports.rawValues',
		http: 'POST',
		path: '/exports/raw',
		access: 'user',
		body: 'RawQuery & { limit: number }',
		response: 'RawPage',
		errors: [400],
		args: [{ classroomId: 'x1' }, 200]
	},
	{
		method: 'exports.rawCsv',
		http: 'POST',
		path: '/exports/raw.csv',
		access: 'user',
		body: 'RawQuery',
		response: 'CsvExport',
		errors: [400],
		args: [{ classroomId: 'x1' }]
	},

	// Public site
	{
		method: 'publicSite.home',
		http: 'GET',
		path: '/public/home',
		access: 'public',
		response: 'PublicHome'
	},
	{
		method: 'publicSite.content',
		http: 'GET',
		path: '/public/content',
		access: 'public',
		response: 'SiteContent'
	},
	{
		method: 'publicSite.station',
		http: 'GET',
		path: '/public/classrooms/{classroomId}',
		access: 'public',
		response: 'PublicStation'
	},
	{
		method: 'publicSite.parameter',
		http: 'GET',
		path: '/public/classrooms/{classroomId}/parameters/{parameterId}',
		access: 'public',
		response: 'PublicParameterPage'
	},
	{
		method: 'publicSite.profile',
		http: 'GET',
		path: '/public/classrooms/{classroomId}/profile',
		access: 'public',
		response: 'PublicClassroomProfile',
		notes: 'Kit serial numbers are included only for members of the classroom.'
	},

	// Guardian consent
	{
		method: 'consent.get',
		http: 'GET',
		path: '/consent/{token}',
		access: 'public',
		response: 'ConsentRequest'
	},
	{
		method: 'consent.decide',
		http: 'POST',
		path: '/consent/{token}',
		access: 'public',
		body: '{ give: boolean }',
		response: 'ConsentRequest',
		args: ['x1', true]
	},

	// Hosting
	{
		method: 'hosting.questions',
		http: 'GET',
		path: '/host/questions',
		access: 'public',
		response: 'HostQuestion[]'
	},
	{
		method: 'hosting.apply',
		http: 'POST',
		path: '/host/applications',
		access: 'public',
		body: 'HostApplicationInput',
		response: 'HostApplicationReceipt',
		errors: [400, 429],
		args: [{}]
	},
	{
		method: 'hosting.status',
		http: 'GET',
		path: '/host/status/{token}',
		access: 'public',
		response: 'HostApplication'
	},
	{
		method: 'hosting.list',
		http: 'GET',
		path: '/admin/host-applications',
		access: 'admin',
		response: 'HostApplication[]'
	},
	{
		method: 'hosting.decide',
		http: 'POST',
		path: '/admin/host-applications/{applicationId}/decision',
		access: 'admin',
		body: '{ accept: boolean }',
		response: 'HostApplication',
		errors: [409],
		args: ['x1', true]
	},

	// Places
	{
		method: 'places.autocomplete',
		http: 'GET',
		path: '/places/autocomplete',
		access: 'user',
		query: { q: { type: 'string', required: true, description: 'What the user typed.' } },
		response: 'PlaceSuggestion[]',
		errors: [429],
		notes: 'Proxies the places provider; the API key stays on the server.'
	},
	{
		method: 'places.getAddress',
		http: 'GET',
		path: '/places/{placeId}',
		access: 'user',
		response: 'Address',
		errors: [429]
	},

	// Registry
	{
		method: 'registry.listParameters',
		http: 'GET',
		path: '/registry/parameters',
		access: 'user',
		response: 'Parameter[]'
	},
	{
		method: 'registry.listDomains',
		http: 'GET',
		path: '/registry/domains',
		access: 'user',
		response: 'DomainDefinition[]'
	},
	{
		method: 'registry.listDeviceTypes',
		http: 'GET',
		path: '/registry/device-types',
		access: 'user',
		response: 'DeviceTypeDefinition[]'
	},
	{
		method: 'registry.listObservationTemplates',
		http: 'GET',
		path: '/registry/observation-templates',
		access: 'user',
		response: 'ObservationTemplate[]'
	},
	{
		method: 'registry.getCalculation',
		http: 'GET',
		path: '/registry/parameters/{parameterId}/calculation',
		access: 'user',
		response: 'CalculationDetails'
	},
	{
		method: 'registry.listChanges',
		http: 'GET',
		path: '/registry/changes',
		access: 'admin',
		response: 'RegistryChange[]'
	},
	{
		method: 'registry.createDomain',
		http: 'POST',
		path: '/registry/domains',
		access: 'admin',
		body: 'DomainInput',
		response: 'DomainDefinition',
		errors: [400, 409]
	},
	{
		method: 'registry.updateDomain',
		http: 'PATCH',
		path: '/registry/domains/{domainId}',
		access: 'admin',
		body: "Omit<DomainInput, 'id'>",
		response: 'DomainDefinition',
		errors: [400, 409]
	},
	{
		method: 'registry.deleteDomain',
		http: 'DELETE',
		path: '/registry/domains/{domainId}',
		access: 'admin',
		response: null,
		errors: [409]
	},
	{
		method: 'registry.createDeviceType',
		http: 'POST',
		path: '/registry/device-types',
		access: 'admin',
		body: 'DeviceTypeInput',
		response: 'DeviceTypeDefinition',
		errors: [400, 409]
	},
	{
		method: 'registry.updateDeviceType',
		http: 'PATCH',
		path: '/registry/device-types/{deviceTypeId}',
		access: 'admin',
		body: 'DeviceTypeInput',
		response: 'DeviceTypeDefinition',
		errors: [400, 409]
	},
	{
		method: 'registry.mapParameter',
		http: 'PATCH',
		path: '/registry/parameters/{parameterId}',
		access: 'admin',
		body: '{ domain: string | null }',
		response: 'Parameter',
		errors: [400],
		args: ['x1', null]
	},
	{
		method: 'registry.explainParameter',
		http: 'PUT',
		path: '/registry/parameters/{parameterId}/explanation',
		access: 'admin',
		body: 'ExplanationInput',
		response: 'Parameter',
		errors: [400],
		args: ['x1', {}]
	},
	{
		method: 'registry.listAlertDefaults',
		http: 'GET',
		path: '/registry/alert-defaults',
		access: 'user',
		response: 'AlertRule[]'
	},
	{
		method: 'registry.saveAlertDefault',
		operationId: 'registry.createAlertDefault',
		http: 'POST',
		path: '/registry/alert-defaults',
		access: 'admin',
		body: 'AlertRuleInput',
		response: 'AlertRule[]',
		errors: [400],
		args: [{}]
	},
	{
		method: 'registry.saveAlertDefault',
		operationId: 'registry.updateAlertDefault',
		http: 'PUT',
		path: '/registry/alert-defaults/{ruleId}',
		access: 'admin',
		body: 'AlertRuleInput',
		response: 'AlertRule[]',
		errors: [400],
		args: [{}, 'x1']
	},
	{
		method: 'registry.deleteAlertDefault',
		http: 'DELETE',
		path: '/registry/alert-defaults/{ruleId}',
		access: 'admin',
		response: 'AlertRule[]'
	}
];
