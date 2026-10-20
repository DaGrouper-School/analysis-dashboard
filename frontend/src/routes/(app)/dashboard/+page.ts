import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	const [mine, all, parameters] = await Promise.all([
		services.labs.listMyDevices(),
		services.registry.listDevices(),
		services.registry.listParameters()
	]);
	// Prefer a weather station registered to one of the user's labs, else one in their country.
	const isWeather = (d: { type: string }) => d.type === 'weather_station';
	const station =
		mine.find(isWeather) ??
		all.find((d) => isWeather(d) && d.country === user.address?.country) ??
		all.find(isWeather);
	const snapshot = station ? await services.registry.getLatestSnapshot(station.id) : null;
	return {
		station,
		snapshot,
		parameters,
		isOwnStation: Boolean(station && mine.includes(station))
	};
};
