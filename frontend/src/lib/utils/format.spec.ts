import { describe, expect, it } from 'vitest';
import { givenName, initials } from '#lib/utils/format.ts';

describe('initials', () => {
	it('takes up to two capitalised words', () => {
		expect(initials('Bay Street School')).toBe('BS');
		expect(initials('Trinity School, Addis Ababa')).toBe('TS');
	});
});

describe('givenName', () => {
	it('skips a title', () => {
		expect(givenName('Dr. Marcus L.')).toBe('Marcus');
		expect(givenName('Tesfaye G.')).toBe('Tesfaye');
	});
});
