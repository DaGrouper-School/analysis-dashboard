import type { Parameter, Reading } from '#lib/domain/types.ts';

/**
 * A simulated device's next reading, for the demo sync only: real readings come from the device
 * itself. Nothing here is made up per device type. Each value the Registry expects from the
 * device moves a little from the device's last reading, or starts in the middle of the range the
 * Registry accepts. Text values pick one of the Registry's answers; times are now. A value with
 * neither a last reading nor an accepted range is left out, as there is nothing to go on.
 */
export function sampleValues(
	parameterIds: string[],
	parameters: Pick<Parameter, 'id' | 'valueType' | 'options' | 'min' | 'max'>[],
	last: Reading['values'] = {},
	random: () => number = Math.random
): Reading['values'] {
	const values: Reading['values'] = {};
	for (const id of parameterIds) {
		const p = parameters.find((q) => q.id === id);
		if (!p) continue;
		if (p.valueType === 'text') {
			const options = p.options ?? [];
			if (options.length) values[id] = options[Math.floor(random() * options.length)];
			continue;
		}
		if (p.valueType === 'time') {
			values[id] = Math.round(Date.now() / 1000);
			continue;
		}
		const previous = typeof last[id] === 'number' ? last[id] : undefined;
		const ranged = p.min !== undefined && p.max !== undefined;
		if (previous === undefined && !ranged) continue;
		const base = previous ?? (p.min! + p.max!) / 2;
		// A step of a twentieth of the range, or of the value itself when there is no range.
		const spread = ranged ? (p.max! - p.min!) / 20 : Math.max(Math.abs(base) / 10, 0.5);
		let value = base + (random() - 0.5) * spread;
		if (ranged) value = Math.min(p.max!, Math.max(p.min!, value));
		values[id] = Math.round(value * 10) / 10;
	}
	return values;
}
