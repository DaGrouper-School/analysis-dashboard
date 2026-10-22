import type { DeviceType, Parameter, Reading } from '#lib/domain/types.ts';

/**
 * Plausible values for a simulated device upload. Real readings come from the device itself;
 * this only exists so the mock can demonstrate connecting one.
 */
export function sampleValues(
	type: DeviceType,
	parameterIds: string[] = [],
	parameters: Pick<Parameter, 'id' | 'valueType' | 'options'>[] = []
): Reading['values'] {
	const jitter = (base: number, spread: number) =>
		Math.round((base + (Math.random() - 0.5) * spread) * 10) / 10;
	switch (type) {
		case 'weather_station':
			return {
				air_temperature: jitter(24, 6),
				relative_humidity: Math.round(jitter(60, 30)),
				wind_avg: jitter(3.5, 3),
				precip_accum_local_day: Math.max(0, jitter(1, 2)),
				uv: jitter(5, 4)
			};
		case 'rain_gauge':
			return { rain_gauge_daily_accumulation: Math.max(0, jitter(3, 6)) };
		case 'soil_sensor':
			return { soil_moisture: Math.round(jitter(30, 20)), soil_temperature: jitter(19, 4) };
		case 'water_probe':
			return { water_temperature: jitter(21, 4), water_ph: jitter(7.2, 1) };
		case 'air_quality_sensor':
			return { pm2_5: Math.round(jitter(18, 20)), pm10: Math.round(jitter(30, 30)) };
		default:
			// A type a platform admin registered: something for each value it is expected to send.
			return parameterIds.length
				? Object.fromEntries(
						parameterIds.map((id, i) => {
							// Each value as the registry has it: an answer for text, now for a time.
							const p = parameters.find((q) => q.id === id);
							if (p?.valueType === 'text') {
								const options = p.options ?? [];
								return [id, options[Math.floor(Math.random() * options.length)] ?? 'Seen'];
							}
							if (p?.valueType === 'time') return [id, Math.round(Date.now() / 1000)];
							return [id, jitter(10 + i * 5, 6)];
						})
					)
				: { value: jitter(1, 1) };
	}
}
