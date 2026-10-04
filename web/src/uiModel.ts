import { filterLocations, type LocationSearchItem } from './locationSearch.ts';

export function returnStatus(tasks: { completed_at: string | null; part_note: string | null }[]): string | null {
  const open = tasks.filter(task => !task.completed_at);
  if (!open.length) return null;
  return open.some(task => !task.part_note?.trim()) ? '待归位' : '部分待归位';
}

export function searchLocations<T extends LocationSearchItem>(query: string, rows: T[]): T[] {
  const q = query.trim().toLowerCase();
  return filterLocations(query, rows).sort((a, b) => Number(b.code?.toLowerCase() === q) - Number(a.code?.toLowerCase() === q));
}

export function locationDirectoryRows<T extends LocationSearchItem & { id: number; parent_id: number | null; type: string }>(nodes: T[], expanded: Set<number>, query: string, type: string) {
  if (type === 'all' && !query.trim()) return treeRows(nodes, expanded);
  const matches = searchLocations(query, nodes).filter(node => type === 'all' || node.type === type);
  return matches.map(node => ({ node, depth: 0, hasChildren: false }));
}

export function treeRows<T extends { id: number; parent_id: number | null }>(nodes: T[], expanded: Set<number>): { node: T; depth: number; hasChildren: boolean }[] {
  const children = new Map<number | null, T[]>();
  for (const node of nodes) {
    const siblings = children.get(node.parent_id) ?? [];
    siblings.push(node);
    children.set(node.parent_id, siblings);
  }
  const out: { node: T; depth: number; hasChildren: boolean }[] = [];
  const visited = new Set<number>();
  function visit(parent: number | null, depth: number) {
    for (const node of children.get(parent) ?? []) {
      if (visited.has(node.id)) continue;
      visited.add(node.id);
      out.push({ node, depth, hasChildren: children.has(node.id) });
      if (expanded.has(node.id)) visit(node.id, depth + 1);
    }
  }
  visit(null, 0);
  return out;
}
