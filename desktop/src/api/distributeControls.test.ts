import { afterEach, expect, it, vi } from 'vitest';
import { distributeControlsApi } from './distributeControls';
import { productApi } from './product';

afterEach(() => vi.restoreAllMocks());

it('normalizes clean serialized preview and initial/no-op comparison collections', async () => {
  const preview = JSON.parse('{"sourceSHA256":"abc","workbook":{"sheets":[{"name":"Controls","headers":[],"rows":[]}],"issues":null},"suggestedColumns":{}}');
  vi.spyOn(productApi, 'uploadJSON').mockResolvedValue(preview);
  const file = new File(['xlsx'], 'controls.xlsx');
  expect((await distributeControlsApi.preview(file)).workbook.issues).toEqual([]);
  const initial = JSON.parse('{"expectedRevision":0,"sourceSHA256":"abc","sheet":"Controls","columns":{},"added":["SEC-1"],"changed":null,"removed":null,"controlCount":1,"diffDigest":"digest"}');
  const noOp = JSON.parse('{"expectedRevision":1,"sourceSHA256":"abc","sheet":"Controls","columns":{},"added":null,"changed":null,"removed":null,"controlCount":1,"diffDigest":"digest"}');
  vi.spyOn(productApi, 'request').mockResolvedValueOnce(initial).mockResolvedValueOnce(noOp);
  const input = { expectedRevision: 0, sourceSHA256: 'abc', sheet: 'Controls', columns: {}, workbookBase64: 'eGxzeA==' };
  expect(await distributeControlsApi.compare(input)).toMatchObject({ added: ['SEC-1'], changed: [], removed: [] });
  expect(await distributeControlsApi.compare({...input,expectedRevision:1})).toMatchObject({ added: [], changed: [], removed: [] });
});
