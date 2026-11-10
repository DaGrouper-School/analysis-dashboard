import type { DomainCompleteness, Reading } from '#lib/domain/types.ts';

type Record_ = Pick<Reading, 'observedAt' | 'values'>;

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

interface Expectation {
	parameterId: string;
	/** From the classroom's protocol: 24 is hourly, 1 daily, 1/7 weekly. */
	readingsPerDay: number;
}

/** The slot a reading counts for: an hour, a day or a week since the start of the month. */
function slotOf(time: number, start: number, perDay: number) {
	const size = perDay >= 1 ? DAY / Math.round(perDay) : 7 * DAY;
	return Math.floor((time - start) / size);
}

/**
 * How complete a month is (AP-22): for each domain, the protocol's slots that have a reading, out
 * of the slots expected from the first of the month through `day`. A slot with several readings
 * counts once, so extra readings never hide a gap. Each day is complete, partial or missing for
 * the daily and faster values; a domain measured only weekly has no days.
 */
export function monthCompleteness(
	records: Record_[],
	day: string,
	expectations: Expectation[],
	domainOf: (parameterId: string) => DomainCompleteness['domain']
): { month: string; domains: DomainCompleteness[] } {
	const month = day.slice(0, 7);
	const start = Date.parse(`${month}-01T00:00:00.000Z`);
	const end = Date.parse(`${day}T00:00:00.000Z`) + DAY;
	const dayCount = Math.round((end - start) / DAY);
	const days = Array.from({ length: dayCount }, (_, i) =>
		new Date(start + i * DAY).toISOString().slice(0, 10)
	);

	// The fastest rate any site asks for, per value.
	const rates = new Map<string, number>();
	for (const e of expectations) {
		rates.set(e.parameterId, Math.max(rates.get(e.parameterId) ?? 0, e.readingsPerDay));
	}

	const slots = new Map<string, Set<number>>();
	for (const record of records) {
		const time = Date.parse(record.observedAt);
		if (time < start || time >= end) continue;
		for (const [parameterId, perDay] of rates) {
			const value = record.values[parameterId];
			if (value === undefined || value === null || value === '') continue;
			const set = slots.get(parameterId) ?? new Set<number>();
			set.add(slotOf(time, start, perDay));
			slots.set(parameterId, set);
		}
	}

	const groups = new Map<string, { domain: DomainCompleteness['domain']; ids: string[] }>();
	for (const parameterId of rates.keys()) {
		const domain = domainOf(parameterId);
		const key = domain?.id ?? '';
		const group = groups.get(key) ?? { domain, ids: [] };
		group.ids.push(parameterId);
		groups.set(key, group);
	}

	const domains = [...groups.values()].map(({ domain, ids }) => {
		let received = 0;
		let expected = 0;
		for (const id of ids) {
			const perDay = rates.get(id)!;
			const total = perDay >= 1 ? dayCount * Math.round(perDay) : Math.ceil(dayCount / 7);
			expected += total;
			received += Math.min(slots.get(id)?.size ?? 0, total);
		}
		const daily = ids.filter((id) => rates.get(id)! >= 1);
		const dayStatus = daily.length
			? days.map((d, i) => {
					let want = 0;
					let got = 0;
					for (const id of daily) {
						const perDay = Math.round(rates.get(id)!);
						want += perDay;
						const set = slots.get(id);
						for (let s = i * perDay; s < (i + 1) * perDay; s++) if (set?.has(s)) got++;
					}
					const status = got === 0 ? 'missing' : got < want ? 'partial' : 'complete';
					return { day: d, status: status as DomainCompleteness['days'][number]['status'] };
				})
			: [];
		return { domain, received, expected, days: dayStatus };
	});
	return { month, domains };
}
