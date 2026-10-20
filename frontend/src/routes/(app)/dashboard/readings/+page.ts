import { services } from '#lib/services/index.ts';
import { selectSensor } from '#lib/sensor-selection.ts';
import type { PageLoad } from './$types';

/** Every recent reading of the dashboard's picked sensor, with all the values it sent. */
export const load: PageLoad = async ({ url }) => {
	const [selection, parameters] = await Promise.all([
		selectSensor(url),
		services.registry.listParameters()
	]);
	const { device } = selection;
	return {
		...selection,
		readings: device ? await services.labs.deviceReadings(device.id, 100) : [],
		parameters
	};
};
