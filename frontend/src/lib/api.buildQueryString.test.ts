import { describe, it, expect } from 'vitest';
import { buildQueryString } from './api';

describe('buildQueryString', () => {
	it('returns empty string for no params', () => {
		expect(buildQueryString({})).toBe('');
	});

	it('returns empty string when all values are undefined', () => {
		expect(buildQueryString({ a: undefined, b: null, c: '' })).toBe('');
	});

	it('builds query string with single param', () => {
		expect(buildQueryString({ page: 1 })).toBe('?page=1');
	});

	it('builds query string with multiple params', () => {
		const result = buildQueryString({ page: 1, per_page: 20, sort: 'name' });
		expect(result).toBe('?page=1&per_page=20&sort=name');
	});

	it('filters out undefined and null values', () => {
		const result = buildQueryString({
			page: 1,
			status: undefined,
			search: null,
			sort: 'name'
		});
		expect(result).toBe('?page=1&sort=name');
	});

	it('filters out empty strings', () => {
		const result = buildQueryString({ page: 1, search: '' });
		expect(result).toBe('?page=1');
	});

	it('converts numbers to strings', () => {
		const result = buildQueryString({ limit: 50, offset: 100 });
		expect(result).toBe('?limit=50&offset=100');
	});

	it('converts booleans to strings', () => {
		const result = buildQueryString({ allow_any: true });
		expect(result).toBe('?allow_any=true');
	});

	it('keeps zero as a valid value', () => {
		const result = buildQueryString({ offset: 0 });
		expect(result).toBe('?offset=0');
	});

	it('includes leading ? prefix', () => {
		const result = buildQueryString({ a: 'b' });
		expect(result.startsWith('?')).toBe(true);
	});
});
