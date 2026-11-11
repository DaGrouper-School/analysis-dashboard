import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });
const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);
const IMAGE = 'data:image/jpeg;base64,/9j/4AAQSkZJRg==';

beforeEach(() => resetDb());

describe('institution photo (AP-43)', () => {
	it('is set by the institution head and shows in the overview', async () => {
		await signInAs('g-tesfaye');
		const info = await services.classrooms.setInstitutionPhoto('cls_trinity', {
			image: IMAGE,
			alt: ' The school garden '
		});
		expect(info).toMatchObject({ photoUrl: IMAGE, photoAlt: 'The school garden' });
		const station = await services.publicSite.station('cls_trinity');
		expect(station.institution?.photoUrl).toBe(IMAGE);
	});

	it('can be removed', async () => {
		await signInAs('g-tesfaye');
		await services.classrooms.setInstitutionPhoto('cls_trinity', { image: IMAGE, alt: 'Garden' });
		const info = await services.classrooms.setInstitutionPhoto('cls_trinity', {
			image: null,
			alt: ''
		});
		expect(info.photoUrl).toBeNull();
	});

	it('needs an image and what it shows', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.classrooms.setInstitutionPhoto('cls_trinity', { image: 'not an image', alt: '' })
		).rejects.toMatchObject({ code: 'invalid_input' });
	});

	it('is only for the institution head', async () => {
		await signInAs('g-marcus');
		await expect(
			services.classrooms.setInstitutionPhoto('cls_trinity', { image: IMAGE, alt: 'Garden' })
		).rejects.toMatchObject({ code: 'forbidden' });
	});
});
