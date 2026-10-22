import type {
	Address,
	AlertRule,
	AlertRuleInput,
	ExplanationInput,
	AlertScope,
	DeviceAlerts,
	CalculationDetails,
	ClassroomDetail,
	ClassroomRole,
	ClassroomSummary,
	CreateClassroomInput,
	Invitation,
	InvitationPreview,
	AuthResult,
	DeviceTypeDefinition,
	DeviceTypeInput,
	DomainDefinition,
	DomainInput,
	Parameter,
	RegistryChange,
	PhoneChallenge,
	PlaceSuggestion,
	ProfileInput,
	CreateLabInput,
	CsvExport,
	RawPage,
	RawQuery,
	CurrentConditions,
	Device,
	Lab,
	LabDetail,
	Observation,
	ObservationInput,
	ObservationTarget,
	ObservationTemplate,
	Reading,
	ReadingInput,
	RecordOptions,
	RegisterDeviceInput,
	UpdateClassroomInput,
	User
} from '#lib/domain/types.ts';

/**
 * The contract between the UI and its data source. Pages only talk to these interfaces
 * through `services` in `#lib/services`, so swapping the mock layer for the real auth API
 * and SAM means providing another implementation, not changing pages.
 *
 * All methods reject with a `ServiceError` on failure.
 */

export interface AuthService {
	/** The signed-in user, or null. Real backend: reads the HTTP-only session cookie. */
	getCurrentUser(): Promise<User | null>;
	/** Exchange a Google ID token for a session. The backend verifies the token. */
	signInWithGoogle(idToken: string): Promise<AuthResult>;
	/** Validate the number and send a 6-digit SMS code. */
	requestPhoneCode(phone: string): Promise<PhoneChallenge>;
	/** Verify the SMS code and start a session. */
	verifyPhoneCode(requestId: string, code: string): Promise<AuthResult>;
	signOut(): Promise<void>;
}

export interface ProfileService {
	updateProfile(input: ProfileInput): Promise<User>;
	/** Google sign-ups verify a phone number during onboarding. */
	startPhoneVerification(phone: string): Promise<PhoneChallenge>;
	confirmPhoneVerification(requestId: string, code: string): Promise<User>;
	updateAddress(address: Address): Promise<User>;
	/** Checks required fields and sets `onboardingCompleted`. */
	completeOnboarding(): Promise<User>;
}

export interface PlacesService {
	autocomplete(query: string): Promise<PlaceSuggestion[]>;
	getAddress(placeId: string): Promise<Address>;
}

/**
 * The shared registry (SAM-05, SAM-23): parameters, environmental domains and device types.
 * Everyone signed in can read it; only platform admins change it, and every change is logged.
 */
export interface RegistryService {
	listParameters(): Promise<Parameter[]>;
	listDomains(): Promise<DomainDefinition[]>;
	listDeviceTypes(): Promise<DeviceTypeDefinition[]>;
	/**
	 * Guided forms for manual observations (AP-16), plus a plain one for each device type that
	 * has none.
	 */
	listObservationTemplates(): Promise<ObservationTemplate[]>;
	/** "How is this calculated?" (AP-07): formula, inputs and SAM version of one value. */
	getCalculation(parameterId: string): Promise<CalculationDetails>;
	/** Platform admins only. */
	listChanges(): Promise<RegistryChange[]>;
	createDomain(input: DomainInput): Promise<DomainDefinition>;
	/** The code never changes. */
	updateDomain(id: string, input: Omit<DomainInput, 'id'>): Promise<DomainDefinition>;
	/** Refused while any parameter is mapped to the domain. */
	deleteDomain(id: string): Promise<void>;
	createDeviceType(input: DeviceTypeInput): Promise<DeviceTypeDefinition>;
	/**
	 * Renames a type or changes its values. New values join the registry; the label, unit and
	 * type of values added by device types can be edited. Station values stay as registered.
	 */
	updateDeviceType(id: string, input: DeviceTypeInput): Promise<DeviceTypeDefinition>;
	/** Puts a parameter under a domain, or takes it out with null. */
	mapParameter(parameterId: string, domain: string | null): Promise<Parameter>;
	/** Sets what a value means and how it is calculated (AP-07). */
	explainParameter(parameterId: string, input: ExplanationInput): Promise<Parameter>;
	/**
	 * The platform default alert levels (AP-08): in force on every device whose classroom and
	 * device have no levels of their own for that value.
	 */
	listAlertDefaults(): Promise<AlertRule[]>;
	/** Adds a default level, or changes one. Turning one off is a change with `enabled` false. */
	saveAlertDefault(input: AlertRuleInput, ruleId?: string): Promise<AlertRule[]>;
	deleteAlertDefault(ruleId: string): Promise<AlertRule[]>;
}

/**
 * Classrooms (AP-03). Permission checks and code redaction happen behind this interface,
 * never only in the UI.
 */
export interface ClassroomService {
	listMine(): Promise<ClassroomSummary[]>;
	get(classroomId: string): Promise<ClassroomDetail>;
	/** Institution heads create their own; platform admins can set one up for a head. */
	create(input: CreateClassroomInput): Promise<ClassroomSummary>;
	/** Name and institution. The classroom code is its identifier and cannot change. */
	update(classroomId: string, input: UpdateClassroomInput): Promise<ClassroomSummary>;
	/** Sends one invitation email per address. */
	invite(classroomId: string, emails: string[], role: ClassroomRole): Promise<Invitation[]>;
	revokeInvitation(invitationId: string): Promise<void>;
	setMemberRole(classroomId: string, userId: string, role: ClassroomRole): Promise<void>;
	removeMember(classroomId: string, userId: string): Promise<void>;
	/** Public: what an invite link points to. */
	getInvitation(token: string): Promise<InvitationPreview>;
	/** Pending invitations sent to the signed-in user's email, newest first. */
	listMyInvitations(): Promise<InvitationPreview[]>;
	/** Joins the signed-in user to the classroom. Their email must match the invitation. */
	acceptInvitation(token: string): Promise<ClassroomSummary>;
}

/** Labs and their devices (AP-05). Each device is registered in SAM as a source. */
export interface LabService {
	listForClassroom(classroomId: string): Promise<Lab[]>;
	get(labId: string): Promise<LabDetail>;
	/** Classroom onboarding step: the lab and its site profile questionnaire. */
	create(classroomId: string, input: CreateLabInput): Promise<Lab>;
	update(labId: string, input: CreateLabInput): Promise<Lab>;
	/** Registers the device and its SAM source. No location: that comes with each reading. */
	registerDevice(labId: string, input: RegisterDeviceInput): Promise<Device>;
	removeDevice(deviceId: string): Promise<void>;
	/**
	 * A permitted user connects a device and its readings are sent to the backend and on to
	 * SAM, tagged with the classroom. Readings without GPS get the lab's location, marked
	 * `locationSource: 'lab'`.
	 */
	uploadReadings(
		deviceId: string,
		readings: ReadingInput[],
		options?: RecordOptions
	): Promise<Reading[]>;
	/**
	 * Latest reading per device, only for classrooms the user is a member of. Nobody sees
	 * conditions from other classrooms' devices, platform admins included.
	 */
	currentConditions(): Promise<CurrentConditions[]>;
	/** Recent readings from devices in the user's classrooms, newest first. Same membership rule. */
	recentReadings(limit?: number): Promise<CurrentConditions[]>;
	/** Every device registered to a classroom's labs, for anyone who can view the classroom. */
	listClassroomDevices(classroomId: string): Promise<Device[]>;
	/** One device's readings, newest first, for anyone who can view its classroom. */
	deviceReadings(deviceId: string, limit?: number): Promise<Reading[]>;
	/** Labs the user may record observations in, with their registered devices. */
	observationTargets(): Promise<ObservationTarget[]>;
	/**
	 * A manual observation from the guided form (AP-16). The backend runs the template's checks
	 * again, tags it with the classroom and sends it to SAM: under the device for a device
	 * reading, under the classroom for a field observation. No GPS: the lab's location, flagged.
	 */
	submitObservation(input: ObservationInput, options?: RecordOptions): Promise<Observation>;
	/**
	 * The alert levels in force for a device (AP-08), each with its layer: the device's own, its
	 * classroom's for its sensor type, or the platform default. Anyone who can view the device.
	 */
	listDeviceAlerts(deviceId: string): Promise<DeviceAlerts>;
	/**
	 * Adds a rule at `scope` for the device's parameter, or changes one when `ruleId` is given.
	 * Heads and facilitators set the device's own; only the head sets the sensor type's for the
	 * classroom. The first change at a layer copies the levels it replaces, so the others stay.
	 */
	saveAlert(
		deviceId: string,
		scope: AlertScope,
		input: AlertRuleInput,
		ruleId?: string
	): Promise<DeviceAlerts>;
	/** Removes one rule from its layer. With none left there, the next layer applies. */
	deleteAlert(deviceId: string, ruleId: string): Promise<DeviceAlerts>;
	/** Drops a layer's rules for a parameter, back to the next layer down. */
	resetAlerts(deviceId: string, scope: AlertScope, parameterId: string): Promise<DeviceAlerts>;
}

/**
 * SAM's export service (AP-14): raw values of one classroom, filtered and sorted on the server,
 * and the same as a CSV file. Only classrooms the user belongs to; platform admins included.
 */
export interface ExportService {
	/** Up to `limit` matching values, with the classroom's data terms. */
	rawValues(query: RawQuery, limit?: number): Promise<RawPage>;
	/** Every matching value as CSV. Each row carries the licence, attribution and restrictions. */
	rawCsv(query: RawQuery): Promise<CsvExport>;
}

export interface Services {
	auth: AuthService;
	profile: ProfileService;
	classrooms: ClassroomService;
	labs: LabService;
	exports: ExportService;
	places: PlacesService;
	registry: RegistryService;
}

export type ServiceErrorCode =
	| 'unauthenticated'
	| 'invalid_input'
	| 'not_found'
	| 'rate_limited'
	| 'code_invalid'
	| 'code_expired'
	| 'conflict'
	| 'forbidden'
	| 'network';

export class ServiceError extends Error {
	constructor(
		readonly code: ServiceErrorCode,
		message: string,
		/** Field-level messages, keyed by input name. */
		readonly fields: Record<string, string> = {}
	) {
		super(message);
		this.name = 'ServiceError';
	}
}

export function errorMessage(e: unknown): string {
	if (e instanceof ServiceError) return e.message;
	return 'Something went wrong. Please try again.';
}
