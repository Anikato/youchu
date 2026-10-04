export function locationDeletable(loc: { child_count: number; direct_item_count: number }): boolean {
  return loc.child_count === 0 && loc.direct_item_count === 0;
}

export function toggleSelected(current: Set<number>, id: number): Set<number> {
  const next = new Set(current);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

export function pageAllSelected(ids: number[], selected: Set<number>): boolean {
  return ids.length > 0 && ids.every((id) => selected.has(id));
}

export function selectPageIds(ids: number[], selected: Set<number>): Set<number> {
  if (pageAllSelected(ids, selected)) {
    const next = new Set(selected);
    for (const id of ids) next.delete(id);
    return next;
  }
  const next = new Set(selected);
  for (const id of ids) next.add(id);
  return next;
}
