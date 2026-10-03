export type ItemListSort = "created_at" | "name";

export type ItemListState = {
  q: string;
  unlocated: boolean;
  inLocation: string;
  locationSelf: boolean;
  categoryIds: string[];
  matchAll: boolean;
  categorySelf: boolean;
  uncategorized: boolean;
  sort: ItemListSort;
  offset: string;
};

export function readItemList(params: URLSearchParams): ItemListState {
  const categoryRaw = params.get("category") ?? "";
  const categoryIds = categoryRaw === "" ? [] : categoryRaw.split(",");
  return {
    q: params.get("q") ?? "",
    unlocated: params.get("placement") === "unlocated",
    inLocation: params.get("in_location") ?? "",
    locationSelf: params.get("in_location_descendants") === "0",
    categoryIds,
    matchAll: params.get("category_match") === "all",
    categorySelf: params.get("category_descendants") === "0",
    uncategorized: params.get("uncategorized") === "1",
    sort: params.get("sort") === "name" ? "name" : "created_at",
    offset: params.get("offset") ?? "",
  };
}

export function writeItemList(state: ItemListState): URLSearchParams {
  const next: ItemListState = { ...state };
  if (next.unlocated) {
    next.inLocation = "";
    next.locationSelf = false;
  }
  if (next.inLocation === "") {
    next.locationSelf = false;
  } else {
    next.unlocated = false;
  }
  if (next.uncategorized) {
    next.categoryIds = [];
    next.matchAll = false;
    next.categorySelf = false;
  }
  if (next.categoryIds.length === 0) {
    next.matchAll = false;
    next.categorySelf = false;
  } else {
    next.uncategorized = false;
  }
  if (next.categoryIds.length < 2) {
    next.matchAll = false;
  }
  const params = new URLSearchParams();
  const q = next.q.trim();
  if (q !== "") params.set("q", q);
  if (next.unlocated) params.set("placement", "unlocated");
  if (next.inLocation !== "") {
    params.set("in_location", next.inLocation);
    if (next.locationSelf) params.set("in_location_descendants", "0");
  }
  if (next.uncategorized) params.set("uncategorized", "1");
  if (next.categoryIds.length > 0) {
    params.set("category", next.categoryIds.join(","));
    if (next.matchAll) params.set("category_match", "all");
    if (next.categorySelf) params.set("category_descendants", "0");
  }
  if (next.sort === "name") params.set("sort", "name");
  if (next.offset !== "" && next.offset !== "0") params.set("offset", next.offset);
  return params;
}

export function clampPageOffset(total: number, offset: number, limit = 30): number {
  if (total <= 0 || offset <= 0) return 0;
  if (offset < total) return offset;
  return Math.floor((total - 1) / limit) * limit;
}
