import { describe, expect, test } from 'vitest';
import { contrast, luminance, parseColor, type Rgb } from './contrast';

const BLACK: Rgb = [0, 0, 0];
const WHITE: Rgb = [1, 1, 1];
const MID_GREY: Rgb = [0.5, 0.5, 0.5];

describe('parseColor', () => {
	test('reads rgb() with 0–255 channels', () => {
		expect(parseColor('rgb(255, 0, 51)')).toEqual([1, 0, 0.2]);
	});

	test('reads an opaque rgba()', () => {
		expect(parseColor('rgba(0, 255, 0, 1)')).toEqual([0, 1, 0]);
	});

	test('reads color(srgb) with 0–1 channels, what a color-mix() computes to', () => {
		expect(parseColor('color(srgb 0.25 0.5 1)')).toEqual([0.25, 0.5, 1]);
	});

	test('refuses a translucent colour', () => {
		expect(() => parseColor('rgba(0, 0, 0, 0.5)')).toThrow(/translucent/);
		expect(() => parseColor('color(srgb 0 0 0 / 0.5)')).toThrow(/translucent/);
	});

	test('refuses anything else', () => {
		expect(() => parseColor('transparent')).toThrow(/not an sRGB colour/);
	});
});

describe('contrast', () => {
	test('luminance runs from black to white', () => {
		expect(luminance(BLACK)).toBe(0);
		expect(luminance(WHITE)).toBeCloseTo(1);
	});

	test('black on white is 21:1, either way round', () => {
		expect(contrast(BLACK, WHITE)).toBeCloseTo(21);
		expect(contrast(WHITE, BLACK)).toBeCloseTo(21);
	});

	test('a colour against itself is 1:1', () => {
		expect(contrast(MID_GREY, MID_GREY)).toBe(1);
	});

	test('mid grey on white is below the text threshold', () => {
		// A channel of 0.5 is about 3.98:1 on white through the sRGB curve, and
		// about 3.92:1 through a plain 2.2 power (a channel of 128/255 is 3.95:1).
		expect(contrast(MID_GREY, WHITE)).toBeCloseTo(3.98, 2);
	});

	test('luminance weights each channel', () => {
		expect(luminance([1, 0, 0])).toBeCloseTo(0.2126, 4);
		expect(luminance([0, 1, 0])).toBeCloseTo(0.7152, 4);
		expect(luminance([0, 0, 1])).toBeCloseTo(0.0722, 4);
	});

	test('a dark channel is on the linear segment of the sRGB curve', () => {
		// 0.02 / 12.92; the power branch would give about 0.001755.
		expect(luminance([0.02, 0.02, 0.02])).toBeCloseTo(0.001548, 6);
	});
});
