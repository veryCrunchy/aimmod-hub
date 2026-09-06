import test from 'node:test';
import assert from 'node:assert/strict';
import { spinnerPreviewState, sliderPreviewPosition } from '../src/lib/skinPreview';
test('displayed RPM matches the derivative of spinner rotation', () => {
 for (const t of [1,2.5,4,6]) {
  const speed = (spinnerPreviewState(t + .0001).rotation - spinnerPreviewState(t).rotation) / .0001 * 60 / (Math.PI * 2);
  assert.ok(Math.abs(speed - spinnerPreviewState(t).rpm) < 1.1);
 }
});
test('spinner progress controls scale, completion and shrinking approach ring', () => {
 const start = spinnerPreviewState(0), middle = spinnerPreviewState(3), end = spinnerPreviewState(7);
 assert.equal(start.progress, 0); assert.equal(start.scale, .8); assert.equal(start.cleared, false);
 assert.ok(middle.scale > start.scale && middle.approachScale < start.approachScale);
 assert.equal(end.progress, 1); assert.equal(end.scale, 1); assert.equal(end.cleared, true);
 assert.deepEqual(spinnerPreviewState(8), start);
});
test('slider cursor follows the same bezier and reaches both ends', () => {
 assert.deepEqual(sliderPreviewPosition(0), [285,280]);
 assert.deepEqual(sliderPreviewPosition(1), [560,235]);
 assert.deepEqual(sliderPreviewPosition(.5), [418.75,268.75]);
});
