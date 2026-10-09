import {test} from 'node:test';import assert from 'node:assert/strict';import {fixture} from './fixture.mjs';
test('original response lacks requested navigation callback receipt (expected coverage failure)',async()=>{const value=await fixture('original').promise;assert.equal(value?.status,'matched');});
