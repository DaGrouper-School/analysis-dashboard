import type { ClassroomRole, DeviceType, SiteSetting } from '#lib/domain/types.ts';

export const CLASSROOM_ROLE_LABELS: Record<ClassroomRole, string> = {
	institution_head: 'Institution head',
	facilitator: 'Facilitator',
	community_scientist: 'Community scientist'
};

export const DEVICE_TYPE_LABELS: Record<DeviceType, string> = {
	weather_station: 'Weather station',
	soil_sensor: 'Soil sensor',
	rain_gauge: 'Rain gauge',
	water_probe: 'Water probe',
	air_quality_sensor: 'Air quality sensor',
	other: 'Other'
};

export const SITE_SETTING_LABELS: Record<SiteSetting, string> = {
	coastal: 'Coastal',
	urban: 'Urban',
	rural: 'Rural or agricultural',
	highland: 'Highland',
	forest: 'Forest',
	other: 'Other'
};
