import { describe, expect, it } from 'vitest';
import type { DeviceTypeDefinition, Parameter } from '#lib/domain/types.ts';
import {
	blankRow,
	blankType,
	domainOptions,
	fillRow,
	firstStepWithError,
	NEW_DOMAIN,
	typeForm,
	typeInput
} from './type-form.ts';

const soil = {
	id: 'soil_ph',
	label: 'Soil pH',
	unit: '',
	valueType: 'number',
	domain: 'SOIL',
	source: 'device'
} as Parameter;
const parameters = new Map([[soil.id, soil]]);

describe('domainOptions', () => {
	it('offers no domain first and a new domain last', () => {
		const options = domainOptions([{ id: 'SOIL', label: 'Soil', icon: 'tag', description: '' }]);
		expect(options.map((o) => o.value)).toEqual(['', 'SOIL', NEW_DOMAIN]);
	});
});

describe('typeForm', () => {
	it('fills a row per value, with its tile and card choices', () => {
		const type = {
			id: 'probe',
			label: 'Probe',
			description: '',
			color: '#fff',
			icon: 'soil',
			parameterIds: ['soil_ph', 'unknown'],
			tileParameterIds: ['soil_ph'],
			cardParameterIds: []
		} as unknown as DeviceTypeDefinition;
		const form = typeForm(type, parameters);
		expect(form.rows.map((r) => [r.id, r.label, r.domain, r.tile, r.card])).toEqual([
			['soil_ph', 'Soil pH', 'SOIL', true, false],
			['unknown', 'unknown', null, false, false]
		]);
		expect(form.manual).toBe(false);
	});
});

describe('fillRow', () => {
	it("copies a registry value's label, unit and domain", () => {
		const row = blankRow();
		fillRow(row, 'soil_ph ', parameters);
		expect([row.id, row.label, row.domain]).toEqual(['soil_ph ', 'Soil pH', 'SOIL']);
	});

	it('leaves a new ID as typed', () => {
		const row = blankRow();
		fillRow(row, 'new_value', parameters);
		expect([row.id, row.label]).toEqual(['new_value', '']);
	});
});

describe('typeInput', () => {
	it('leaves out blank rows', () => {
		const form = blankType();
		form.label = 'Probe';
		form.rows = [{ ...blankRow(), id: 'soil_ph' }, blankRow()];
		expect(typeInput(form).values.map((v) => v.id)).toEqual(['soil_ph']);
	});
});

describe('firstStepWithError', () => {
	it('picks the earliest step with an error', () => {
		expect(firstStepWithError({ 'values.0.id': 'x', label: 'y' })).toBe(0);
		expect(firstStepWithError({ 'values.0.id': 'x' })).toBe(1);
		expect(firstStepWithError({})).toBeNull();
	});
});
