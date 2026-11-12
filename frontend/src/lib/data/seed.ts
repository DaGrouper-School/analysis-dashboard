import type {
	AlertRule,
	AltUnit,
	Calculation,
	DeviceTypeDefinition,
	DomainDefinition,
	EnvironmentalDomain,
	GeoPoint,
	ObservationTemplate,
	Parameter,
	Reading,
	User
} from '#lib/domain/types.ts';
import type { MockDbState } from './mock-db';
import { DEFAULT_SHARING } from '#lib/domain/permissions.ts';

/**
 * Prefilled demo data. Personas come from the vault (User Personas); parameter values come
 * from the sample weather-station payload (User Flow / Sensor Data).
 */

const SEEDED_AT = '2026-10-01T09:00:00.000Z';

const users: User[] = [
	{
		id: 'usr_sarah',
		participantId: 100001,
		publicProfile: false,
		authProvider: 'google',
		fullName: 'Sarah K.',
		email: 'sarah.k@example.org',
		phone: '+12423570100',
		phoneVerified: true,
		avatarUrl: null,
		role: 'custodian',
		platformAdmin: false,
		address: {
			street: '14 Bay Street',
			city: 'Nassau',
			state: 'New Providence',
			zip: '',
			country: 'Bahamas'
		},
		onboardingCompleted: true,
		minor: false,
		consent: 'not_needed',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_abebe',
		participantId: 100002,
		publicProfile: true,
		authProvider: 'phone',
		fullName: 'Abebe T.',
		email: 'abebe.t@example.org',
		phone: '+251911000001',
		phoneVerified: true,
		avatarUrl: null,
		role: 'community_scientist',
		platformAdmin: false,
		address: {
			street: 'Kebele 04',
			city: 'Shashamane',
			state: 'Oromia',
			zip: '',
			country: 'Ethiopia'
		},
		onboardingCompleted: true,
		minor: false,
		consent: 'not_needed',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_marcus',
		participantId: 100003,
		publicProfile: false,
		authProvider: 'google',
		fullName: 'Dr. Marcus L.',
		email: 'marcus.l@planetwerx.example',
		phone: '+14155550123',
		phoneVerified: true,
		avatarUrl: null,
		role: 'researcher',
		platformAdmin: false,
		address: {
			street: '200 Research Way',
			city: 'San Francisco',
			state: 'CA',
			zip: '94107',
			country: 'United States'
		},
		onboardingCompleted: true,
		minor: false,
		consent: 'not_needed',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		// Platform staff only: not a member of any classroom.
		id: 'usr_leila',
		participantId: 100004,
		publicProfile: false,
		authProvider: 'google',
		fullName: 'Leila M.',
		email: 'leila.m@planetwerx.example',
		phone: '+14155550188',
		phoneVerified: true,
		avatarUrl: null,
		role: 'community_scientist',
		platformAdmin: true,
		address: {
			street: '200 Research Way',
			city: 'San Francisco',
			state: 'CA',
			zip: '94107',
			country: 'United States'
		},
		onboardingCompleted: true,
		minor: false,
		consent: 'not_needed',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_tesfaye',
		participantId: 100005,
		publicProfile: false,
		authProvider: 'google',
		fullName: 'Tesfaye G.',
		email: 'tesfaye.g@trinity-school.example',
		phone: '+251911000002',
		phoneVerified: true,
		avatarUrl: null,
		role: 'institution_head',
		platformAdmin: false,
		address: {
			street: 'Churchill Avenue',
			city: 'Addis Ababa',
			state: 'Addis Ababa',
			zip: '',
			country: 'Ethiopia'
		},
		onboardingCompleted: true,
		minor: false,
		consent: 'not_needed',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		// A student under 18: their guardian has not answered the consent link yet.
		id: 'usr_liya',
		participantId: 100006,
		publicProfile: false,
		authProvider: 'phone',
		fullName: 'Liya A.',
		email: 'liya.a@example.org',
		phone: '+251911000003',
		phoneVerified: true,
		avatarUrl: null,
		role: 'community_scientist',
		platformAdmin: false,
		address: {
			street: 'Kebele 04',
			city: 'Shashamane',
			state: 'Oromia',
			zip: '',
			country: 'Ethiopia'
		},
		onboardingCompleted: true,
		minor: true,
		consent: 'pending',
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	}
];

/**
 * The public site's words (AP-20). Draft copy for the content team to replace; pages read it
 * from here, never from their own markup.
 */
const siteContent: MockDbState['siteContent'] = {
	hero: {
		eyebrow: 'Da Grouper School',
		title: 'Communities that measure the world around them',
		body: 'Global Classrooms run a Lab in a Bag: weather, rain, soil and reef measurements taken by students and community scientists, checked, and shared openly with researchers.'
	},
	programme: 'Da Grouper School · Global Classrooms programme',
	sampleData: true,
	hosting: {
		title: 'Host a Global Classroom',
		intro:
			'Tell us about your institution, how ready you are and the place you would study. We review every application before a classroom is set up.',
		submitted: 'Your application is in. Keep this page’s link to check its status.'
	},
	joinClassroom: {
		title: 'Join this classroom',
		body: 'People join a classroom by invitation from its facilitators. Sign in with the email your invitation was sent to.',
		action: 'Sign in to join'
	},
	consentSheet: {
		title: 'Guardian consent for a young contributor',
		intro:
			'{institution} runs {classroom}, a Global Classroom of the Da Grouper School programme. Students take weather, rain, soil and reef measurements and share them openly with researchers. Anyone under 18 needs a parent or guardian’s consent before they send any measurement.',
		sections: [
			{
				heading: 'What your child will do',
				body: 'Take measurements with the classroom’s kit, with a facilitator present, and send them through the DaGrouper app.'
			},
			{
				heading: 'What we keep',
				body: 'The measurements and when they were taken. Your child’s name is never shown with their measurements. A position from their phone is rounded to {decimals} decimal places before it is stored.'
			},
			{
				heading: 'Changing your mind',
				body: 'You can withdraw consent at any time. Tell a facilitator at {institution} or fill in a new sheet. From then on, your child cannot send measurements.'
			}
		],
		choices: {
			give: 'I give consent for my child to take part and send measurements.',
			withdraw: 'I do not give consent, or I withdraw consent I gave before.'
		},
		contributorField: 'Young contributor’s full name',
		fields: [
			'Parent or guardian’s full name',
			'Relationship to the contributor',
			'Phone or email (optional)',
			'Signature',
			'Date'
		],
		officeNote:
			'For the classroom: keep this sheet. The institution head or a facilitator records it in the app under the classroom’s Consent tab.'
	},
	sections: [
		{
			id: 'classrooms',
			label: 'Classrooms',
			description:
				'Every classroom on the map: what it measures, who runs it, and how to host one.',
			href: '/classrooms'
		},
		{ id: 'learn', label: 'Learn', description: 'What the measurements mean.', href: null },
		{
			id: 'train',
			label: 'Train',
			description: 'Courses for taking good measurements.',
			href: null
		},
		{
			id: 'cop32',
			label: 'COP32 Addis',
			description: 'The network on its way to COP32 in Addis Ababa.',
			href: null
		},
		{ id: 'give', label: 'Give', description: 'Fund a Lab in a Bag for a classroom.', href: null }
	],
	policies: [
		{ id: 'data-terms', label: 'Data terms', description: 'How the data may be used.', href: null },
		{
			id: 'privacy',
			label: 'Privacy and consent',
			description: 'What we keep about people, and how consent works.',
			href: null
		},
		{
			id: 'safeguarding',
			label: 'Safeguarding',
			description: 'How young contributors are kept safe.',
			href: null
		}
	],
	stages: [
		{
			id: 'preparing',
			label: 'Preparing',
			description: 'Setting up the lab and training people.',
			color: '#f6921e'
		},
		{
			id: 'pilot',
			label: 'Pilot',
			description: 'Measuring, with the first data in review.',
			color: '#00bdff'
		},
		{
			id: 'operating',
			label: 'Operating',
			description: 'Measuring and releasing data regularly.',
			color: '#3cc04b'
		}
	],
	regions: [
		{ id: 'caribbean', label: 'Caribbean' },
		{ id: 'east_africa', label: 'East Africa' }
	],
	languages: [
		{ code: 'en', label: 'English', ready: true },
		{ code: 'am', label: 'አማርኛ', ready: false },
		{ code: 'so', label: 'Soomaali', ready: false },
		{ code: 'fr', label: 'Français', ready: false }
	]
};

/** The values the public core readings strip is made of. */
const coreParameters: MockDbState['coreParameters'] = {
	rainDaily: ['precip_accum_local_day', 'rain_gauge_daily_accumulation'],
	airTemperature: ['air_temperature'],
	rainTolerance: 2,
	rainReference: 'rain_gauge_daily_accumulation'
};

/**
 * "What's happening now?" on a station (AP-43): which values, in order, and their plain
 * captions. The first parameter with released data is shown.
 */
const nowTiles: MockDbState['nowTiles'] = [
	{
		id: 'temperature',
		label: 'Temperature',
		icon: 'thermometer',
		parameterIds: ['air_temperature'],
		caption: { parameterIds: ['feels_like'], text: 'Feels like {value}' }
	},
	{
		id: 'wind',
		label: 'Wind',
		icon: 'wind',
		parameterIds: ['wind_avg'],
		caption: { parameterIds: ['wind_direction'], text: 'From the {value}', format: 'compass' }
	},
	{
		id: 'uv',
		label: 'UV index',
		icon: 'sun',
		parameterIds: ['uv'],
		caption: { parameterIds: ['solar_radiation'], text: 'Solar radiation {value}' }
	},
	{
		id: 'pressure',
		label: 'Pressure',
		icon: 'waves',
		parameterIds: ['sea_level_pressure', 'station_pressure'],
		caption: { parameterIds: ['pressure_trend'], text: 'Trend: {value}' }
	}
];

/** The terms a new classroom's data starts with, until its head changes them. */
const defaultDataTerms: MockDbState['defaultDataTerms'] = {
	licence: 'CC BY 4.0',
	licenceUrl: 'https://creativecommons.org/licenses/by/4.0/',
	attribution: '{classroom}, {institution}. DaGrouper Global Classrooms.',
	restrictions: [],
	coordinateDecimals: null
};

const classrooms: MockDbState['classrooms'] = [
	{
		id: 'cls_trinity',
		name: 'Trinity Lab Classroom',
		institutionName: 'Trinity School, Addis Ababa',
		code: 'DGC-7K3Q9M',
		setup: 'admin_assisted',
		createdBy: 'usr_leila',
		createdAt: SEEDED_AT,
		stageId: 'operating',
		regionId: 'east_africa',
		releasedThrough: '2026-10-20T06:00:00.000Z',
		sharing: { ...DEFAULT_SHARING },
		profile: {
			areaCode: 'ET-AA',
			language: 'Amharic and English',
			description:
				'Grade 9 to 11 students and community scientists from Addis Ababa and Shashamane, studying rain-fed farmland and the school garden.',
			ecosystems: ['Highland farmland', 'School garden'],
			hostAgreementSigned: true,
			projects: [
				{
					id: 'prj_trinity_rain',
					title: 'Two towns, one rainy season',
					question: 'How does rainfall in Shashamane compare with Addis Ababa through the season?',
					sites: [
						{
							code: 'TRN-S1',
							name: 'Shashamane farm plot',
							ecosystem: 'Highland farmland',
							parameterIds: [
								'precip_accum_local_day',
								'rain_gauge_daily_accumulation',
								'air_temperature'
							],
							frequency: 'Daily',
							readingsPerDay: 1,
							yearsPlanned: 3
						},
						{
							code: 'TRN-S2',
							name: 'Trinity School roof',
							ecosystem: 'City',
							parameterIds: ['air_temperature', 'relative_humidity'],
							frequency: 'Daily',
							readingsPerDay: 1,
							yearsPlanned: 3
						}
					]
				},
				{
					id: 'prj_trinity_soil',
					title: 'When to plant',
					question: 'When is soil moisture high enough for planting in the school garden?',
					sites: [
						{
							code: 'TRN-S3',
							name: 'School garden',
							ecosystem: 'School garden',
							parameterIds: ['soil_moisture', 'soil_temperature'],
							frequency: 'Weekly',
							readingsPerDay: 1 / 7,
							yearsPlanned: 2
						}
					]
				}
			],
			sessions: [
				{
					id: 'ses_trinity_1',
					title: 'Rain gauge check and data review',
					startsAt: '2026-10-28T07:00:00.000Z',
					place: 'Trinity School lab'
				},
				{
					id: 'ses_trinity_2',
					title: 'Soil moisture survey in the garden',
					startsAt: '2026-11-18T07:00:00.000Z',
					place: 'School garden'
				},
				{
					id: 'ses_trinity_3',
					title: 'Season review with Shashamane',
					startsAt: '2026-12-09T08:00:00.000Z',
					place: 'Online'
				}
			],
			custodianRole: 'facilitator',
			institution: {
				photoUrl: null,
				photoAlt: 'Trinity School buildings and the school garden in Addis Ababa.',
				history:
					'Trinity School has taught in Addis Ababa since 1994. Its science club and community scientists in Shashamane measure rain and heat on highland farmland and in the school garden, and share what they find with farmers nearby.',
				hub: null,
				website: null
			}
		},
		terms: {
			licence: 'CC BY 4.0',
			licenceUrl: 'https://creativecommons.org/licenses/by/4.0/',
			attribution:
				'Trinity Lab Classroom, Trinity School, Addis Ababa. DaGrouper Global Classrooms.',
			restrictions: [],
			coordinateDecimals: null
		}
	},
	{
		id: 'cls_nassau',
		name: 'Bay Street Classroom',
		institutionName: 'Bay Street School, Nassau',
		code: 'DGC-BS4N7R',
		setup: 'admin_assisted',
		createdBy: 'usr_leila',
		createdAt: SEEDED_AT,
		stageId: 'pilot',
		regionId: 'caribbean',
		releasedThrough: '2026-10-20T08:00:00.000Z',
		sharing: { ...DEFAULT_SHARING },
		profile: {
			areaCode: 'BS-NP',
			language: 'English',
			description:
				'Secondary students on the Nassau harbour front, studying the shoreline and nearby reef through the storm season.',
			ecosystems: ['Shoreline', 'Coral reef'],
			hostAgreementSigned: true,
			projects: [
				{
					id: 'prj_nassau_storm',
					title: 'Heat on the shore',
					question: 'How do storm-season winds change heat stress on the shore?',
					sites: [
						{
							code: 'BSC-S1',
							name: 'Harbour front',
							ecosystem: 'Shoreline',
							parameterIds: ['wind_avg', 'wind_gust', 'air_temperature', 'uv'],
							frequency: 'Daily',
							readingsPerDay: 1,
							yearsPlanned: 2
						}
					]
				}
			],
			sessions: [
				{
					id: 'ses_nassau_1',
					title: 'Weather station cleaning',
					startsAt: '2026-11-21T14:00:00.000Z',
					place: 'Bay Street School'
				}
			],
			custodianRole: 'facilitator',
			institution: {
				photoUrl: null,
				photoAlt: 'Bay Street School on the Nassau harbour front.',
				history:
					'Bay Street School has stood on the Nassau harbour front since 1962. Its shoreline lab tracks wind, heat and storm conditions on the harbour, with student interns and local volunteers taking part.',
				hub: null,
				website: null
			}
		},
		terms: {
			licence: 'CC BY-NC 4.0',
			licenceUrl: 'https://creativecommons.org/licenses/by-nc/4.0/',
			attribution: 'Bay Street Classroom, Bay Street School, Nassau. DaGrouper Global Classrooms.',
			restrictions: ['No commercial use.'],
			coordinateDecimals: null
		}
	},
	{
		id: 'cls_hargeisa',
		name: 'Hargeisa Science Club',
		institutionName: 'Hargeisa Secondary School',
		code: 'DGC-HG2W5K',
		setup: 'admin_assisted',
		createdBy: 'usr_leila',
		createdAt: SEEDED_AT,
		stageId: 'preparing',
		regionId: 'east_africa',
		releasedThrough: null,
		sharing: { ...DEFAULT_SHARING },
		profile: {
			areaCode: 'SO-WO',
			language: 'Somali and English',
			description: 'A school science club studying dry-season heat and dust in the highlands.',
			ecosystems: ['Dry highland'],
			hostAgreementSigned: false,
			projects: [
				{
					id: 'prj_hargeisa_uv',
					title: 'Dry-season sun',
					question: 'How high does UV exposure get during the dry season?',
					sites: [
						{
							code: 'HSC-S1',
							name: 'School grounds',
							ecosystem: 'Dry highland',
							parameterIds: ['uv', 'air_temperature'],
							frequency: 'Daily',
							readingsPerDay: 1,
							yearsPlanned: 1
						}
					]
				}
			],
			sessions: [],
			custodianRole: 'institution_head',
			institution: {
				photoUrl: null,
				photoAlt: 'Hargeisa Secondary School grounds.',
				history:
					'A secondary school science club in Hargeisa studying dry-season heat and dust in the highlands.',
				hub: null,
				website: null
			}
		},
		terms: {
			licence: 'CC BY 4.0',
			licenceUrl: 'https://creativecommons.org/licenses/by/4.0/',
			attribution: 'Hargeisa Science Club, Hargeisa Secondary School. DaGrouper Global Classrooms.',
			restrictions: [
				'Do not publish positions more precisely than 0.01°: the lab is on school grounds.'
			],
			coordinateDecimals: 2
		}
	}
];

const memberships: MockDbState['memberships'] = [
	{
		classroomId: 'cls_trinity',
		userId: 'usr_tesfaye',
		role: 'institution_head',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_trinity',
		userId: 'usr_abebe',
		role: 'community_scientist',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_trinity',
		userId: 'usr_liya',
		role: 'community_scientist',
		joinedAt: SEEDED_AT
	},
	// The demo researcher follows every site; Sarah records for two of them.
	{
		classroomId: 'cls_trinity',
		userId: 'usr_marcus',
		role: 'researcher',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_nassau',
		userId: 'usr_marcus',
		role: 'researcher',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_hargeisa',
		userId: 'usr_marcus',
		role: 'researcher',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_nassau',
		userId: 'usr_sarah',
		role: 'community_scientist',
		joinedAt: SEEDED_AT
	},
	{
		classroomId: 'cls_hargeisa',
		userId: 'usr_sarah',
		role: 'community_scientist',
		joinedAt: SEEDED_AT
	}
];

const invitations: MockDbState['invitations'] = [
	{
		id: 'inv_hana',
		token: 'demo-hana-invite',
		classroomId: 'cls_trinity',
		email: 'hana.bekele@example.org',
		role: 'facilitator',
		status: 'pending',
		invitedBy: 'usr_tesfaye',
		createdAt: SEEDED_AT,
		acceptedBy: null
	}
];

const googleAccounts: MockDbState['googleAccounts'] = [
	{ sub: 'g-sarah', name: 'Sarah K.', email: 'sarah.k@example.org', avatarUrl: null },
	{ sub: 'g-marcus', name: 'Dr. Marcus L.', email: 'marcus.l@planetwerx.example', avatarUrl: null },
	{ sub: 'g-leila', name: 'Leila M.', email: 'leila.m@planetwerx.example', avatarUrl: null },
	{ sub: 'g-hana', name: 'Hana Bekele', email: 'hana.bekele@example.org', avatarUrl: null },
	{ sub: 'g-amina', name: 'Amina Yusuf', email: 'amina.yusuf@example.org', avatarUrl: null },
	{
		sub: 'g-tesfaye',
		name: 'Tesfaye G.',
		email: 'tesfaye.g@trinity-school.example',
		avatarUrl: null
	}
];

type ParameterSeed = [id: string, label: string, unit: string, definition: string];

const registry: Record<EnvironmentalDomain, ParameterSeed[]> = {
	AIR: [
		['air_temperature', 'Air temperature', '°C', 'Dry-bulb air temperature.'],
		[
			'feels_like',
			'Feels like',
			'°C',
			'Perceived temperature combining heat index and wind chill.'
		],
		['heat_index', 'Heat index', '°C', 'Perceived temperature from heat and humidity.'],
		['wind_chill', 'Wind chill', '°C', 'Perceived temperature from cold and wind.'],
		['relative_humidity', 'Relative humidity', '%', 'Moisture in the air relative to saturation.'],
		['dew_point', 'Dew point', '°C', 'Temperature at which the air becomes saturated.'],
		[
			'wet_bulb_temperature',
			'Wet bulb temperature',
			'°C',
			'Lowest temperature reachable by evaporative cooling.'
		],
		[
			'wet_bulb_globe_temperature',
			'Wet bulb globe temperature',
			'°C',
			'Heat stress index combining temperature, humidity, wind and sun.'
		],
		[
			'delta_t',
			'Delta T',
			'°C',
			'Dry bulb minus wet bulb: evaporative cooling potential for agricultural spraying.'
		],
		['air_density', 'Air density', 'kg/m³', 'Mass of air per unit volume.']
	],
	WAVES: [
		['station_pressure', 'Station pressure', 'hPa', 'Air pressure measured at the station.'],
		['barometric_pressure', 'Barometric pressure', 'hPa', 'Barometric air pressure.'],
		['sea_level_pressure', 'Sea level pressure', 'hPa', 'Pressure normalised to sea level.'],
		['pressure_trend', 'Pressure trend', '', 'Barometric trend: falling, steady or rising.']
	],
	SUN: [
		['solar_radiation', 'Solar radiation', 'W/m²', 'Incoming solar irradiance.'],
		['uv', 'UV index', '', 'Ultraviolet exposure index.'],
		['brightness', 'Brightness', 'lux', 'Illuminance.']
	],
	RAIN: [
		['precip', 'Precipitation rate', 'mm/min', 'Current precipitation intensity.'],
		['precip_accum_last_1hr', 'Rain, last hour', 'mm', 'Accumulation over the last hour.'],
		['precip_accum_local_day', 'Rain, today', 'mm', 'Accumulation since local midnight.'],
		[
			'precip_accum_local_day_final',
			'Rain, today (final)',
			'mm',
			'Quality-controlled accumulation since local midnight.'
		],
		[
			'precip_accum_local_yesterday',
			'Rain, yesterday',
			'mm',
			'Accumulation for the previous local day.'
		],
		[
			'precip_accum_local_yesterday_final',
			'Rain, yesterday (final)',
			'mm',
			'Quality-controlled accumulation for the previous local day.'
		],
		[
			'precip_analysis_type_yesterday',
			'Rain analysis type',
			'',
			'Analysis method used for yesterday’s total.'
		],
		[
			'precip_minutes_local_day',
			'Rain minutes, today',
			'min',
			'Minutes with rain since local midnight.'
		],
		[
			'precip_minutes_local_yesterday',
			'Rain minutes, yesterday',
			'min',
			'Minutes with rain on the previous local day.'
		],
		[
			'precip_minutes_local_yesterday_final',
			'Rain minutes, yesterday (final)',
			'min',
			'Quality-controlled minutes with rain on the previous local day.'
		]
	],
	WIND: [
		['wind_avg', 'Average wind speed', 'm/s', 'Mean wind speed.'],
		['wind_gust', 'Wind gust', 'm/s', 'Peak wind speed.'],
		['wind_lull', 'Wind lull', 'm/s', 'Minimum wind speed.'],
		['wind_direction', 'Wind direction', '°', 'Direction the wind blows from.']
	],
	LIGHTNING: [
		['lightning_strike_count', 'Lightning strikes', '', 'Strikes in the current interval.'],
		['lightning_strike_count_last_1hr', 'Strikes, last hour', '', 'Strikes in the last hour.'],
		[
			'lightning_strike_count_last_3hr',
			'Strikes, last 3 hours',
			'',
			'Strikes in the last three hours.'
		],
		[
			'lightning_strike_last_distance',
			'Last strike distance',
			'km',
			'Distance to the most recent strike.'
		],
		[
			'lightning_strike_last_epoch',
			'Last strike time',
			's',
			'When the most recent strike was, as Unix time.'
		]
	]
};

const parameters: Parameter[] = [
	...Object.entries(registry).flatMap(([domain, rows]) =>
		rows.map(([id, label, unit, definition]): Parameter => ({
			id,
			label,
			unit,
			definition,
			domain: domain as EnvironmentalDomain,
			source: 'weather_station',
			valueType: id === 'pressure_trend' ? 'text' : id.endsWith('_epoch') ? 'time' : 'number'
		}))
	),
	{
		id: 'rain_gauge_daily_accumulation',
		label: 'Manual rain gauge, daily',
		unit: 'mm',
		definition: 'Daily accumulation read from a manual rain gauge.',
		domain: 'RAIN',
		source: 'manual'
	},
	// Values from the other built-in device types. Air quality sits under AIR; soil and water
	// have no domain yet, so they show as unmapped until a platform admin adds one (SAM-23).
	...(
		[
			['soil_moisture', 'Soil moisture', '%', 'Volumetric water content of the soil.', null],
			[
				'soil_temperature',
				'Soil temperature',
				'°C',
				'Temperature a few cm below the surface.',
				null
			],
			['water_temperature', 'Water temperature', '°C', 'Temperature of the water body.', null],
			['water_ph', 'Water pH', '', 'Acidity of the water, 0 to 14.', null],
			['pm2_5', 'PM2.5', 'µg/m³', 'Fine particles up to 2.5 µm across.', 'AIR'],
			['pm10', 'PM10', 'µg/m³', 'Particles up to 10 µm across.', 'AIR']
		] as const
	).map(([id, label, unit, definition, domain]): Parameter => ({
		id,
		label,
		unit,
		definition,
		domain,
		source: 'device'
	})),
	// Values only people record, from the field observation templates below.
	{
		id: 'coral_spawning_observed',
		label: 'Spawning seen',
		unit: '',
		definition: 'Whether corals were releasing eggs and sperm during the check.',
		domain: 'REEF',
		source: 'manual',
		valueType: 'text',
		options: ['Yes', 'No', 'Not sure']
	},
	{
		id: 'coral_colonies_spawning',
		label: 'Colonies spawning',
		unit: 'colonies',
		definition: 'How many coral colonies were seen spawning.',
		domain: 'REEF',
		source: 'manual'
	},
	{
		id: 'species_name',
		label: 'Species',
		unit: '',
		definition: 'Common or scientific name of what was seen.',
		domain: 'LIFE',
		source: 'manual',
		valueType: 'text'
	},
	{
		id: 'species_count',
		label: 'How many',
		unit: 'individuals',
		definition: 'Number of individuals seen.',
		domain: 'LIFE',
		source: 'manual'
	}
];

const CELSIUS_FROM_F: AltUnit = { unit: '°F', factor: 5 / 9, offset: -160 / 9 };
const HPA_FROM_INHG: AltUnit = { unit: 'inHg', factor: 33.8639 };
const MS_FROM: AltUnit[] = [
	{ unit: 'km/h', factor: 1 / 3.6 },
	{ unit: 'mph', factor: 0.44704 }
];

/**
 * Pre-submission checks for values people enter by hand (AP-16): the plausible range in the
 * value's own unit, and other units it may be measured in.
 */
const checks: Record<string, Pick<Parameter, 'min' | 'max' | 'altUnits'>> = {
	air_temperature: { min: -60, max: 60, altUnits: [CELSIUS_FROM_F] },
	relative_humidity: { min: 0, max: 100 },
	wind_avg: { min: 0, max: 80, altUnits: MS_FROM },
	station_pressure: { min: 500, max: 1100, altUnits: [HPA_FROM_INHG] },
	rain_gauge_daily_accumulation: { min: 0, max: 500, altUnits: [{ unit: 'in', factor: 25.4 }] },
	soil_moisture: { min: 0, max: 100 },
	soil_temperature: { min: -20, max: 70, altUnits: [CELSIUS_FROM_F] },
	water_temperature: { min: -2, max: 45, altUnits: [CELSIUS_FROM_F] },
	water_ph: { min: 0, max: 14 },
	pm2_5: { min: 0, max: 1000 },
	pm10: { min: 0, max: 2000 },
	coral_colonies_spawning: { min: 0, max: 1000 },
	species_count: { min: 1, max: 100000 }
};
for (const parameter of parameters) Object.assign(parameter, checks[parameter.id]);

/** The SAM registry release the mock serves. Calculations cite it. */
const registryVersion: MockDbState['registryVersion'] = {
	version: 'SAM v1.4',
	publishedAt: '2026-10-01'
};

/** How SAM gets a value with no formula of its own, by where the value comes from. */
export const SOURCE_CALCULATION: Record<
	Parameter['source'],
	Pick<Calculation, 'method' | 'formula'>
> = {
	weather_station: { method: 'measured', formula: 'Measured directly by the weather station.' },
	device: { method: 'measured', formula: 'Measured directly by the sensor.' },
	manual: { method: 'manual', formula: 'Read by a person and entered on a guided form.' }
};

/**
 * Plain-language text for indicators (AP-07): what a value means, the change a sensor can
 * wobble by, and formulas for derived values. States what is measured; no advice and no cause
 * and effect (a unit test checks).
 */
const indicators: Record<
	string,
	Pick<Parameter, 'plain' | 'noise'> & { formula?: string; inputs?: string[] }
> = {
	air_temperature: {
		plain: 'How warm the air is in the shade, measured about 2 m above the ground.',
		noise: 0.3
	},
	relative_humidity: {
		plain:
			'How much water vapour the air holds, as a share of the most it could hold at this temperature.',
		noise: 2
	},
	wind_avg: {
		plain: 'How fast the air is moving, averaged over a short period.',
		noise: 0.5
	},
	precip_accum_local_day: {
		plain:
			'How much rain has fallen since local midnight, as the depth of water it would leave on flat ground.',
		noise: 0.2
	},
	uv: {
		plain: "How strong the sun's ultraviolet light is. Higher numbers mean stronger light.",
		noise: 0.3
	},
	lightning_strike_last_epoch: {
		plain: 'When the most recent lightning strike was, in your own time zone.'
	},
	station_pressure: {
		plain:
			'The weight of the air above the station. It is lower at higher places and changes with the weather.',
		noise: 0.5
	},
	heat_index: {
		plain: 'How warm it feels to people once humidity is added to the air temperature.',
		noise: 0.5,
		formula:
			'HI = −8.78 + 1.611·T + 2.339·RH − 0.1461·T·RH − 0.01231·T² − 0.01642·RH² + 0.002212·T²·RH + 0.000725·T·RH² − 0.000003582·T²·RH², with T in °C and RH in %. Below 27 °C the heat index equals the air temperature.',
		inputs: ['air_temperature', 'relative_humidity']
	},
	feels_like: {
		plain: 'How warm or cold it feels to people, from the air temperature, humidity and wind.',
		noise: 0.5,
		formula:
			'The heat index when the air is 27 °C or warmer, the wind chill when it is 10 °C or colder with wind over 1.3 m/s, otherwise the air temperature.',
		inputs: ['air_temperature', 'relative_humidity', 'wind_avg']
	},
	dew_point: {
		plain:
			'The temperature the air would have to cool to before dew forms. Higher means more moisture in the air.',
		noise: 0.5,
		formula:
			'Magnus formula: γ = ln(RH/100) + 17.62·T / (243.12 + T), then Td = 243.12·γ / (17.62 − γ), with T in °C and RH in %.',
		inputs: ['air_temperature', 'relative_humidity']
	},
	wind_chill: {
		plain: 'How cold it feels to people once the wind is added to the air temperature.',
		noise: 0.5,
		formula:
			'WC = 13.12 + 0.6215·T − 11.37·V^0.16 + 0.3965·T·V^0.16, with T in °C and V the wind speed in km/h.',
		inputs: ['air_temperature', 'wind_avg']
	},
	rain_gauge_daily_accumulation: {
		plain:
			'How much rain a person read from the gauge for the day, as the depth of water collected.',
		noise: 0.5
	},
	soil_moisture: {
		plain: 'How much water the soil holds, as a share of its volume.',
		noise: 1
	},
	soil_temperature: {
		plain: 'How warm the soil is a few centimetres below the surface.',
		noise: 0.3
	},
	water_temperature: {
		plain: 'How warm the water is where the probe sits.',
		noise: 0.2
	},
	water_ph: {
		plain:
			'How acidic or alkaline the water is. 7 is neutral, lower is more acidic and higher is more alkaline.',
		noise: 0.1
	},
	pm2_5: {
		plain: 'How much fine dust and smoke is in the air, counting particles up to 2.5 µm across.',
		noise: 2
	},
	pm10: {
		plain: 'How much dust is in the air, counting particles up to 10 µm across.',
		noise: 3
	}
};
for (const parameter of parameters) {
	const { formula, inputs, ...text } = indicators[parameter.id] ?? {};
	Object.assign(parameter, text, {
		calculation: {
			...(formula ? { method: 'derived' as const, formula } : SOURCE_CALCULATION[parameter.source]),
			inputs: inputs ?? [],
			...registryVersion
		}
	});
}

/** Each value's main chart on its page (AP-23); anything else numeric charts as a line. */
const CHARTS: Record<string, Parameter['chart']> = {
	rain_gauge_daily_accumulation: 'daily_bars',
	precip_accum_local_day: 'daily_bars',
	air_temperature: 'line_band',
	soil_temperature: 'line_band'
};
for (const parameter of parameters) {
	if (CHARTS[parameter.id]) parameter.chart = CHARTS[parameter.id];
}

const labs: MockDbState['labs'] = [
	{
		id: 'lab_trinity',
		classroomId: 'cls_trinity',
		name: 'Trinity Lab',
		placeName: 'Trinity School, Addis Ababa',
		country: 'Ethiopia',
		location: { lat: 9.03, lng: 38.74 },
		siteProfile: {
			setting: 'urban',
			description: 'School lab in the capital, working with a partner station in Shashamane.',
			researchQuestions: [
				'How does rainfall in Shashamane compare with Addis Ababa through the season?',
				'When is soil moisture high enough for planting in the school garden?'
			],
			parameterNotes: {
				rain_gauge_daily_accumulation:
					'Our gauge in Shashamane is read by hand every morning and counts as the reference. The station next to it measures the same rain, so a big gap between the two sends the day to review.',
				air_temperature:
					'The station sits in open ground at the Shashamane partner site, not in the school yard. Highland mornings are cool, so the day’s low and high are often far apart.'
			}
		},
		createdAt: SEEDED_AT
	},
	{
		id: 'lab_nassau',
		classroomId: 'cls_nassau',
		name: 'Bay Street Coastal Lab',
		placeName: 'Bay Street School, Nassau',
		country: 'Bahamas',
		location: { lat: 25.077, lng: -77.339 },
		siteProfile: {
			setting: 'coastal',
			description: 'Shoreline lab tracking wind, heat and storm conditions on the harbour.',
			researchQuestions: ['How do storm-season winds change heat stress on the shore?']
		},
		createdAt: SEEDED_AT
	},
	{
		id: 'lab_hargeisa',
		classroomId: 'cls_hargeisa',
		name: 'Hargeisa School Lab',
		placeName: 'Hargeisa',
		country: 'Somalia',
		location: { lat: 9.56, lng: 44.065 },
		siteProfile: {
			setting: 'highland',
			description: 'Highland school lab studying dry-season heat and dust.',
			researchQuestions: ['How high does UV exposure get during the dry season?']
		},
		createdAt: SEEDED_AT
	}
];

// The Shashamane devices feed the Trinity lab from about 220 km away (AP-05). Devices carry no
// location; their readings do.
const devices: MockDbState['devices'] = [
	{
		id: 'dev_shashamane_ws',
		labId: 'lab_trinity',
		name: 'Shashamane Weather Station',
		type: 'weather_station',
		identifier: 'TWS-2041-SH',
		registeredAt: SEEDED_AT
	},
	{
		id: 'dev_shashamane_gauge',
		labId: 'lab_trinity',
		name: 'Shashamane Rain Gauge',
		type: 'rain_gauge',
		identifier: 'RG-SH-01',
		registeredAt: SEEDED_AT
	},
	{
		id: 'dev_trinity_soil',
		labId: 'lab_trinity',
		name: 'School Garden Soil Sensor',
		type: 'soil_sensor',
		identifier: 'SOIL-AA-07',
		registeredAt: SEEDED_AT
	},
	{
		id: 'dev_nassau_ws',
		labId: 'lab_nassau',
		name: 'Nassau Harbour Weather Station',
		type: 'weather_station',
		identifier: 'TWS-1187-NA',
		registeredAt: SEEDED_AT
	},
	{
		id: 'dev_hargeisa_ws',
		labId: 'lab_hargeisa',
		name: 'Hargeisa Weather Station',
		type: 'weather_station',
		identifier: 'TWS-3302-HG',
		registeredAt: SEEDED_AT
	}
];

const sample: Reading['values'] = {
	air_density: 1.17629,
	air_temperature: 27,
	barometric_pressure: 1013.5,
	brightness: 41591,
	delta_t: 4.3,
	dew_point: 20.8,
	feels_like: 28.8,
	heat_index: 28.8,
	lightning_strike_count: 0,
	lightning_strike_count_last_1hr: 0,
	lightning_strike_count_last_3hr: 0,
	lightning_strike_last_distance: 38,
	// Unix seconds: 2026-10-20 05:47 UTC, a couple of hours before the latest reading.
	lightning_strike_last_epoch: 1792475220,
	precip: 0,
	precip_accum_last_1hr: 0,
	precip_accum_local_day: 0,
	precip_accum_local_day_final: 0,
	precip_accum_local_yesterday: 0.297905,
	precip_accum_local_yesterday_final: 0.006257,
	precip_analysis_type_yesterday: 1,
	precip_minutes_local_day: 0,
	precip_minutes_local_yesterday: 24,
	precip_minutes_local_yesterday_final: 24,
	pressure_trend: 'steady',
	relative_humidity: 69,
	sea_level_pressure: 1014.9,
	solar_radiation: 346,
	station_pressure: 1013.5,
	uv: 3.4,
	wet_bulb_globe_temperature: 24.7,
	wet_bulb_temperature: 22.7,
	wind_avg: 3.5,
	wind_chill: 27,
	wind_direction: 123,
	wind_gust: 5,
	wind_lull: 2
};

const READING_AT = '2026-10-20T08:00:00.000Z';
const YESTERDAY = '2026-10-19T08:00:00.000Z';

/**
 * Seed readings. A device's GPS travels with each reading, so the Shashamane station shows
 * two positions on two days. The rain gauge and the Hargeisa station send no GPS: their
 * readings use the lab's location and are marked as such.
 */
function reading(
	deviceId: string,
	observedAt: string,
	values: Reading['values'],
	gps: GeoPoint | null
): MockDbState['readings'][number] {
	const device = devices.find((d) => d.id === deviceId)!;
	const lab = labs.find((l) => l.id === device.labId)!;
	const classroom = classrooms.find((c) => c.id === lab.classroomId)!;
	return {
		id: `rdg_${deviceId.slice(4)}_${observedAt.slice(0, 13)}`,
		deviceId,
		labId: lab.id,
		classroomId: classroom.id,
		observedAt,
		receivedAt: observedAt,
		values,
		location: gps ?? { ...lab.location },
		locationSource: gps ? 'device' : 'lab',
		samSourceId: `${classroom.code}:${device.identifier}`,
		creditedToUserId: null,
		recordedByUserId: null
	};
}

const readings: MockDbState['readings'] = [
	// Midday sun in Shashamane: high enough UV to raise an alert.
	reading('dev_shashamane_ws', READING_AT, { ...sample, uv: 6.8 }, { lat: 7.2, lng: 38.6 }),
	reading(
		'dev_shashamane_ws',
		YESTERDAY,
		{ ...sample, air_temperature: 25.6, relative_humidity: 74, uv: 2.9 },
		{ lat: 7.215, lng: 38.588 }
	),
	reading('dev_shashamane_gauge', READING_AT, { rain_gauge_daily_accumulation: 4.2 }, null),
	reading(
		'dev_trinity_soil',
		READING_AT,
		{ soil_moisture: 31, soil_temperature: 18.4 },
		{ lat: 9.031, lng: 38.742 }
	),
	reading(
		'dev_nassau_ws',
		READING_AT,
		{
			...sample,
			air_temperature: 29.4,
			relative_humidity: 78,
			wind_avg: 6.1,
			wind_gust: 9.2,
			uv: 7.8
		},
		{ lat: 25.06, lng: -77.34 }
	),
	reading(
		'dev_hargeisa_ws',
		READING_AT,
		{ ...sample, air_temperature: 24.1, relative_humidity: 41, wind_avg: 4.4, uv: 9.1 },
		null
	)
];

/** How far each parameter wanders from day to day in the seeded history. */
const DAILY_SWING: Record<string, number> = {
	air_temperature: 2.5,
	relative_humidity: 8,
	wind_avg: 1.5,
	wind_gust: 2,
	uv: 1.8,
	precip_accum_local_day: 3,
	rain_gauge_daily_accumulation: 3,
	soil_moisture: 4,
	soil_temperature: 1.5
};
const HISTORY_DAYS = 30;
/** Days back with no reading, so completeness and charts show real gaps. */
const GAP_DAYS: Record<string, number[]> = {
	dev_shashamane_gauge: [3, 4, 11],
	dev_nassau_ws: [6]
};
const HOUR = 3_600_000;
/** Extra readings on each past day: hours from the day's main reading, and how far they swing. */
const DAY_SPREAD: [number, number][] = [
	[-5, -1.2],
	[4, 0.9]
];

/**
 * A month of daily readings behind each device's latest one, plus hourly readings over the last
 * day, so the 24 h, 7 d and 30 d views all have a history. Values swing smoothly around the
 * latest reading; UV follows the sun through the day; no reading goes below zero.
 */
function history(): MockDbState['readings'] {
	const result: MockDbState['readings'] = [];
	devices.forEach((device, d) => {
		const latest = readings.find((r) => r.deviceId === device.id && r.observedAt === READING_AT)!;
		const gps = latest.locationSource === 'device' ? latest.location : null;
		const add = (observedAt: string, wave: number, daylight: number | null) => {
			if (readings.some((r) => r.deviceId === device.id && r.observedAt === observedAt)) return;
			const values = { ...latest.values };
			for (const [id, swing] of Object.entries(DAILY_SWING)) {
				const value = values[id];
				if (typeof value !== 'number') continue;
				const shifted =
					id === 'uv' && daylight !== null ? value * daylight : value + (swing * wave) / 1.5;
				values[id] = Math.max(0, Math.round(shifted * 10) / 10);
			}
			// Sunlight follows the sun by the hour, and the cloud cover from day to day.
			for (const id of ['solar_radiation', 'brightness']) {
				const value = values[id];
				if (typeof value !== 'number') continue;
				values[id] = Math.round(value * (daylight ?? 1 + wave / 4));
			}
			result.push(reading(device.id, observedAt, values, gps));
		};
		for (let day = 1; day < HISTORY_DAYS; day++) {
			if (GAP_DAYS[device.id]?.includes(day)) continue;
			const wave = Math.sin(day * 0.9 + d) + 0.5 * Math.sin(day * 2.3 + d * 2);
			const at = Date.parse(READING_AT) - day * 24 * HOUR;
			add(new Date(at).toISOString(), wave, null);
			// Early morning and mid-afternoon too, so each day has a low and a high to chart.
			for (const [hours, shift] of DAY_SPREAD) {
				const extra = at + hours * HOUR;
				// The last day has its own hourly readings.
				if (extra > Date.parse(READING_AT) - 24 * HOUR) continue;
				add(new Date(extra).toISOString(), wave + shift, null);
			}
		}
		for (let hour = 1; hour < 24; hour++) {
			const at = Date.parse(READING_AT) - hour * HOUR;
			// Local solar time, roughly: the sun is up from 6 to 18 at the lab's longitude.
			const lab = labs.find((l) => l.id === device.labId)!;
			const solar = (new Date(at).getUTCHours() + lab.location.lng / 15 + 24) % 24;
			const daylight = Math.max(0, Math.sin(((solar - 6) / 12) * Math.PI));
			add(new Date(at).toISOString(), 0.6 * Math.sin(hour * 0.5 + d) + daylight - 0.5, daylight);
		}
	});
	return result;
}

readings.push(...history());

/**
 * The Shashamane station stands next to the manual gauge, so its rain follows the gauge's to
 * within a few tenths of a mm. On two days its funnel was blocked and it read well over, which
 * the rain page flags for review (AP-23).
 */
const BLOCKED_FUNNEL_DAYS = ['2026-10-07', '2026-10-14'];
const gaugeReadings = readings
	.filter((r) => r.deviceId === 'dev_shashamane_gauge')
	.sort((a, b) => a.observedAt.localeCompare(b.observedAt));
for (const r of readings.filter((r) => r.deviceId === 'dev_shashamane_ws')) {
	const day = r.observedAt.slice(0, 10);
	const sameDay = gaugeReadings.filter((g) => g.observedAt.slice(0, 10) === day);
	if (!sameDay.length) continue;
	const upToNow = sameDay.filter((g) => g.observedAt <= r.observedAt);
	const gauge = Math.max(
		...(upToNow.length ? upToNow : sameDay.slice(0, 1)).map(
			(g) => g.values.rain_gauge_daily_accumulation as number
		)
	);
	const jitter = ((Number(day.slice(8)) % 5) - 2) * 0.2;
	const over = BLOCKED_FUNNEL_DAYS.includes(day) ? 3.5 : 0;
	r.values = {
		...r.values,
		precip_accum_local_day: Math.max(0, Math.round((gauge + jitter + over) * 10) / 10)
	};
}

const domains: DomainDefinition[] = [
	{ id: 'AIR', label: 'Air', icon: 'tree', description: 'Temperature, humidity and air quality.' },
	{ id: 'RAIN', label: 'Rain', icon: 'rain', description: 'Rainfall rates and totals.' },
	{ id: 'WIND', label: 'Wind', icon: 'wind', description: 'Wind speed and direction.' },
	{ id: 'SUN', label: 'Sun', icon: 'sun', description: 'Sunlight, UV and brightness.' },
	{ id: 'WAVES', label: 'Waves', icon: 'waves', description: 'Air pressure and its trend.' },
	{ id: 'LIGHTNING', label: 'Lightning', icon: 'bolt', description: 'Strikes and their distance.' },
	{ id: 'REEF', label: 'Reefs', icon: 'droplet', description: 'Coral health and spawning.' },
	{ id: 'LIFE', label: 'Wildlife', icon: 'sprout', description: 'Species seen and how many.' }
];

const deviceTypes: DeviceTypeDefinition[] = [
	{
		id: 'weather_station',
		label: 'Weather station',
		description: 'Tempest-style station: air, rain, wind, sun, pressure and lightning.',
		color: '#3cc04b',
		icon: 'station',
		parameterIds: parameters.filter((p) => p.source === 'weather_station').map((p) => p.id),
		// In the registry's own order, as the Registry form keeps them.
		tileParameterIds: parameters
			.filter((p) => ['air_temperature', 'precip_accum_local_day', 'uv', 'wind_avg'].includes(p.id))
			.map((p) => p.id),
		cardParameterIds: parameters
			.filter((p) => ['air_temperature', 'precip_accum_local_day', 'uv'].includes(p.id))
			.map((p) => p.id)
	},
	{
		id: 'rain_gauge',
		label: 'Rain gauge',
		description: 'Manual gauge read once a day.',
		color: '#00bdff',
		icon: 'gauge',
		parameterIds: ['rain_gauge_daily_accumulation'],
		tileParameterIds: ['rain_gauge_daily_accumulation'],
		cardParameterIds: ['rain_gauge_daily_accumulation'],
		manual: true
	},
	{
		id: 'soil_sensor',
		label: 'Soil sensor',
		description: 'Probe in the ground: moisture and temperature.',
		color: '#f6921e',
		icon: 'soil',
		parameterIds: ['soil_moisture', 'soil_temperature'],
		tileParameterIds: ['soil_moisture', 'soil_temperature'],
		cardParameterIds: ['soil_moisture', 'soil_temperature']
	},
	{
		id: 'water_probe',
		label: 'Water probe',
		description: 'Probe in a stream, pond or tank.',
		color: '#2f7ff0',
		icon: 'water',
		parameterIds: ['water_temperature', 'water_ph'],
		tileParameterIds: ['water_temperature', 'water_ph'],
		cardParameterIds: ['water_temperature', 'water_ph']
	},
	{
		id: 'air_quality_sensor',
		label: 'Air quality sensor',
		description: 'Particulate matter counter.',
		color: '#e8457a',
		icon: 'particles',
		parameterIds: ['pm2_5', 'pm10'],
		tileParameterIds: ['pm2_5', 'pm10'],
		cardParameterIds: ['pm2_5', 'pm10']
	},
	{
		id: 'other',
		label: 'Other',
		description: 'Anything else; shows whatever it sends.',
		color: '#9aa5ae',
		icon: 'sensor',
		parameterIds: [],
		tileParameterIds: [],
		cardParameterIds: []
	}
];

/**
 * Platform default alert levels (AP-08). A classroom that sets its own levels for a parameter
 * replaces these for that parameter. Titles state the reading; they give no advice.
 */
const alertDefaults: AlertRule[] = (
	[
		['uv', 'above', 8, 'warning', 'Very high UV'],
		['uv', 'above', 6, 'caution', 'High UV'],
		['heat_index', 'above', 39, 'warning', 'Dangerous heat'],
		['air_temperature', 'above', 32, 'caution', 'Hot'],
		['wind_gust', 'above', 17, 'warning', 'Strong gusts'],
		['precip_accum_local_day', 'above', 50, 'warning', 'Heavy rain'],
		['rain_gauge_daily_accumulation', 'above', 50, 'warning', 'Heavy rain'],
		['soil_moisture', 'below', 15, 'caution', 'Dry soil']
	] as const
).map(([parameterId, direction, value, level, title]) => ({
	id: `alr_default_${parameterId}_${level}`,
	parameterId,
	direction,
	value,
	level,
	title,
	enabled: true
}));

/** A field observation recorded by a person, before the release date so the public sees it. */
function fieldObservation(
	id: string,
	labId: string,
	userId: string,
	credit: boolean,
	observedAt: string,
	values: Reading['values']
): MockDbState['observations'][number] {
	const lab = labs.find((l) => l.id === labId)!;
	const classroom = classrooms.find((c) => c.id === lab.classroomId)!;
	return {
		id,
		templateId: 'tpl_species_sighting',
		deviceId: null,
		labId,
		classroomId: classroom.id,
		observedAt,
		receivedAt: observedAt,
		values,
		location: { ...lab.location },
		locationSource: 'lab',
		samSourceId: classroom.code,
		creditedToUserId: credit ? userId : null,
		recordedByUserId: userId
	};
}

// Abebe takes credit; Tesfaye does not, so the profile counts him without naming him.
const observations: MockDbState['observations'] = [
	fieldObservation(
		'obs_trinity_kites',
		'lab_trinity',
		'usr_abebe',
		true,
		'2026-10-12T07:30:00.000Z',
		{
			species_name: 'Black kite',
			species_count: 4
		}
	),
	fieldObservation(
		'obs_trinity_ibis',
		'lab_trinity',
		'usr_tesfaye',
		false,
		'2026-10-14T06:45:00.000Z',
		{
			species_name: 'Hadada ibis',
			species_count: 2
		}
	)
];

/** Guided forms for manual observations (AP-16). Device types without one get a plain form. */
const observationTemplates: ObservationTemplate[] = [
	{
		id: 'tpl_rain_gauge',
		label: 'Rain gauge reading',
		description: 'Read the gauge once a day, at the same time, then empty it.',
		domain: 'RAIN',
		deviceType: 'rain_gauge',
		fields: [{ parameterId: 'rain_gauge_daily_accumulation', required: true }]
	},
	{
		id: 'tpl_station_spot_check',
		label: 'Weather station spot check',
		description: 'Copy what the station display shows, to check it against what it sends.',
		domain: 'AIR',
		deviceType: 'weather_station',
		fields: [
			{ parameterId: 'air_temperature', required: true },
			{ parameterId: 'relative_humidity', required: false },
			{ parameterId: 'wind_avg', required: false },
			{ parameterId: 'station_pressure', required: false }
		]
	},
	{
		id: 'tpl_soil_check',
		label: 'Soil check',
		description: 'Read the soil sensor, or a handheld probe pushed in next to it.',
		domain: null,
		deviceType: 'soil_sensor',
		fields: [
			{ parameterId: 'soil_moisture', required: true },
			{ parameterId: 'soil_temperature', required: false }
		]
	},
	{
		id: 'tpl_water_check',
		label: 'Water check',
		description: 'Take the probe reading at the usual spot, away from the edge.',
		domain: null,
		deviceType: 'water_probe',
		fields: [
			{ parameterId: 'water_temperature', required: true },
			{ parameterId: 'water_ph', required: true }
		]
	},
	{
		id: 'tpl_air_quality',
		label: 'Air quality reading',
		description: 'Read the counter after it has run for a minute.',
		domain: 'AIR',
		deviceType: 'air_quality_sensor',
		fields: [
			{ parameterId: 'pm2_5', required: true },
			{ parameterId: 'pm10', required: false }
		]
	},
	{
		id: 'tpl_coral_spawning',
		label: 'Coral spawning',
		description: 'A night dive or snorkel check on the reef after the full moon.',
		domain: 'REEF',
		deviceType: null,
		fields: [
			{ parameterId: 'coral_spawning_observed', required: true },
			{ parameterId: 'coral_colonies_spawning', required: false },
			{ parameterId: 'water_temperature', required: false }
		]
	},
	{
		id: 'tpl_species_sighting',
		label: 'Species sighting',
		description: 'A plant or animal you identified, and how many you saw.',
		domain: 'LIFE',
		deviceType: null,
		fields: [
			{ parameterId: 'species_name', required: true },
			{ parameterId: 'species_count', required: true }
		]
	}
];

/** Readiness questions for hosting a classroom (AP-30). Draft for the programme team to replace. */
const hostQuestions: MockDbState['hostQuestions'] = [
	{
		id: 'storage',
		label: 'Is there a safe, dry place to keep the Lab in a Bag?',
		help: 'A locked cupboard or room is enough.',
		kind: 'yes_no',
		required: true
	},
	{
		id: 'lead',
		label: 'Is there a staff member who can lead a session each week?',
		help: 'They would become the classroom’s facilitator.',
		kind: 'yes_no',
		required: true
	},
	{
		id: 'connection',
		label: 'Is there mobile data or wifi at least once a week?',
		help: 'Observations are saved on the phone and sync when there is a connection.',
		kind: 'yes_no',
		required: true
	},
	{
		id: 'learners',
		label: 'About how many learners would take part?',
		help: '',
		kind: 'number',
		required: true
	},
	{
		id: 'notes',
		label: 'Anything else we should know?',
		help: '',
		kind: 'text',
		required: false
	}
];

/** One application waiting for review, so the back office has something to show. */
const hostApplications: MockDbState['hostApplications'] = [
	{
		id: 'happ_andros',
		token: 'host_andros_demo',
		institutionName: 'North Andros High School',
		contactName: 'Renee P.',
		contactRole: 'Science teacher',
		contactEmail: 'renee.p@example.org',
		contactPhone: '',
		answers: { storage: 'yes', lead: 'yes', connection: 'no', learners: '24' },
		site: {
			placeName: 'Nicholls Town, Andros',
			country: 'Bahamas',
			ecosystems: ['Mangrove', 'Coral reef'],
			learners: 'Grade 10 and 11 students and their families.',
			studyGoals: 'How the mangrove shoreline changes after storms.'
		},
		status: 'submitted',
		submittedAt: '2026-10-30T15:00:00.000Z',
		decidedAt: null,
		classroomId: null
	}
];

export function createSeed(): Omit<MockDbState, 'version'> {
	// Deep copy so a reset never shares objects with mutated state.
	return structuredClone({
		users,
		googleAccounts,
		googleLinks: {
			'g-sarah': 'usr_sarah',
			'g-marcus': 'usr_marcus',
			'g-leila': 'usr_leila',
			'g-tesfaye': 'usr_tesfaye'
		},
		challenges: [],
		session: null,
		parameters,
		domains,
		deviceTypes,
		registryChanges: [],
		registryVersion,
		observationTemplates,
		alertDefaults,
		alertRules: [],
		readings,
		observations,
		classrooms,
		defaultDataTerms,
		siteContent,
		coreParameters,
		nowTiles,
		memberships,
		invitations,
		labs,
		devices,
		guardianConsents: [
			{
				userId: 'usr_liya',
				token: 'consent_liya_demo',
				method: 'link' as const,
				signedOn: null,
				recordedBy: null,
				guardianName: 'Almaz A.',
				guardianContact: '+251911000004',
				status: 'pending' as const,
				requestedAt: SEEDED_AT,
				decidedAt: null
			}
		],
		samAssertions: [],
		privacy: { minorCoordinateDecimals: 2 },
		hostQuestions,
		hostApplications,
		samSources: [
			...classrooms.map((c) => ({
				sourceId: c.code,
				kind: 'classroom' as const,
				registeredAt: c.createdAt
			})),
			...devices.map((d) => {
				const lab = labs.find((l) => l.id === d.labId)!;
				const code = classrooms.find((c) => c.id === lab.classroomId)!.code;
				return {
					sourceId: `${code}:${d.identifier}`,
					kind: 'device' as const,
					registeredAt: d.registeredAt
				};
			})
		]
	});
}
