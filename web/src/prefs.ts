const locationKey = "youchu-last-location-id";
const categoryKey = "youchu-last-category-ids";

export function readLastLocationId(): number | null {
  const value = localStorage.getItem(locationKey);
  if (value == null || !/^[1-9][0-9]*$/.test(value)) return null;
  return Number(value);
}

export function writeLastLocationId(id: number | null): void {
  if (id == null) {
    localStorage.removeItem(locationKey);
    return;
  }
  localStorage.setItem(locationKey, String(id));
}

export function readLastCategoryIds(): number[] {
  const raw = localStorage.getItem(categoryKey);
  if (raw == null || raw === "") return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is number => typeof item === "number" && Number.isInteger(item) && item >= 1);
  } catch {
    return [];
  }
}

export function writeLastCategoryIds(ids: number[]): void {
  if (ids.length === 0) {
    localStorage.removeItem(categoryKey);
    return;
  }
  localStorage.setItem(categoryKey, JSON.stringify(ids));
}

export function rememberItemPlacement(locationIds: number[], categoryIds: number[]): void {
  writeLastLocationId(locationIds[0] ?? null);
  writeLastCategoryIds(categoryIds);
}
