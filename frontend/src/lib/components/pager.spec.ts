import { describe, expect, it } from 'vitest';
import { Pager } from './pager.svelte.ts';

describe('Pager', () => {
	it('shows one page of rows and stays in range when the list shrinks', () => {
		let rows = Array.from({ length: 25 }, (_, i) => i);
		const pager = new Pager(() => rows, 10);
		expect(pager.rows).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
		pager.page = 3;
		expect(pager.rows).toEqual([20, 21, 22, 23, 24]);
		rows = rows.slice(0, 12);
		expect([pager.current, pager.total]).toEqual([2, 12]);
		expect(pager.rows).toEqual([10, 11]);
	});
});
