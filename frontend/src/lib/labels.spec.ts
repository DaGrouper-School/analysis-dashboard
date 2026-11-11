import { describe, expect, it } from 'vitest';
import { withArticle } from '#lib/labels.ts';

describe('withArticle', () => {
	it('picks a or an by the first letter', () => {
		expect(withArticle('researcher')).toBe('a researcher');
		expect(withArticle('institution head')).toBe('an institution head');
	});
});
