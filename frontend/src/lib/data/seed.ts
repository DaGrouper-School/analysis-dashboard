import type { EnvironmentalDomain, GeoPoint, Parameter, Reading, User } from '#lib/domain/types.ts';
import type { MockDbState } from './mock-db';

/**
 * Prefilled demo data. Personas come from the vault (User Personas); parameter values come
 * from the sample weather-station payload (User Flow / Sensor Data).
 */

const SEEDED_AT = '2026-10-01T09:00:00.000Z';

const users: User[] = [
	{
		id: 'usr_sarah',
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
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_abebe',
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
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_marcus',
		authProvider: 'google',
		fullName: 'Dr. Marcus L.',
		email: 'marcus.l@planetwerx.example',
		phone: '+14155550123',
		phoneVerified: true,
		avatarUrl: null,
		role: 'researcher',
		platformAdmin: true,
		address: {
			street: '200 Research Way',
			city: 'San Francisco',
			state: 'CA',
			zip: '94107',
			country: 'United States'
		},
		onboardingCompleted: true,
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	},
	{
		id: 'usr_tesfaye',
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
		createdAt: SEEDED_AT,
		updatedAt: SEEDED_AT
	}
];

const classrooms: MockDbState['classrooms'] = [
	{
		id: 'cls_trinity',
		name: 'Trinity Lab Classroom',
		institutionName: 'Trinity School, Addis Ababa',
		code: 'DGC-7K3Q9M',
		setup: 'admin_assisted',
		createdBy: 'usr_marcus',
		createdAt: SEEDED_AT
	},
	{
		id: 'cls_nassau',
		name: 'Bay Street Classroom',
		institutionName: 'Bay Street School, Nassau',
		code: 'DGC-BS4N7R',
		setup: 'admin_assisted',
		createdBy: 'usr_marcus',
		createdAt: SEEDED_AT
	},
	{
		id: 'cls_hargeisa',
		name: 'Hargeisa Science Club',
		institutionName: 'Hargeisa Secondary School',
		code: 'DGC-HG2W5K',
		setup: 'admin_assisted',
		createdBy: 'usr_marcus',
		createdAt: SEEDED_AT
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
		['lightning_strike_last_epoch', 'Last strike time', 's', 'Unix time of the most recent strike.']
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
			source: 'weather_station'
		}))
	),
	{
		id: 'rain_gauge_daily_accumulation',
		label: 'Manual rain gauge, daily',
		unit: 'mm',
		definition: 'Daily accumulation read from a manual rain gauge.',
		domain: 'RAIN',
		source: 'manual'
	}
];

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
			]
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
		placeName: 'Hargeisa Secondary School',
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
	lightning_strike_last_epoch: 1715421128,
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
		id: `rdg_${deviceId.slice(4)}_${observedAt.slice(0, 10)}`,
		deviceId,
		labId: lab.id,
		classroomId: classroom.id,
		observedAt,
		receivedAt: observedAt,
		values,
		location: gps ?? { ...lab.location },
		locationSource: gps ? 'device' : 'lab',
		samSourceId: `${classroom.code}:${device.identifier}`
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

export function createSeed(): Omit<MockDbState, 'version'> {
	// Deep copy so a reset never shares objects with mutated state.
	return structuredClone({
		users,
		googleAccounts,
		googleLinks: { 'g-sarah': 'usr_sarah', 'g-marcus': 'usr_marcus', 'g-tesfaye': 'usr_tesfaye' },
		challenges: [],
		session: null,
		parameters,
		readings,
		classrooms,
		memberships,
		invitations,
		labs,
		devices,
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
