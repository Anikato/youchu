export type Me = { username: string };

export type AccessTokenScope = "read" | "organize" | "write";

export type AccessToken = {
  id: number;
  name: string;
  token_prefix: string;
  scopes: AccessTokenScope[];
  created_at: string;
};

export type CreatedAccessToken = AccessToken & { token: string };

export type LocationType = "area" | "fixed" | "movable";

export type PathNode = {
  id: number;
  name: string;
  type: string;
  code: string | null;
  icon: string | null;
  custom_icon_id: number | null;
};

export type Location = {
  id: number;
  name: string;
  type: LocationType;
  code: string | null;
  parent_id: number | null;
  icon: string | null;
  custom_icon_id: number | null;
  version: number;
  created_at: string;
  updated_at: string;
  path: PathNode[];
  direct_item_count: number;
};

export type LocationIconRecord = {
  id: number;
  name: string;
  svg: string;
  version: number;
  created_at: string;
  updated_at: string;
};

export type ItemLocation = {
  location_id: number;
  note: string | null;
  path: PathNode[];
};

export type CategoryPathNode = { id: number; name: string };

export type Category = {
  id: number;
  name: string;
  parent_id: number | null;
  version: number;
  created_at: string;
  updated_at: string;
  path: CategoryPathNode[];
  direct_item_count: number;
};

export type ItemCategory = {
  category_id: number;
  source: "human" | "ai";
  path: CategoryPathNode[];
};

export type Photo = {
  id: number;
  item_id: number;
  position: number;
  width: number;
  height: number;
  byte_size: number;
  version: number;
  created_at: string;
  updated_at: string;
};

export type CoverPhoto = { id: number };

export type ReturnTask = {
  id: number;
  item_id: number;
  item_name: string;
  part_note: string | null;
  reason: string | null;
  destination_note: string | null;
  completed_at: string | null;
  version: number;
  created_at: string;
  updated_at: string;
  cover_photo: CoverPhoto | null;
};

export type ReturnTaskCreate = {
  part_note?: string | null;
  reason?: string | null;
  destination_note?: string | null;
};

export type Item = {
  id: number;
  name: string;
  alias: string | null;
  model: string | null;
  spec: string | null;
  quantity_note: string | null;
  note: string | null;
  version: number;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
  locations: ItemLocation[];
  categories: ItemCategory[];
  return_tasks: ReturnTask[];
  photos: Photo[];
};

export type Page<T> = {
  data: T[];
  total: number;
  limit: number;
  offset: number;
};

export type ItemLocationWrite = {
  location_id: number;
  note?: string | null;
};

export type ItemCreate = {
  name: string;
  alias?: string | null;
  model?: string | null;
  spec?: string | null;
  quantity_note?: string | null;
  note?: string | null;
  locations?: ItemLocationWrite[];
  categories?: { category_id: number }[];
};

export type ItemUpdate = ItemCreate & { version: number };

export type LocationCreate = {
  name: string;
  type: LocationType;
  code?: string | null;
  parent_id?: number | null;
  icon?: string | null;
  custom_icon_id?: number | null;
};

export type LocationUpdate = {
  version: number;
  name?: string;
  code?: string | null;
  parent_id?: number | null;
  icon?: string | null;
  custom_icon_id?: number | null;
};

export type CategoryCreate = {
  name: string;
  parent_id?: number | null;
};

export type CategoryUpdate = {
  version: number;
  name?: string;
  parent_id?: number | null;
};

export class ApiError extends Error {
  code: string;
  fields?: Record<string, string>;

  constructor(code: string, message: string, fields?: Record<string, string>) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.fields = fields;
  }
}

export function isUnauthenticated(error: unknown): boolean {
  return error instanceof ApiError && error.code === "unauthenticated";
}

export function isVersionConflict(error: unknown): boolean {
  return error instanceof ApiError && error.code === "version_conflict";
}

export function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === "not_found";
}

export function isAlreadyCompleted(error: unknown): boolean {
  return error instanceof ApiError && error.code === "already_completed";
}

export function isIconInUse(error: unknown): boolean {
  return error instanceof ApiError && error.code === "icon_in_use";
}

type ErrorBody = {
  code?: unknown;
  message?: unknown;
  fields?: unknown;
};

function readFields(value: unknown): Record<string, string> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const fields: Record<string, string> = {};
  for (const [key, item] of Object.entries(value)) {
    if (typeof item === "string") fields[key] = item;
  }
  return fields;
}

async function parseError(response: Response, fallback: string): Promise<ApiError> {
  try {
    const body = (await response.json()) as ErrorBody;
    const code =
      typeof body.code === "string" && body.code
        ? body.code
        : response.status === 401
          ? "unauthenticated"
          : "";
    const message = typeof body.message === "string" && body.message ? body.message : fallback;
    return new ApiError(code, message, readFields(body.fields));
  } catch {
    return new ApiError(response.status === 401 ? "unauthenticated" : "", fallback);
  }
}

async function fetchSameOrigin(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  // FormData 必须由浏览器带 multipart boundary，不能改成 application/json。
  if (init.body != null && !headers.has("Content-Type") && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  return fetch(path, {
    ...init,
    credentials: "same-origin",
    cache: "no-store",
    headers,
  });
}

async function request(path: string, init: RequestInit = {}, unauthorizedFallback = "未登录"): Promise<Response> {
  const response = await fetchSameOrigin(path, init);
  if (response.status === 401) {
    throw await parseError(response, unauthorizedFallback);
  }
  return response;
}

function withQuery(path: string, params: URLSearchParams): string {
  const suffix = params.toString();
  return suffix ? `${path}?${suffix}` : path;
}

function normalizePathNode(node: PathNode): PathNode {
  return { ...node, icon: node.icon ?? null, custom_icon_id: node.custom_icon_id ?? null };
}

function normalizeLocation(loc: Location): Location {
  return {
    ...loc,
    icon: loc.icon ?? null,
    custom_icon_id: loc.custom_icon_id ?? null,
    path: Array.isArray(loc.path) ? loc.path.map(normalizePathNode) : [],
    direct_item_count: loc.direct_item_count ?? 0,
  };
}

function normalizeCategory(cat: Category): Category {
  return {
    ...cat,
    path: Array.isArray(cat.path) ? cat.path : [],
    direct_item_count: cat.direct_item_count ?? 0,
  };
}

function normalizeReturnTask(task: ReturnTask): ReturnTask {
  return {
    ...task,
    part_note: task.part_note ?? null,
    reason: task.reason ?? null,
    destination_note: task.destination_note ?? null,
    completed_at: task.completed_at ?? null,
    cover_photo: task.cover_photo ?? null,
  };
}

function normalizeItem(item: Item): Item {
  const links = Array.isArray(item.locations) ? item.locations : [];
  const categories = Array.isArray(item.categories) ? item.categories : [];
  const tasks = Array.isArray(item.return_tasks) ? item.return_tasks : [];
  const photos = Array.isArray(item.photos)
    ? [...item.photos].sort((a, b) => a.position - b.position || a.id - b.id)
    : [];
  return {
    ...item,
    deleted_at: item.deleted_at ?? null,
    locations: links.map((link) => ({
      ...link,
      path: Array.isArray(link.path) ? link.path.map(normalizePathNode) : [],
    })),
    categories: categories.map((link) => ({
      ...link,
      path: Array.isArray(link.path) ? link.path : [],
    })),
    return_tasks: tasks.map(normalizeReturnTask),
    photos,
  };
}

function normalizePage<T>(page: Page<T>, mapItem: (item: T) => T): Page<T> {
  return {
    data: Array.isArray(page.data) ? page.data.map(mapItem) : [],
    total: page.total,
    limit: page.limit,
    offset: page.offset,
  };
}

async function getJSON<T>(path: string, fallback: string): Promise<T> {
  const response = await request(path);
  if (!response.ok) throw await parseError(response, fallback);
  return response.json() as Promise<T>;
}

async function sendJSON<T>(path: string, method: string, body: unknown, fallback: string): Promise<T> {
  const response = await request(path, { method, body: JSON.stringify(body) });
  if (!response.ok) throw await parseError(response, fallback);
  return response.json() as Promise<T>;
}

// 登录页靠 401 得到 null。这里抛 unauthenticated 会让未登录状态来回跳转。
export async function fetchMe(): Promise<Me | null> {
  const response = await fetchSameOrigin("/api/v1/me");
  if (response.status === 401) return null;
  if (!response.ok) throw await parseError(response, "无法读取当前用户");
  return response.json() as Promise<Me>;
}

export async function login(username: string, password: string): Promise<void> {
  const response = await request(
    "/api/v1/session",
    {
      method: "POST",
      body: JSON.stringify({ username, password }),
    },
    "登录失败",
  );
  if (!response.ok) throw await parseError(response, "登录失败");
}

export async function logout(): Promise<void> {
  const response = await request("/api/v1/session", { method: "DELETE" }, "退出失败");
  if (!response.ok) throw await parseError(response, "退出失败");
}

export async function updateUsername(username: string): Promise<Me> {
  return sendJSON<Me>("/api/v1/me", "PATCH", { username }, "无法保存用户名");
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  const response = await request(
    "/api/v1/me/password",
    { method: "POST", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) },
    "无法修改密码",
  );
  if (!response.ok) throw await parseError(response, "无法修改密码");
}

export async function listTokens(): Promise<AccessToken[]> {
  const page = await getJSON<{ data: AccessToken[] }>("/api/v1/tokens", "无法读取接入令牌");
  return Array.isArray(page.data) ? page.data : [];
}

export async function createToken(name: string, scopes: AccessTokenScope[]): Promise<CreatedAccessToken> {
  return sendJSON<CreatedAccessToken>("/api/v1/tokens", "POST", { name, scopes }, "无法创建接入令牌");
}

export async function revokeToken(id: number): Promise<void> {
  const response = await request(`/api/v1/tokens/${encodeURIComponent(String(id))}`, { method: "DELETE" });
  if (!response.ok) throw await parseError(response, "无法撤销接入令牌");
}

async function getPage<T>(path: string, params: URLSearchParams, limit: number, offset: number): Promise<Page<T>> {
  const query = new URLSearchParams(params);
  query.set("limit", String(limit));
  query.set("offset", String(offset));
  const page = await getJSON<Page<T>>(withQuery(path, query), "无法读取列表");
  return {
    data: Array.isArray(page.data) ? page.data : [],
    total: page.total,
    limit: page.limit,
    offset: page.offset,
  };
}

export async function fetchAllLocations(params: URLSearchParams): Promise<Location[]> {
  const out: Location[] = [];
  let offset = 0;
  for (;;) {
    const page = await getPage<Location>("/api/v1/locations", params, 100, offset);
    out.push(...page.data);
    if (out.length >= page.total || page.data.length === 0) {
      return out.map(normalizeLocation);
    }
    offset += page.data.length;
  }
}

export async function listLocations(params: URLSearchParams): Promise<Page<Location>> {
  const page = await getJSON<Page<Location>>(withQuery("/api/v1/locations", params), "无法读取位置");
  return normalizePage(page, normalizeLocation);
}

export async function getLocation(id: string): Promise<Location> {
  const loc = await getJSON<Location>(`/api/v1/locations/${encodeURIComponent(id)}`, "无法读取位置");
  return normalizeLocation(loc);
}

export async function createLocation(body: LocationCreate): Promise<Location> {
  const loc = await sendJSON<Location>("/api/v1/locations", "POST", body, "无法新增位置");
  return normalizeLocation(loc);
}

export async function updateLocation(id: string, body: LocationUpdate): Promise<Location> {
  const loc = await sendJSON<Location>(`/api/v1/locations/${encodeURIComponent(id)}`, "PATCH", body, "无法保存位置");
  return normalizeLocation(loc);
}

export async function cloneLocation(id: string): Promise<Location> {
  const loc = await sendJSON<Location>(`/api/v1/locations/${encodeURIComponent(id)}/clone`, "POST", {}, "无法克隆");
  return normalizeLocation(loc);
}

export async function listLocationIcons(): Promise<LocationIconRecord[]> {
  const page = await getJSON<{ data: LocationIconRecord[] }>("/api/v1/location-icons", "无法读取图标");
  return Array.isArray(page.data) ? page.data : [];
}

export async function createLocationIcon(name: string, svg: string): Promise<LocationIconRecord> {
  return sendJSON<LocationIconRecord>("/api/v1/location-icons", "POST", { name, svg }, "无法上传图标");
}

export async function updateLocationIcon(
  id: number,
  body: { version: number; name?: string; svg?: string },
): Promise<LocationIconRecord> {
  return sendJSON<LocationIconRecord>(
    `/api/v1/location-icons/${encodeURIComponent(String(id))}`,
    "PATCH",
    body,
    "无法保存图标",
  );
}

export async function deleteLocationIcon(id: number, version: number): Promise<void> {
  const response = await request(
    `/api/v1/location-icons/${encodeURIComponent(String(id))}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法删除图标");
}

export async function deleteLocation(id: string, version: number): Promise<void> {
  const response = await request(
    `/api/v1/locations/${encodeURIComponent(id)}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法删除位置");
}

export async function fetchAllCategories(params: URLSearchParams): Promise<Category[]> {
  const out: Category[] = [];
  let offset = 0;
  for (;;) {
    const page = await getPage<Category>("/api/v1/categories", params, 100, offset);
    out.push(...page.data);
    if (out.length >= page.total || page.data.length === 0) {
      return out.map(normalizeCategory);
    }
    offset += page.data.length;
  }
}

export async function listCategories(params: URLSearchParams): Promise<Page<Category>> {
  const page = await getJSON<Page<Category>>(withQuery("/api/v1/categories", params), "无法读取分类");
  return normalizePage(page, normalizeCategory);
}

export async function getCategory(id: string): Promise<Category> {
  const cat = await getJSON<Category>(`/api/v1/categories/${encodeURIComponent(id)}`, "无法读取分类");
  return normalizeCategory(cat);
}

export async function createCategory(body: CategoryCreate): Promise<Category> {
  const cat = await sendJSON<Category>("/api/v1/categories", "POST", body, "无法新增分类");
  return normalizeCategory(cat);
}

export async function updateCategory(id: string, body: CategoryUpdate): Promise<Category> {
  const cat = await sendJSON<Category>(`/api/v1/categories/${encodeURIComponent(id)}`, "PATCH", body, "无法保存分类");
  return normalizeCategory(cat);
}

export async function deleteCategory(id: string, version: number): Promise<void> {
  const response = await request(
    `/api/v1/categories/${encodeURIComponent(id)}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法删除分类");
}

export async function listItems(params: URLSearchParams): Promise<Page<Item>> {
  const page = await getJSON<Page<Item>>(withQuery("/api/v1/items", params), "无法读取物品");
  return normalizePage(page, normalizeItem);
}

export async function getItem(id: string): Promise<Item> {
  const item = await getJSON<Item>(`/api/v1/items/${encodeURIComponent(id)}`, "无法读取物品");
  return normalizeItem(item);
}

export async function createItem(body: ItemCreate): Promise<Item> {
  const item = await sendJSON<Item>("/api/v1/items", "POST", body, "无法新增物品");
  return normalizeItem(item);
}

export async function updateItem(id: string, body: ItemUpdate): Promise<Item> {
  const item = await sendJSON<Item>(`/api/v1/items/${encodeURIComponent(id)}`, "PATCH", body, "无法保存物品");
  return normalizeItem(item);
}

export async function deleteItem(id: string, version: number): Promise<void> {
  const response = await request(
    `/api/v1/items/${encodeURIComponent(id)}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法删除物品");
}

export async function listReturnTasks(params: URLSearchParams): Promise<Page<ReturnTask>> {
  const page = await getJSON<Page<ReturnTask>>(withQuery("/api/v1/return-tasks", params), "无法读取待归位事项");
  return normalizePage(page, normalizeReturnTask);
}

export async function createReturnTask(itemId: string, body: ReturnTaskCreate): Promise<ReturnTask> {
  const task = await sendJSON<ReturnTask>(
    `/api/v1/items/${encodeURIComponent(itemId)}/return-tasks`,
    "POST",
    body,
    "无法新增归位事项",
  );
  return normalizeReturnTask(task);
}

export async function completeReturnTask(id: number, version: number): Promise<ReturnTask> {
  const task = await sendJSON<ReturnTask>(
    `/api/v1/return-tasks/${encodeURIComponent(String(id))}/complete`,
    "POST",
    { version },
    "无法完成归位",
  );
  return normalizeReturnTask(task);
}

export async function deleteReturnTask(id: number, version: number): Promise<void> {
  const response = await request(
    `/api/v1/return-tasks/${encodeURIComponent(String(id))}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法去掉事项");
}

export async function listTrash(params: URLSearchParams): Promise<Page<Item>> {
  const page = await getJSON<Page<Item>>(withQuery("/api/v1/trash", params), "无法读取回收站");
  return normalizePage(page, normalizeItem);
}

export async function getTrashItem(id: string): Promise<Item> {
  const item = await getJSON<Item>(`/api/v1/trash/${encodeURIComponent(id)}`, "无法读取回收站物品");
  return normalizeItem(item);
}

export async function restoreTrashItem(id: string, version: number): Promise<Item> {
  const item = await sendJSON<Item>(
    `/api/v1/trash/${encodeURIComponent(id)}/restore`,
    "POST",
    { version },
    "无法恢复物品",
  );
  return normalizeItem(item);
}

export async function purgeTrashItem(id: string, version: number): Promise<void> {
  const response = await request(
    `/api/v1/trash/${encodeURIComponent(id)}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法永久删除");
}

export async function uploadItemPhoto(itemId: string, file: File): Promise<Photo> {
  const body = new FormData();
  body.append("file", file);
  const response = await request(`/api/v1/items/${encodeURIComponent(itemId)}/photos`, {
    method: "POST",
    body,
  });
  if (!response.ok) throw await parseError(response, "无法添加照片");
  return response.json() as Promise<Photo>;
}

export async function setPhotoFirst(id: number, version: number): Promise<Photo> {
  return sendJSON<Photo>(
    `/api/v1/photos/${encodeURIComponent(String(id))}/first`,
    "POST",
    { version },
    "无法设为第一张",
  );
}

export async function deletePhoto(id: number, version: number): Promise<void> {
  const response = await request(
    `/api/v1/photos/${encodeURIComponent(String(id))}?version=${encodeURIComponent(String(version))}`,
    { method: "DELETE" },
  );
  if (!response.ok) throw await parseError(response, "无法删除照片");
}
