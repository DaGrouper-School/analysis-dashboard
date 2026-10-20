import type {
	EnvironmentalDomain,
	Parameter,
	Station,
	StationSnapshot,
	User
} from '#lib/domain/types.ts';
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
			street: 'Kebele 04',
			city: 'Shashamane',
			state: 'Oromia',
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
		institutionName: 'Trinity School, Shashamane',
		code: 'DGC-7K3Q9M',
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

const stations: Station[] = [
	{
		id: 'stn_shashamane',
		name: 'Shashamane Weather Station',
		type: 'weather_station',
		siteName: 'Trinity lab',
		country: 'Ethiopia',
		lat: 7.2,
		lng: 38.6
	},
	{
		id: 'stn_shashamane_gauge',
		name: 'Shashamane Rain Gauge',
		type: 'rain_gauge',
		siteName: 'Trinity lab',
		country: 'Ethiopia',
		lat: 7.2,
		lng: 38.6
	},
	{
		id: 'stn_nassau',
		name: 'Nassau Coastal Station',
		type: 'weather_station',
		siteName: 'Bay Street node',
		country: 'Bahamas',
		lat: 25.06,
		lng: -77.34
	},
	{
		id: 'stn_hargeisa',
		name: 'Hargeisa Station',
		type: 'weather_station',
		siteName: 'Hargeisa school',
		country: 'Somalia',
		lat: 9.56,
		lng: 44.06
	}
];

const sample: StationSnapshot['values'] = {
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

const snapshots: StationSnapshot[] = [
	{ stationId: 'stn_shashamane', observedAt: '2026-10-20T08:00:00.000Z', values: sample },
	{
		stationId: 'stn_nassau',
		observedAt: '2026-10-20T08:00:00.000Z',
		values: {
			...sample,
			air_temperature: 29.4,
			relative_humidity: 78,
			wind_avg: 6.1,
			wind_gust: 9.2,
			uv: 7.8
		}
	},
	{
		stationId: 'stn_hargeisa',
		observedAt: '2026-10-20T08:00:00.000Z',
		values: { ...sample, air_temperature: 24.1, relative_humidity: 41, wind_avg: 4.4, uv: 9.1 }
	}
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
		stations,
		snapshots,
		classrooms,
		memberships,
		invitations,
		samSources: classrooms.map((c) => ({
			sourceId: c.code,
			kind: 'classroom' as const,
			registeredAt: c.createdAt
		}))
	});
}
