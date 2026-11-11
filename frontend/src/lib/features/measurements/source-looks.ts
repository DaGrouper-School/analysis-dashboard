import type { DeviceTypeDefinition, DeviceType } from '#lib/domain/types.ts';
import { deviceTypeColor, deviceTypeIcon } from '#lib/utils/labels.ts';

/** A source's registry icon and colour, keyed by device ID, for the Measurement dropdown. */
export type SourceLooks = Record<string, { icon: string; color: string }>;

/** Each device's look, read from the registry's device types like the device picker's. */
export function sourceLooks(
	devices: { id: string; type: DeviceType }[],
	types: DeviceTypeDefinition[]
): SourceLooks {
	return Object.fromEntries(
		devices.map((d) => [
			d.id,
			{ icon: deviceTypeIcon(types, d.type), color: deviceTypeColor(types, d.type) }
		])
	);
}
