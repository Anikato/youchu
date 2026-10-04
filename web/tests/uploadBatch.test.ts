import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { uploadBatch } from '../src/uploadBatch.ts';

test('部分上传失败保留原文件，重试不重复成功文件', async () => {
  const files = ['a','b','c'];
  const uploaded: string[] = [];
  const failed = await uploadBatch(files, async file => { if(file==='b')throw new Error('离线');uploaded.push(file); });
  assert.deepEqual(failed.map(row=>row.file), ['b']);
  await uploadBatch(failed.map(row=>row.file), async file => {uploaded.push(file);});
  assert.deepEqual(uploaded, ['a','c','b']);
});
