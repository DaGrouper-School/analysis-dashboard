/** Rows a table that pages in the browser shows at a time. */
export const TABLE_PAGE_SIZE = 10;

/**
 * Pages a list that is already loaded, for a table with `Pagination` under it. The page stays
 * in range when the list shrinks, such as after a filter or a shorter date range.
 */
export class Pager<T> {
	page = $state(1);
	readonly #rows: () => T[];
	readonly pageSize: number;

	constructor(rows: () => T[], pageSize = TABLE_PAGE_SIZE) {
		this.#rows = rows;
		this.pageSize = pageSize;
	}

	get total() {
		return this.#rows().length;
	}

	/** The page shown, from 1. */
	get current() {
		return Math.min(this.page, Math.max(1, Math.ceil(this.total / this.pageSize)));
	}

	/** The rows on the page shown. */
	get rows() {
		const from = (this.current - 1) * this.pageSize;
		return this.#rows().slice(from, from + this.pageSize);
	}
}
