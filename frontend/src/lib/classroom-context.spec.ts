import { beforeEach, describe, expect, it, vi } from 'vitest';
import { contextClassroom, rememberClassroom } from './classroom-context';

const classrooms = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
const at = (query = '') => new URL(`https://example.test/dashboard${query}`);

beforeEach(() => {
	const store = new Map<string, string>();
	vi.stubGlobal('localStorage', {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => void store.set(key, value)
	});
});

describe('classroom context', () => {
	it('takes the address first, then the last pick, then the fallback, then the first', () => {
		expect(contextClassroom(classrooms, at())?.id).toBe('a');
		expect(contextClassroom(classrooms, at(), 'c')?.id).toBe('c');
		rememberClassroom('b');
		expect(contextClassroom(classrooms, at(), 'c')?.id).toBe('b');
		expect(contextClassroom(classrooms, at('?classroom=c'))?.id).toBe('c');
	});

	it('ignores classrooms the user cannot see', () => {
		rememberClassroom('gone');
		expect(contextClassroom(classrooms, at('?classroom=nope'))?.id).toBe('a');
		expect(contextClassroom([], at())).toBeNull();
	});
});
