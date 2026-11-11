import { describe, expect, it } from 'vitest';
import { initials } from '#lib/utils/format.ts';

describe('initials', () => {
	it('takes up to two capitalised words', () => {
		expect(initials('Bay Street School')).toBe('BS');
		expect(initials('Trinity School, Addis Ababa')).toBe('TS');
	});
});
