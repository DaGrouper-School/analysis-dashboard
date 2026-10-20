import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	const [stations, parameters] = await Promise.all([
		services.registry.listStations(),
		services.registry.listParameters()
	]);
	// Until classrooms link users to stations (sprint 2), pick the nearest by country.
	const weatherStations = stations.filter((s) => s.type === 'weather_station');
	const station =
		weatherStations.find((s) => s.country === user.address?.country) ?? weatherStations[0];
	const snapshot = station ? await services.registry.getLatestSnapshot(station.id) : null;
	return { station, snapshot, parameters, stations: weatherStations };
};
