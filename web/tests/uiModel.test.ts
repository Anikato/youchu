import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { returnStatus, searchLocations, treeRows } from '../src/uiModel.ts';

test('归位状态区分整件、部分配件和已完成事项', () => {
  assert.equal(returnStatus([]), null);
  assert.equal(returnStatus([{completed_at:'2026-10-04',part_note:null}]), null);
  assert.equal(returnStatus([{completed_at:null,part_note:'充电器'}]), '部分待归位');
  assert.equal(returnStatus([{completed_at:null,part_note:null}]), '待归位');
});

test('位置按路径搜索，精确编号优先', () => {
  const rows = [
    {id:1,name:'B101 配件',code:'B102',path:[{name:'厨房',code:null}]},
    {id:2,name:'盒子',code:'B101',path:[{name:'厨房',code:null}]},
  ];
  assert.deepEqual(searchLocations('b101', rows).map(r=>r.id), [2,1]);
  assert.equal(searchLocations('厨房', rows).length, 2);
  assert.equal(searchLocations('不存在', rows).length, 0);
});

test('展开树不依赖接口 id 顺序，折叠隐藏后代', () => {
  const rows = [{id:3,parent_id:2},{id:1,parent_id:null},{id:2,parent_id:1}];
  assert.deepEqual(treeRows(rows,new Set()).map(r=>[r.node.id,r.depth]), [[1,0]]);
  assert.deepEqual(treeRows(rows,new Set([1,2])).map(r=>[r.node.id,r.depth]), [[1,0],[2,1],[3,2]]);
});
