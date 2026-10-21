import { domainTabs, isTimeRange } from '#lib/domains.ts';
import { services } from '#lib/services/index.ts';
import { selectSensor } from '#lib/sensor-selection.ts';
import type { PageLoad } from './$types';

/**
 * AP-10: one sensor's readings by environmental domain. Only sensors in the user's own
 * classrooms can be opened; the pick falls back to one of theirs otherwise.
 */
export const load: PageLoad = async ({ url, parent }) => {
	const [selection, parameters, { domains }] = await Promise.all([
		selectSensor(url),
		services.registry.listParameters(),
		parent()
	]);
	const { device, classroom } = selection;
	const [readings, alertRules] = await Promise.all([
		// A month of readings covers every time range.
		device ? services.labs.deviceReadings(device.id, 1000) : [],
		classroom ? services.classrooms.listAlerts(classroom.id) : null
	]);
	const tabs = domainTabs(readings, parameters, domains);
	const asked = url.searchParams.get('range');
	return {
		...selection,
		parameters,
		readings,
		alertRules: alertRules?.rules ?? [],
		tabs,
		tab: tabs.find((t) => t.id === url.searchParams.get('domain')) ?? tabs[0] ?? null,
		range: isTimeRange(asked) ? asked : '7d'
	};
};
