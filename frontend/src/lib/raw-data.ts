import type {
	DataTerms,
	GeoPoint,
	Parameter,
	RawQuery,
	RawRow,
	RawSortKey
} from '#lib/domain/types.ts';

/** The source ID that stands for every field observation, which has no device. */
export const FIELD_SOURCE = 'field';

/** Rows the raw data table shows at first, and adds with each "Show more". */
export const PAGE_SIZE = 200;

export const SORT_KEYS: RawSortKey[] = ['time', 'parameter', 'source', 'site'];

/** A position rounded as the data's terms require; untouched when they set no limit. */
export function coarsen(point: GeoPoint, decimals: number | null): GeoPoint {
	if (decimals === null) return { ...point };
	const f = 10 ** decimals;
	return { lat: Math.round(point.lat * f) / f, lng: Math.round(point.lng * f) / f };
}

/** The rows that match every filter in the query. Days are compared in UTC. */
export function filterRows(rows: RawRow[], query: RawQuery): RawRow[] {
	return rows.filter((row) => {
		const day = row.observedAt.slice(0, 10);
		return (
			(!query.from || day >= query.from) &&
			(!query.to || day <= query.to) &&
			(!query.parameterId || row.parameterId === query.parameterId) &&
			(!query.sourceId || row.sourceId === query.sourceId) &&
			(!query.labId || row.labId === query.labId)
		);
	});
}

/**
 * Rows in the query's order: newest first by default. Ties fall back to newest first, then to
 * the parameter, so the order is stable whatever the column.
 */
export function sortRows(
	rows: RawRow[],
	query: Pick<RawQuery, 'sort' | 'order'>,
	labelOf: (parameterId: string) => string
): RawRow[] {
	const sort = query.sort ?? 'time';
	const dir = (query.order ?? (sort === 'time' ? 'desc' : 'asc')) === 'asc' ? 1 : -1;
	const key = (row: RawRow) =>
		sort === 'time'
			? row.observedAt
			: sort === 'parameter'
				? labelOf(row.parameterId)
				: sort === 'source'
					? row.sourceName
					: row.siteName;
	return [...rows].sort(
		(a, b) =>
			dir * key(a).localeCompare(key(b)) ||
			b.observedAt.localeCompare(a.observedAt) ||
			labelOf(a.parameterId).localeCompare(labelOf(b.parameterId))
	);
}

/**
 * One CSV field. Quoted when it holds a comma, quote or line break; text that a spreadsheet
 * would run as a formula is prefixed with an apostrophe. Numbers are written as they are.
 */
export function csvField(value: string | number | null): string {
	if (value === null) return '';
	if (typeof value === 'number') return String(value);
	const safe = /^[=+\-@\t\r]/.test(value) && Number.isNaN(Number(value)) ? `'${value}` : value;
	return /[",\r\n]/.test(safe) ? `"${safe.replaceAll('"', '""')}"` : safe;
}

export const CSV_COLUMNS = [
	'observed_at',
	'received_at',
	'parameter_id',
	'parameter',
	'value',
	'unit',
	'source',
	'source_type',
	'site',
	'classroom',
	'latitude',
	'longitude',
	'location_source',
	'recorded_by',
	'licence',
	'licence_url',
	'attribution',
	'restrictions'
] as const;

/**
 * The rows as CSV, one value per line. Every line carries the classroom's licence, attribution
 * and restrictions, so they stay with the data when rows are copied out or merged.
 */
export function toCsv(
	rows: RawRow[],
	classroomName: string,
	terms: DataTerms,
	parameters: ReadonlyMap<string, Pick<Parameter, 'label' | 'unit'>>
): string {
	const restrictions = terms.restrictions.join(' ');
	const lines = rows.map((row) => {
		const param = parameters.get(row.parameterId);
		return [
			row.observedAt,
			row.receivedAt,
			row.parameterId,
			param?.label ?? row.parameterId,
			row.value,
			param?.unit ?? '',
			row.sourceName,
			row.sourceKind,
			row.siteName,
			classroomName,
			row.location.lat,
			row.location.lng,
			row.locationSource,
			row.creditedTo,
			terms.licence,
			terms.licenceUrl,
			terms.attribution,
			restrictions
		]
			.map(csvField)
			.join(',');
	});
	return [CSV_COLUMNS.join(','), ...lines].join('\r\n') + '\r\n';
}

const slug = (text: string) =>
	text
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-|-$/g, '');

/**
 * `dagrouper-trinity-lab-classroom-shashamane-weather-station-20261022-0930-k3f9q2.csv`: the
 * classroom, the source exported (or all sources), when, and the export's ID, so a new export
 * never takes the name of an earlier one on the user's device.
 */
export function csvFilename(
	classroomName: string,
	sourceName: string | null,
	at: Date,
	exportId: string
): string {
	const stamp = at.toISOString().slice(0, 16).replace(/[-:]/g, '').replace('T', '-');
	const source = (sourceName && slug(sourceName)) || 'all-sources';
	return `dagrouper-${slug(classroomName) || 'classroom'}-${source}-${stamp}-${exportId}.csv`;
}

/** The byte order mark that tells Excel the file is UTF-8, so `°C` is not read as `Â°C`. */
export const UTF8_BOM = '\uFEFF';
