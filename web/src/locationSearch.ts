export type LocationSearchNode = { name: string; code: string | null };

export type LocationSearchItem = {
  name: string;
  code: string | null;
  path: LocationSearchNode[];
};

function nodeLabel(node: LocationSearchNode): string {
  return node.code ? `${node.name} ${node.code}` : node.name;
}

function haystack(loc: LocationSearchItem): string {
  const nodes = loc.path.length > 0 ? loc.path : [{ name: loc.name, code: loc.code }];
  const parts: string[] = [loc.name, loc.code ?? "", nodes.map(nodeLabel).join(" / ")];
  for (const node of nodes) {
    parts.push(node.name, node.code ?? "");
  }
  return parts.join("\n").toLocaleLowerCase("en");
}

export function filterLocations<T extends LocationSearchItem>(query: string, locations: T[]): T[] {
  const q = query.trim().toLocaleLowerCase("en");
  if (q === "") return locations;
  return locations.filter((loc) => haystack(loc).includes(q));
}
