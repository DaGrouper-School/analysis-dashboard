import { formatValue } from '#lib/key-values.ts';
import { rangeText } from '#lib/observations.ts';
import type {
	AlertLevel,
	AlertRule,
	AlertRuleInput,
	DeviceAlert,
	EffectiveAlertRule,
	Parameter,
	Reading
} from './types';

/**
 * Alert levels (AP-08). The registry holds platform defaults; a classroom's own rules for a
 * parameter replace the defaults for that parameter. The backend evaluates them on each reading,
 * and the dashboard draws the same levels as its thresholds. Nothing here is hard-coded: the
 * rules are always passed in.
 */

export const ALERT_LEVELS: AlertLevel[] = ['warning', 'caution'];
export const ALERT_DIRECTIONS = ['above', 'below'] as const;
export const ALERT_TITLE_MAX = 60;

export const LEVEL_LABELS: Record<AlertLevel, string> = { warning: 'Warning', caution: 'Caution' };
export const DIRECTION_LABELS: Record<AlertRule['direction'], string> = {
	above: 'at or above',
	below: 'at or below'
};

const SEVERITY: Record<AlertLevel, number> = { warning: 2, caution: 1 };

/** A stored classroom rule: whose it is and who last changed it. */
export interface ClassroomRuleRecord extends AlertRule {
	classroomId: string;
	updatedBy: string;
	updatedAt: string;
}

/**
 * The rules in force for one classroom: its own rules for the parameters it has set, the
 * defaults for the rest. `nameOf` turns a user ID into the name shown beside a rule.
 */
export function effectiveRules(
	defaults: AlertRule[],
	own: ClassroomRuleRecord[],
	nameOf: (userId: string) => string = (id) => id
): EffectiveAlertRule[] {
	const set = new Set(own.map((r) => r.parameterId));
	return [
		...own.map((rule) => ({
			id: rule.id,
			parameterId: rule.parameterId,
			direction: rule.direction,
			value: rule.value,
			level: rule.level,
			title: rule.title,
			enabled: rule.enabled,
			source: 'classroom' as const,
			updatedBy: nameOf(rule.updatedBy),
			updatedAt: rule.updatedAt
		})),
		...defaults
			.filter((r) => !set.has(r.parameterId))
			.map((rule) => ({
				...rule,
				source: 'default' as const,
				updatedBy: null,
				updatedAt: null
			}))
	].sort(bySeverity);
}

/** Most severe first; within a level, the one that trips first. */
function bySeverity(a: AlertRule, b: AlertRule) {
	return (
		SEVERITY[b.level] - SEVERITY[a.level] ||
		(a.direction === 'above' ? a.value - b.value : b.value - a.value)
	);
}

export function crosses(rule: Pick<AlertRule, 'direction' | 'value'>, value: number) {
	return rule.direction === 'above' ? value >= rule.value : value <= rule.value;
}

/** The enabled rule a value trips, the most severe one, or null. */
function tripped<R extends AlertRule>(rules: R[], parameterId: string, value: number) {
	return (
		rules
			.filter((r) => r.enabled && r.parameterId === parameterId && crosses(r, value))
			.sort(bySeverity)[0] ?? null
	);
}

/** `Warning at or above 8`, for a chip. */
export function ruleText(rule: Pick<AlertRule, 'level' | 'direction' | 'value'>, unit = '') {
	return `${LEVEL_LABELS[rule.level]} ${DIRECTION_LABELS[rule.direction]} ${formatValue(rule.value, unit)}`;
}

/**
 * One alert per parameter the reading trips, most severe first. The message states what was
 * measured against which level; it gives no advice.
 */
export function alertsFor(
	reading: Pick<Reading, 'values'>,
	rules: EffectiveAlertRule[],
	parameters: Parameter[]
): DeviceAlert[] {
	const byId = new Map(parameters.map((p) => [p.id, p]));
	const alerts: [EffectiveAlertRule, DeviceAlert][] = [];
	for (const parameterId of new Set(rules.map((r) => r.parameterId))) {
		const value = reading.values[parameterId];
		if (typeof value !== 'number') continue;
		const rule = tripped(rules, parameterId, value);
		if (!rule) continue;
		const parameter = byId.get(parameterId);
		const unit = parameter?.unit ?? '';
		const whose = rule.source === 'classroom' ? "this classroom's" : 'the default';
		alerts.push([
			rule,
			{
				parameterId,
				level: rule.level,
				title: rule.title,
				message: `${parameter?.label ?? parameterId} ${formatValue(value, unit)} is ${DIRECTION_LABELS[rule.direction]} ${whose} ${rule.level} level of ${formatValue(rule.value, unit)}.`
			}
		]);
	}
	return alerts.sort(([a], [b]) => bySeverity(a, b)).map(([, alert]) => alert);
}

export type ThresholdLevel = 'normal' | AlertLevel;

/**
 * Where a value sits against a classroom's levels, for a badge beside it. Null when the
 * parameter has no enabled level.
 */
export function thresholdStatus(
	rules: AlertRule[],
	parameterId: string,
	value: number | string | undefined
): { level: ThresholdLevel; title: string } | null {
	if (typeof value !== 'number') return null;
	if (!rules.some((r) => r.enabled && r.parameterId === parameterId)) return null;
	const hit = tripped(rules, parameterId, value);
	return hit ? { level: hit.level, title: hit.title } : { level: 'normal', title: 'Normal' };
}

/** The enabled levels of one parameter, to draw on its chart. */
export function thresholdsFor<R extends AlertRule>(rules: R[], parameterId: string): R[] {
	return rules.filter((r) => r.enabled && r.parameterId === parameterId).sort(bySeverity);
}

/** Checks a rule against the registry. Errors are keyed by field. */
export function checkAlertRule(
	input: AlertRuleInput,
	parameters: Parameter[]
): Record<string, string> {
	const errors: Record<string, string> = {};
	const parameter = parameters.find((p) => p.id === input.parameterId);
	if (!parameter) errors.parameterId = 'Pick a value from the registry.';
	else if ((parameter.valueType ?? 'number') !== 'number') {
		errors.parameterId = `${parameter.label} is not a number, so it cannot have a level.`;
	}
	if (!ALERT_DIRECTIONS.includes(input.direction)) errors.direction = 'Pick above or below.';
	if (!ALERT_LEVELS.includes(input.level)) errors.level = 'Pick caution or warning.';
	if (typeof input.value !== 'number' || !Number.isFinite(input.value)) {
		errors.value = 'The level must be a number.';
	} else if (
		parameter &&
		((parameter.min !== undefined && input.value < parameter.min) ||
			(parameter.max !== undefined && input.value > parameter.max))
	) {
		errors.value = `${parameter.label} should be ${rangeText(parameter)}.`;
	}
	const title = input.title?.trim() ?? '';
	if (!title) errors.title = 'Give the alert a short name.';
	else if (title.length > ALERT_TITLE_MAX) {
		errors.title = `Keep the name to ${ALERT_TITLE_MAX} characters.`;
	}
	return errors;
}
