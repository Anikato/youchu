import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useEffect, useState, type CSSProperties } from "react";
import { Link, Navigate, Outlet, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import {
  ApiError,
  type AccessTokenScope,
  type Category,
  type CategoryCreate,
  type CategoryPathNode,
  type CategoryUpdate,
  type Item,
  type ItemCategory,
  type ItemCreate,
  type ItemLocation,
  type ItemUpdate,
  type Location,
  type LocationCreate,
  type LocationType,
  type LocationUpdate,
  type PathNode,
  type Photo,
  type ReturnTask,
  type ReturnTaskCreate,
  changePassword,
  completeReturnTask,
  createCategory,
  createItem,
  createLocation,
  createReturnTask,
  createToken,
  deleteCategory,
  deleteItem,
  deleteLocation,
  deletePhoto,
  deleteReturnTask,
  fetchAllCategories,
  fetchAllLocations,
  fetchMe,
  getCategory,
  getItem,
  getLocation,
  getTrashItem,
  isAlreadyCompleted,
  isNotFound,
  isUnauthenticated,
  isVersionConflict,
  listCategories,
  listItems,
  listLocations,
  listReturnTasks,
  listTokens,
  listTrash,
  logout,
  purgeTrashItem,
  restoreTrashItem,
  revokeToken,
  setPhotoFirst,
  updateCategory,
  updateItem,
  updateLocation,
  updateUsername,
  uploadItemPhoto,
} from "./api";
import { clampPageOffset, readItemList, writeItemList, type ItemListState } from "./filters";
import { ICON_GROUPS, LocationIcon, defaultIconLabel, resolvedIcon } from "./icons";
import styles from "./styles.module.css";
import { type Accent, type Theme, readAccent, readTheme, setAccent, setTheme } from "./theme";

const typeLabel: Record<LocationType, string> = {
  area: "区域",
  fixed: "固定储物位",
  movable: "移动容器",
};

const tokenScopeLabel: Record<AccessTokenScope, string> = {
  read: "读取",
  organize: "归类",
  write: "写入",
};

function formatTokenScopes(scopes: AccessTokenScope[]): string {
  return scopes.map((scope) => tokenScopeLabel[scope] ?? scope).join(" / ");
}

const allTypes: LocationType[] = ["area", "fixed", "movable"];

const inlineFields = new Set([
  "name",
  "alias",
  "model",
  "spec",
  "quantity_note",
  "note",
  "code",
  "parent_id",
  "icon",
  "locations",
  "categories",
  "type",
  "part_note",
  "reason",
  "destination_note",
  "username",
  "current_password",
  "new_password",
  "scopes",
]);

function isLocationType(value: string): value is LocationType {
  return value === "area" || value === "fixed" || value === "movable";
}

function childTypes(parentType: string): LocationType[] {
  if (parentType === "area") return ["area", "fixed"];
  if (parentType === "fixed") return ["fixed", "movable"];
  return parentType === "movable" ? ["movable"] : [];
}

function formatNode(node: { name: string; code: string | null }): string {
  if (node.code) return `${node.name} ${node.code}`;
  return node.name;
}

function formatPath(path: { name: string; code: string | null }[]): string {
  return path.map((node) => formatNode(node)).join(" / ");
}

function formatCategoryPath(path: CategoryPathNode[]): string {
  return path.map((node) => node.name).join(" / ");
}

function treeDepth(pathLength: number): number {
  return Math.max(0, Math.min((pathLength || 1) - 1, 6));
}

function treeStyle(pathLength: number): CSSProperties {
  return { ["--tree-depth" as string]: treeDepth(pathLength) } as CSSProperties;
}

function optionPrefix(pathLength: number): string {
  return "\u3000".repeat(treeDepth(pathLength));
}

function LocationPathLine({ path }: { path: PathNode[] }) {
  const last = path[path.length - 1];
  return (
    <span className={`${styles.path} ${styles.pathWithIcon}`}>
      {last ? <LocationIcon name={resolvedIcon(last)} /> : null}
      <span>{path.length > 0 ? formatPath(path) : ""}</span>
    </span>
  );
}

function IconPicker({
  value,
  type,
  onChange,
  error,
}: {
  value: string | null;
  type?: LocationType | "";
  onChange: (value: string | null) => void;
  error?: string;
}) {
  const previewType = type && isLocationType(type) ? type : "";
  return (
    <fieldset className={styles.iconPicker}>
      <legend>图标</legend>
      <label className={`${styles.iconChoice} ${value == null ? styles.iconChoiceOn : ""}`}>
        <input type="radio" name="icon" checked={value == null} onChange={() => onChange(null)} />
        {previewType ? <LocationIcon name={resolvedIcon({ icon: null, type: previewType })} /> : null}
        <span>默认（{defaultIconLabel(previewType)}）</span>
      </label>
      {ICON_GROUPS.map((group) => (
        <div key={group.label}>
          <p className={styles.iconGroupLabel}>{group.label}</p>
          <div className={styles.iconGrid}>
            {group.icons.map((icon) => (
              <label
                key={icon.slug}
                className={`${styles.iconChoice} ${value === icon.slug ? styles.iconChoiceOn : ""}`}
              >
                <input
                  type="radio"
                  name="icon"
                  checked={value === icon.slug}
                  onChange={() => onChange(icon.slug)}
                />
                <LocationIcon name={icon.slug} />
                <span>{icon.label}</span>
              </label>
            ))}
          </div>
        </div>
      ))}
      {error ? <span className={styles.error}>{error}</span> : null}
    </fieldset>
  );
}

function cloneLinks(links: ItemLocation[]): ItemLocation[] {
  return links.map((link) => ({
    location_id: link.location_id,
    note: link.note,
    path: link.path.map((node) => ({ ...node })),
  }));
}

function cloneCategoryLinks(links: ItemCategory[]): ItemCategory[] {
  return links.map((link) => ({
    category_id: link.category_id,
    source: link.source,
    path: link.path.map((node) => ({ ...node })),
  }));
}

function categoryIdsOf(links: ItemCategory[]): number[] {
  return links.map((link) => link.category_id);
}

function categoryIdsEqual(left: number[], right: number[]): boolean {
  if (left.length !== right.length) return false;
  const a = [...left].sort((x, y) => x - y);
  const b = [...right].sort((x, y) => x - y);
  return a.every((id, index) => id === b[index]);
}

function messageOf(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

function asApiError(error: unknown, fallback: string): ApiError {
  return error instanceof ApiError ? error : new ApiError("", messageOf(error, fallback));
}

function mergeTask(tasks: ReturnTask[], task: ReturnTask): ReturnTask[] {
  const next = tasks.some((item) => item.id === task.id)
    ? tasks.map((item) => (item.id === task.id ? task : item))
    : [...tasks, task];
  return [...next].sort((a, b) => {
    const aDone = a.completed_at ? 1 : 0;
    const bDone = b.completed_at ? 1 : 0;
    if (aDone !== bDone) return aDone - bDone;
    return a.id - b.id;
  });
}

function partLabel(task: ReturnTask): string {
  return task.part_note ? task.part_note : "整件";
}

function fieldText(error: ApiError | null, key: string): string | undefined {
  return error?.fields?.[key];
}

function emptyToNull(value: string): string | null {
  return value === "" ? null : value;
}

function linksBody(links: ItemLocation[]): { location_id: number; note: string | null }[] {
  return links.map((link) => ({
    location_id: link.location_id,
    note: link.note == null || link.note === "" ? null : link.note,
  }));
}

type ItemDraft = {
  name: string;
  alias: string;
  model: string;
  spec: string;
  quantityNote: string;
  note: string;
  locations: ItemLocation[];
  categories: ItemCategory[];
};

function itemBody(draft: ItemDraft, originalCategoryIds?: number[]): ItemCreate {
  const body: ItemCreate = {
    name: draft.name,
    alias: emptyToNull(draft.alias),
    model: emptyToNull(draft.model),
    spec: emptyToNull(draft.spec),
    quantity_note: emptyToNull(draft.quantityNote),
    note: emptyToNull(draft.note),
    locations: linksBody(draft.locations),
  };
  const currentIds = categoryIdsOf(draft.categories);
  if (originalCategoryIds === undefined) {
    if (currentIds.length > 0) body.categories = currentIds.map((category_id) => ({ category_id }));
  } else if (!categoryIdsEqual(originalCategoryIds, currentIds)) {
    body.categories = currentIds.map((category_id) => ({ category_id }));
  }
  return body;
}

function locationCreateBody(
  type: LocationType,
  name: string,
  code: string,
  parentId: number | null,
  icon: string | null,
): LocationCreate {
  const body: LocationCreate = { name, type, icon };
  if (type !== "area") body.code = code;
  if (parentId != null) body.parent_id = parentId;
  return body;
}

function locationUpdateBody(
  type: LocationType,
  version: number,
  name: string,
  code: string,
  parentId: number | null,
  icon: string | null,
): LocationUpdate {
  const body: LocationUpdate = { version, name, parent_id: parentId, icon };
  if (type !== "area") body.code = code;
  return body;
}

function readNotice(state: unknown): string {
  if (!state || typeof state !== "object" || !("notice" in state)) return "";
  const notice = (state as { notice?: unknown }).notice;
  return typeof notice === "string" ? notice : "";
}

function setOffsetParam(current: URLSearchParams, key: string, offset: number): URLSearchParams {
  const next = new URLSearchParams(current);
  if (offset > 0) next.set(key, String(offset));
  else next.delete(key);
  return next;
}

function useOnUnauth(error: unknown) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!isUnauthenticated(error)) return;
    queryClient.setQueryData(["me"], null);
    navigate("/login", { replace: true });
  }, [error, navigate, queryClient]);
}

export function RequireAuth() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe, retry: false });
  if (me.isLoading) return <p className={styles.page}>正在确认登录状态</p>;
  if (!me.data) return <Navigate to="/login" replace />;
  return <AppShell username={me.data.username} />;
}

function AppShell({ username }: { username: string }) {
  const location = useLocation();
  const notice = readNotice(location.state);
  const pathname = location.pathname;
  const itemHere = pathname === "/" || pathname.startsWith("/items/");
  const locHere = pathname === "/locations" || pathname.startsWith("/locations/");
  const catHere = pathname === "/categories" || pathname.startsWith("/categories/");
  const retHere = pathname === "/returns" || pathname.startsWith("/returns/");

  return (
    <main className={styles.appPage}>
      <header className={styles.header}>
        <Link className={styles.brand} to="/" aria-label="有处">
          <img className={styles.brandIcon} src="/logo-yc.svg" alt="" />
          有处
        </Link>
        <nav className={styles.nav} aria-label="主要">
          <Link to="/" aria-current={itemHere ? "page" : undefined}>
            物品
          </Link>
          <Link to="/locations" aria-current={locHere ? "page" : undefined}>
            位置
          </Link>
          <Link to="/categories" aria-current={catHere ? "page" : undefined}>
            分类
          </Link>
          <Link to="/returns" aria-current={retHere ? "page" : undefined}>
            待归位
          </Link>
        </nav>
        <Link className={styles.user} to="/account" aria-current={pathname === "/account" ? "page" : undefined}>
          {username}
        </Link>
      </header>
      {notice ? <p>{notice}</p> : null}
      <Outlet />
    </main>
  );
}

function Loading() {
  return <p>正在加载</p>;
}

function LoadError({ error, onRetry, pending }: { error: unknown; onRetry: () => void; pending: boolean }) {
  const details = Object.entries(asApiError(error, "无法读取").fields ?? {});
  return (
    <div>
      <p className={styles.error}>{messageOf(error, "无法读取")}</p>
      {details.map(([key, text]) => (
        <p key={key} className={styles.error}>
          {text}
        </p>
      ))}
      <button className={styles.button} type="button" onClick={onRetry} disabled={pending}>
        重试
      </button>
    </div>
  );
}

function FormIssues({ error }: { error: ApiError | null }) {
  if (!error) return null;
  const extra = Object.entries(error.fields ?? {}).filter(([key]) => !inlineFields.has(key));
  return (
    <div>
      <p className={styles.error}>{error.message}</p>
      {extra.map(([key, text]) => (
        <p key={key} className={styles.error}>
          {text}
        </p>
      ))}
    </div>
  );
}

function TextField({
  label,
  name,
  value,
  onChange,
  error,
  multiline,
  hint,
}: {
  label: string;
  name: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  multiline?: boolean;
  hint?: string;
}) {
  return (
    <label className={styles.field}>
      {label}
      {hint ? <span className={styles.meta}>{hint}</span> : null}
      {multiline ? (
        <textarea name={name} value={value} autoComplete="off" onChange={(event) => onChange(event.target.value)} />
      ) : (
        <input name={name} value={value} autoComplete="off" onChange={(event) => onChange(event.target.value)} />
      )}
      {error ? <span className={styles.error}>{error}</span> : null}
    </label>
  );
}

function Pager({
  offset,
  limit,
  total,
  count,
  onPage,
}: {
  offset: number;
  limit: number;
  total: number;
  count: number;
  onPage: (offset: number) => void;
}) {
  const step = limit > 0 ? limit : Math.max(count, 1);
  const hasPrev = offset > 0;
  const hasNext = total > offset + count;
  if (!hasPrev && !hasNext) return null;
  return (
    <div className={styles.actions}>
      {hasPrev ? (
        <button className={styles.button} type="button" onClick={() => onPage(Math.max(0, offset - step))}>
          上一页
        </button>
      ) : null}
      {hasNext ? (
        <button className={styles.button} type="button" onClick={() => onPage(offset + step)}>
          下一页
        </button>
      ) : null}
    </div>
  );
}

function Breadcrumb({ path }: { path: PathNode[] }) {
  return (
    <p className={styles.path}>
      {path.map((node, index) => {
        const last = index === path.length - 1;
        return (
          <span key={node.id}>
            {index > 0 ? " / " : null}
            {last ? formatNode(node) : <Link to={`/locations/${node.id}`}>{formatNode(node)}</Link>}
          </span>
        );
      })}
    </p>
  );
}

function CategoryBreadcrumb({ path }: { path: CategoryPathNode[] }) {
  return (
    <p className={styles.path}>
      {path.map((node, index) => {
        const last = index === path.length - 1;
        return (
          <span key={node.id}>
            {index > 0 ? " / " : null}
            {last ? node.name : <Link to={`/categories/${node.id}`}>{node.name}</Link>}
          </span>
        );
      })}
    </p>
  );
}

function ConflictNotice({
  message,
  pending,
  onLoad,
  error,
}: {
  message: string;
  pending: boolean;
  onLoad: () => void;
  error: string;
}) {
  return (
    <div className={styles.stack}>
      <p className={styles.error}>{message}</p>
      <button className={styles.button} type="button" onClick={onLoad} disabled={pending}>
        加载最新内容
      </button>
      {error ? <p className={styles.error}>{error}</p> : null}
    </div>
  );
}

function DeleteConfirm({
  confirming,
  pending,
  error,
  onAsk,
  onCancel,
  onConfirm,
  warning = "永久删除，无法恢复",
  confirmLabel = "确认删除",
  askLabel = "删除",
}: {
  confirming: boolean;
  pending: boolean;
  error: ApiError | null;
  onAsk: () => void;
  onCancel: () => void;
  onConfirm: () => void;
  warning?: string;
  confirmLabel?: string;
  askLabel?: string;
}) {
  if (!confirming) {
    return (
      <button className={styles.buttonDanger} type="button" onClick={onAsk}>
        {askLabel}
      </button>
    );
  }
  return (
    <div className={styles.confirm}>
      <p>{warning}</p>
      <div className={styles.actions}>
        <button className={styles.buttonDanger} type="button" onClick={onConfirm} disabled={pending}>
          {confirmLabel}
        </button>
        <button className={styles.button} type="button" onClick={onCancel}>
          取消
        </button>
      </div>
      <FormIssues error={error} />
    </div>
  );
}

function ItemThumb({ photoId }: { photoId?: number | null }) {
  if (photoId == null) return <span className={styles.itemPlaceholder} aria-hidden="true">物</span>;
  return (
    <img className={styles.itemThumb} src={`/api/v1/photos/${photoId}/thumbnail`} alt="" loading="lazy" />
  );
}

function orderedPhotos(photos: Photo[]): Photo[] {
  return [...photos].sort((a, b) => a.position - b.position || a.id - b.id);
}

function mergeItemPhotos(cur: Item | undefined, photos: Photo[]): Item | undefined {
  if (!cur) return cur;
  const next = orderedPhotos(photos);
  const cover = next[0] ? { id: next[0].id } : null;
  return {
    ...cur,
    photos: next,
    return_tasks: cur.return_tasks.map((task) => ({ ...task, cover_photo: cover })),
  };
}

function photoFailureText(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.fields?.file) return error.fields.file;
  return messageOf(error, fallback);
}

function ItemPhotoSection({ itemId, photos, readOnly }: { itemId: string; photos: Photo[]; readOnly?: boolean }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ordered = orderedPhotos(photos);
  const [selectedId, setSelectedId] = useState<number | null>(ordered[0]?.id ?? null);
  const [notice, setNotice] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [authError, setAuthError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<{ id: number; version: number } | null>(null);
  const selected = ordered.find((photo) => photo.id === selectedId) ?? null;

  useOnUnauth(authError);

  useEffect(() => {
    const next = orderedPhotos(photos);
    if (next.length === 0) {
      setSelectedId(null);
      return;
    }
    setSelectedId((current) => {
      if (current != null && next.some((photo) => photo.id === current)) return current;
      return next[0].id;
    });
  }, [photos]);

  async function refreshPhotos(): Promise<Photo[]> {
    const latest = await getItem(itemId);
    queryClient.setQueryData(["item", itemId], (cur: Item | undefined) => mergeItemPhotos(cur, latest.photos));
    return latest.photos;
  }

  function invalidatePhotoLists() {
    void queryClient.invalidateQueries({ queryKey: ["items"] });
    void queryClient.invalidateQueries({ queryKey: ["locations"] });
    void queryClient.invalidateQueries({ queryKey: ["categories"] });
    void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
    void queryClient.invalidateQueries({ queryKey: ["trash"] });
  }

  async function reloadPhotosOrLeave(): Promise<boolean> {
    try {
      await refreshPhotos();
      return true;
    } catch (error) {
      setAuthError(error);
      if (isNotFound(error)) {
        navigate("/", { replace: true, state: { notice: "未找到" } });
        return false;
      }
      setErrorMessage(messageOf(error, "无法确认物品"));
      return false;
    }
  }

  async function handlePhotoGone() {
    const ok = await reloadPhotosOrLeave();
    if (!ok) return;
    setNotice("该照片已不存在");
    setErrorMessage("");
    setPendingDelete(null);
  }

  async function afterWrite() {
    await refreshPhotos();
    invalidatePhotoLists();
  }

  async function handleFiles(fileList: File[]) {
    setBusy(true);
    setNotice("");
    setErrorMessage("");
    let lastFail = "";
    for (const file of fileList) {
      try {
        await uploadItemPhoto(itemId, file);
        await afterWrite();
      } catch (error) {
        setAuthError(error);
        if (isNotFound(error)) {
          const ok = await reloadPhotosOrLeave();
          if (!ok) break;
          lastFail = photoFailureText(error, "无法添加照片");
          continue;
        }
        if (isVersionConflict(error)) {
          lastFail = messageOf(error, "记录已被修改");
          const ok = await reloadPhotosOrLeave();
          if (!ok) break;
          continue;
        }
        lastFail = photoFailureText(error, "无法添加照片");
      }
    }
    if (lastFail) setErrorMessage(lastFail);
    setBusy(false);
  }

  async function handleSetFirst() {
    if (!selected) return;
    setBusy(true);
    setNotice("");
    setErrorMessage("");
    try {
      await setPhotoFirst(selected.id, selected.version);
      await afterWrite();
    } catch (error) {
      setAuthError(error);
      if (isNotFound(error)) {
        await handlePhotoGone();
      } else if (isVersionConflict(error)) {
        setErrorMessage(messageOf(error, "记录已被修改"));
        await reloadPhotosOrLeave();
      } else {
        setErrorMessage(photoFailureText(error, "无法设为第一张"));
      }
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    if (!pendingDelete) return;
    setBusy(true);
    setNotice("");
    setErrorMessage("");
    try {
      await deletePhoto(pendingDelete.id, pendingDelete.version);
      await afterWrite();
      setPendingDelete(null);
    } catch (error) {
      setAuthError(error);
      if (isNotFound(error)) {
        await handlePhotoGone();
      } else if (isVersionConflict(error)) {
        try {
          const latestPhotos = await refreshPhotos();
          const next = latestPhotos.find((photo) => photo.id === pendingDelete.id);
          if (!next) {
            setNotice("该照片已不存在");
            setErrorMessage("");
            setPendingDelete(null);
          } else {
            setPendingDelete({ id: next.id, version: next.version });
            setErrorMessage(messageOf(error, "记录已被修改"));
          }
        } catch (refreshError) {
          setAuthError(refreshError);
          if (isNotFound(refreshError)) {
            navigate("/", { replace: true, state: { notice: "未找到" } });
          } else {
            setErrorMessage(messageOf(refreshError, "无法确认物品"));
          }
        }
      } else {
        setErrorMessage(photoFailureText(error, "无法删除照片"));
      }
    } finally {
      setBusy(false);
    }
  }

  const atLimit = ordered.length >= 20;

  return (
    <section>
      <h2 className={styles.sectionTitle}>照片</h2>
      {notice ? <p>{notice}</p> : null}
      {errorMessage ? <p className={styles.error}>{errorMessage}</p> : null}
      {selected ? (
        <img className={styles.photoOriginal} src={`/api/v1/photos/${selected.id}/original`} alt="" />
      ) : null}
      {ordered.length > 0 ? (
        <ul className={styles.photoThumbs}>
          {ordered.map((photo) => (
            <li key={photo.id}>
              <button
                className={styles.photoThumbButton}
                type="button"
                aria-label={photo.position === 0 ? "列表用" : `照片 ${photo.position + 1}`}
                aria-current={photo.id === selectedId ? "true" : undefined}
                onClick={() => setSelectedId(photo.id)}
              >
                <ItemThumb photoId={photo.id} />
              </button>
              {photo.position === 0 ? <span className={styles.meta}>列表用</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
      {!readOnly && selected && selected.position !== 0 ? (
        <div className={styles.actions}>
          <button className={styles.button} type="button" onClick={() => void handleSetFirst()} disabled={busy}>
            设为第一张
          </button>
        </div>
      ) : null}
      {!readOnly && selected ? (
        <DeleteConfirm
          confirming={pendingDelete != null}
          pending={busy}
          error={null}
          askLabel="删除这张照片"
          warning="删除这张照片"
          onAsk={() => {
            setPendingDelete({ id: selected.id, version: selected.version });
            setErrorMessage("");
            setNotice("");
          }}
          onCancel={() => setPendingDelete(null)}
          onConfirm={() => void handleDelete()}
        />
      ) : null}
      {readOnly ? null : atLimit ? (
        <p>一件物品最多 20 张照片</p>
      ) : (
        <label className={styles.fileButton}>
          添加照片
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            multiple
            disabled={busy}
            onChange={(event) => {
              // FileList is live; copy before clearing so the same control can pick again.
              const files = Array.from(event.currentTarget.files ?? []);
              event.currentTarget.value = "";
              if (files.length === 0) return;
              void handleFiles(files);
            }}
          />
        </label>
      )}
    </section>
  );
}

function ItemLocationsField({
  links,
  onChange,
  error,
}: {
  links: ItemLocation[];
  onChange: (links: ItemLocation[]) => void;
  error?: string;
}) {
  const options = useQuery({
    queryKey: ["locations", "flat"],
    queryFn: () => fetchAllLocations(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  useOnUnauth(options.error);
  const [picked, setPicked] = useState("");
  const chosen = new Set(links.map((link) => link.location_id));
  const available = (options.data ?? []).filter((loc) => !chosen.has(loc.id));

  return (
    <div className={styles.field}>
      <span>存放位置</span>
      {links.map((link) => (
        <div key={link.location_id}>
          {link.path.length > 0 ? <LocationPathLine path={link.path} /> : <span className={styles.path}>{String(link.location_id)}</span>}
          <label className={styles.field}>
            放置说明
            <textarea
              name={`placement-${link.location_id}`}
              value={link.note ?? ""}
              autoComplete="off"
              onChange={(event) => {
                const note = event.target.value === "" ? null : event.target.value;
                onChange(links.map((item) => (item.location_id === link.location_id ? { ...item, note } : item)));
              }}
            />
          </label>
          <button
            className={styles.button}
            type="button"
            onClick={() => onChange(links.filter((item) => item.location_id !== link.location_id))}
          >
            移除
          </button>
        </div>
      ))}
      {options.isLoading ? <Loading /> : null}
      {options.isError ? (
        <LoadError error={options.error} onRetry={() => void options.refetch()} pending={options.isFetching} />
      ) : null}
      {options.data ? (
        <div className={styles.stack}>
          <select name="location_id" value={picked} onChange={(event) => setPicked(event.target.value)}>
            <option value="">选择存放位置</option>
            {available.map((loc) => (
              <option key={loc.id} value={loc.id}>
                {optionPrefix(loc.path.length)}
                {formatPath(loc.path)}
              </option>
            ))}
          </select>
          <button
            className={styles.button}
            type="button"
            disabled={picked === ""}
            onClick={() => {
              const loc = options.data?.find((item) => String(item.id) === picked);
              if (!loc || chosen.has(loc.id)) return;
              onChange([
                ...links,
                {
                  location_id: loc.id,
                  note: null,
                  path: loc.path.map((node) => ({ ...node })),
                },
              ]);
              setPicked("");
            }}
          >
            添加
          </button>
        </div>
      ) : null}
      {error ? <span className={styles.error}>{error}</span> : null}
    </div>
  );
}

function ItemCategoriesField({
  links,
  onChange,
  error,
}: {
  links: ItemCategory[];
  onChange: (links: ItemCategory[]) => void;
  error?: string;
}) {
  const options = useQuery({
    queryKey: ["categories", "flat"],
    queryFn: () => fetchAllCategories(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  useOnUnauth(options.error);
  const [picked, setPicked] = useState("");
  const chosen = new Set(links.map((link) => link.category_id));
  const available = (options.data ?? []).filter((cat) => !chosen.has(cat.id));

  return (
    <div className={styles.field}>
      <span>分类</span>
      {links.map((link) => (
        <div key={link.category_id}>
          <span className={styles.path}>
            {link.path.length > 0 ? formatCategoryPath(link.path) : String(link.category_id)}
          </span>
          <button
            className={styles.button}
            type="button"
            onClick={() => onChange(links.filter((item) => item.category_id !== link.category_id))}
          >
            移除
          </button>
        </div>
      ))}
      {options.isLoading ? <Loading /> : null}
      {options.isError ? (
        <LoadError error={options.error} onRetry={() => void options.refetch()} pending={options.isFetching} />
      ) : null}
      {options.data ? (
        <div className={styles.stack}>
          <select name="category_id" value={picked} onChange={(event) => setPicked(event.target.value)}>
            <option value="">选择分类</option>
            {available.map((cat) => (
              <option key={cat.id} value={cat.id}>
                {optionPrefix(cat.path.length)}
                {formatCategoryPath(cat.path)}
              </option>
            ))}
          </select>
          <button
            className={styles.button}
            type="button"
            disabled={picked === ""}
            onClick={() => {
              const cat = options.data?.find((item) => String(item.id) === picked);
              if (!cat || chosen.has(cat.id)) return;
              onChange([
                ...links,
                {
                  category_id: cat.id,
                  source: "human",
                  path: cat.path.map((node) => ({ ...node })),
                },
              ]);
              setPicked("");
            }}
          >
            添加
          </button>
        </div>
      ) : null}
      {error ? <span className={styles.error}>{error}</span> : null}
    </div>
  );
}

function ParentField({
  type,
  excludeId,
  parentId,
  onChange,
  allowNone,
  error,
  fallbackLabel,
}: {
  type: LocationType;
  excludeId?: number;
  parentId: number | null;
  onChange: (parentId: number | null) => void;
  allowNone: boolean;
  error?: string;
  fallbackLabel?: string;
}) {
  const options = useQuery({
    queryKey: ["locations", "eligible", type, excludeId ?? null],
    queryFn: () => {
      const params = new URLSearchParams({ eligible_parent_for: type });
      if (excludeId != null) params.set("exclude", String(excludeId));
      return fetchAllLocations(params);
    },
    retry: false,
  });
  useOnUnauth(options.error);
  const known = parentId != null && (options.data ?? []).some((loc) => loc.id === parentId);

  return (
    <label className={styles.field}>
      父级
      {options.isLoading ? <Loading /> : null}
      {options.isError ? (
        <LoadError error={options.error} onRetry={() => void options.refetch()} pending={options.isFetching} />
      ) : null}
      {options.data ? (
        <select
          name="parent_id"
          value={parentId == null ? "" : String(parentId)}
          onChange={(event) => {
            const value = event.target.value;
            onChange(value === "" ? null : Number(value));
          }}
        >
          {allowNone ? <option value="">不设置父级</option> : <option value="" disabled>选择父级</option>}
          {parentId != null && !known ? <option value={parentId}>{fallbackLabel ?? "已选择的父级"}</option> : null}
          {options.data.map((loc) => (
            <option key={loc.id} value={loc.id}>
              {optionPrefix(loc.path.length)}
              {formatPath(loc.path)}
            </option>
          ))}
        </select>
      ) : null}
      {error ? <span className={styles.error}>{error}</span> : null}
    </label>
  );
}

function ItemFields({
  draft,
  onChange,
  error,
  extrasOpen,
}: {
  draft: ItemDraft;
  onChange: (draft: ItemDraft) => void;
  error: ApiError | null;
  extrasOpen: boolean;
}) {
  // React 19 types omit defaultOpen on <details>; seed once so open={extrasOpen} cannot trap it.
  const [extrasShown, setExtrasShown] = useState(extrasOpen);
  function set<K extends keyof ItemDraft>(key: K, value: ItemDraft[K]) {
    onChange({ ...draft, [key]: value });
  }
  return (
    <>
      <TextField label="名称" name="name" value={draft.name} onChange={(value) => set("name", value)} error={fieldText(error, "name")} />
      <ItemLocationsField links={draft.locations} onChange={(locations) => set("locations", locations)} error={fieldText(error, "locations")} />
      <ItemCategoriesField
        links={draft.categories}
        onChange={(categories) => set("categories", categories)}
        error={fieldText(error, "categories")}
      />
      <details
        className={styles.filters}
        open={extrasShown}
        onToggle={(event) => setExtrasShown(event.currentTarget.open)}
      >
        <summary>更多说明</summary>
        <TextField label="别名" name="alias" value={draft.alias} onChange={(value) => set("alias", value)} error={fieldText(error, "alias")} />
        <TextField label="型号" name="model" value={draft.model} onChange={(value) => set("model", value)} error={fieldText(error, "model")} />
        <TextField label="规格" name="spec" value={draft.spec} onChange={(value) => set("spec", value)} error={fieldText(error, "spec")} />
        <TextField
          label="数量说明"
          name="quantity_note"
          value={draft.quantityNote}
          onChange={(value) => set("quantityNote", value)}
          error={fieldText(error, "quantity_note")}
        />
        <TextField
          label="备注"
          name="note"
          value={draft.note}
          onChange={(value) => set("note", value)}
          error={fieldText(error, "note")}
          multiline
        />
      </details>
    </>
  );
}

const emptyDraft: ItemDraft = {
  name: "",
  alias: "",
  model: "",
  spec: "",
  quantityNote: "",
  note: "",
  locations: [],
  categories: [],
};

function draftFromItem(item: Item): ItemDraft {
  return {
    name: item.name,
    alias: item.alias ?? "",
    model: item.model ?? "",
    spec: item.spec ?? "",
    quantityNote: item.quantity_note ?? "",
    note: item.note ?? "",
    locations: cloneLinks(item.locations),
    categories: cloneCategoryLinks(item.categories),
  };
}

const emptyItemList: ItemListState = {
  q: "",
  unlocated: false,
  inLocation: "",
  locationSelf: false,
  categoryIds: [],
  matchAll: false,
  categorySelf: false,
  uncategorized: false,
  offset: "",
};

function emptyCopy(state: ItemListState): string {
  const hasFilter =
    state.q.trim() !== "" ||
    state.unlocated ||
    state.inLocation !== "" ||
    state.categoryIds.length > 0 ||
    state.uncategorized;
  if (!hasFilter) return "还没有物品";
  if (
    state.unlocated &&
    state.q.trim() === "" &&
    state.categoryIds.length === 0 &&
    !state.uncategorized &&
    state.inLocation === ""
  ) {
    return "没有待定位的物品";
  }
  return "没有符合条件的物品";
}

export function ItemListPage() {
  const [params, setParams] = useSearchParams();
  const state = readItemList(params);
  const listParams = writeItemList(state);
  const view = readItemList(listParams);
  const listQuery = listParams.toString();
  const [qInput, setQInput] = useState(state.q);
  const [pickedCategory, setPickedCategory] = useState("");
  useEffect(() => {
    setQInput(state.q);
  }, [state.q]);
  const query = useQuery({
    queryKey: ["items", "list", listQuery],
    queryFn: () => listItems(new URLSearchParams(listQuery)),
    retry: false,
  });
  const locationsQuery = useQuery({
    queryKey: ["locations", "flat"],
    queryFn: () => fetchAllLocations(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  const categoriesQuery = useQuery({
    queryKey: ["categories", "flat"],
    queryFn: () => fetchAllCategories(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  useOnUnauth(query.error);
  useOnUnauth(locationsQuery.error);
  useOnUnauth(categoriesQuery.error);
  const page = query.data;
  const locations = locationsQuery.data ?? [];
  const categories = categoriesQuery.data ?? [];
  const chosenCategories = new Set(view.categoryIds);
  const availableCategories = categories.filter((cat) => !chosenCategories.has(String(cat.id)));
  const locationKnown = locations.some((loc) => String(loc.id) === view.inLocation);
  const hasFilter =
    view.q.trim() !== "" ||
    view.unlocated ||
    view.inLocation !== "" ||
    view.categoryIds.length > 0 ||
    view.uncategorized;

  function applyFilters(patch: Partial<ItemListState>) {
    setParams((current) => writeItemList({ ...readItemList(current), ...patch, offset: "" }));
  }

  function locationPath(id: string): string {
    const loc = locations.find((item) => String(item.id) === id);
    if (loc && loc.path.length > 0) return formatPath(loc.path);
    return id;
  }

  function categoryPath(id: string): string {
    const cat = categories.find((item) => String(item.id) === id);
    if (cat && cat.path.length > 0) return formatCategoryPath(cat.path);
    return id;
  }

  return (
    <>
      <h1 className={styles.title}>家里的物品</h1>
      <p className={styles.intro}>记下拥有的东西，找到它们的去处。</p>
      <form
        className={styles.searchRow}
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          applyFilters({ q: qInput });
        }}
      >
        <input
          type="search"
          name="q"
          value={qInput}
          autoComplete="off"
          aria-label="找家里的东西"
          placeholder="找家里的东西"
          onChange={(event) => setQInput(event.target.value)}
        />
        <button className={styles.buttonPrimary} type="submit">
          查找
        </button>
        <Link className={styles.button} to="/items/new">
          ＋ 新增物品
        </Link>
      </form>
      {view.unlocated ? (
        <button className={styles.button} type="button" onClick={() => applyFilters({ unlocated: false })}>
          全部物品
        </button>
      ) : (
        <button className={styles.button} type="button" onClick={() => applyFilters({ unlocated: true })}>
          只看待定位
        </button>
      )}
      <details className={styles.filters}>
        <summary>筛选</summary>
        <label className={styles.field}>
          位置范围
          {locationsQuery.isLoading ? <Loading /> : null}
          {locationsQuery.isError ? (
            <LoadError
              error={locationsQuery.error}
              onRetry={() => void locationsQuery.refetch()}
              pending={locationsQuery.isFetching}
            />
          ) : null}
          {locationsQuery.data ? (
            <select
              name="in_location"
              value={view.inLocation}
              onChange={(event) => {
                const inLocation = event.target.value;
                applyFilters({
                  inLocation,
                  unlocated: false,
                  locationSelf: inLocation === "" ? false : view.locationSelf,
                });
              }}
            >
              <option value="">不限位置</option>
              {view.inLocation !== "" && !locationKnown ? (
                <option value={view.inLocation}>{view.inLocation}</option>
              ) : null}
              {locations.map((loc) => (
                <option key={loc.id} value={loc.id}>
                  {optionPrefix(loc.path.length)}
                  {formatPath(loc.path)}
                </option>
              ))}
            </select>
          ) : null}
        </label>
        {view.inLocation !== "" ? (
          <label className={styles.filterChoice}>
            <input
              type="checkbox"
              name="in_location_descendants"
              checked={view.locationSelf}
              onChange={(event) => applyFilters({ locationSelf: event.target.checked })}
            />
            仅当前位置
          </label>
        ) : null}
        <div className={styles.field}>
          <span>分类</span>
          {view.categoryIds.map((id) => (
            <div key={id}>
              <span className={styles.path}>{categoryPath(id)}</span>
              <button
                className={styles.button}
                type="button"
                onClick={() => applyFilters({ categoryIds: view.categoryIds.filter((item) => item !== id) })}
              >
                移除
              </button>
            </div>
          ))}
          {categoriesQuery.isLoading ? <Loading /> : null}
          {categoriesQuery.isError ? (
            <LoadError
              error={categoriesQuery.error}
              onRetry={() => void categoriesQuery.refetch()}
              pending={categoriesQuery.isFetching}
            />
          ) : null}
          {categoriesQuery.data ? (
            <div className={styles.stack}>
              <select name="category_id" value={pickedCategory} onChange={(event) => setPickedCategory(event.target.value)}>
                <option value="">选择分类</option>
                {availableCategories.map((cat) => (
                  <option key={cat.id} value={cat.id}>
                    {optionPrefix(cat.path.length)}
                    {formatCategoryPath(cat.path)}
                  </option>
                ))}
              </select>
              <button
                className={styles.button}
                type="button"
                disabled={pickedCategory === ""}
                onClick={() => {
                  if (pickedCategory === "" || view.categoryIds.includes(pickedCategory)) return;
                  applyFilters({
                    categoryIds: [...view.categoryIds, pickedCategory],
                    uncategorized: false,
                  });
                  setPickedCategory("");
                }}
              >
                添加
              </button>
            </div>
          ) : null}
        </div>
        {view.categoryIds.length > 0 ? (
          <label className={styles.filterChoice}>
            <input
              type="checkbox"
              name="category_descendants"
              checked={view.categorySelf}
              onChange={(event) => applyFilters({ categorySelf: event.target.checked })}
            />
            仅当前分类
          </label>
        ) : null}
        {view.categoryIds.length >= 2 ? (
          <div className={styles.filterChoice}>
            <label className={styles.filterChoice}>
              <input
                type="radio"
                name="category_match"
                value="any"
                checked={!view.matchAll}
                onChange={() => applyFilters({ matchAll: false })}
              />
              任意匹配
            </label>
            <label className={styles.filterChoice}>
              <input
                type="radio"
                name="category_match"
                value="all"
                checked={view.matchAll}
                onChange={() => applyFilters({ matchAll: true })}
              />
              全部匹配
            </label>
          </div>
        ) : null}
        <label className={styles.filterChoice}>
          <input
            type="checkbox"
            name="placement"
            checked={view.unlocated}
            onChange={(event) => applyFilters({ unlocated: event.target.checked })}
          />
          待定位
        </label>
        <label className={styles.filterChoice}>
          <input
            type="checkbox"
            name="uncategorized"
            checked={view.uncategorized}
            onChange={(event) => applyFilters({ uncategorized: event.target.checked })}
          />
          未分类
        </label>
      </details>
      {hasFilter ? (
        <div className={styles.filterSummary}>
          {view.q.trim() !== "" ? <p>关键词：{view.q.trim()}</p> : null}
          {view.inLocation !== "" ? (
            <p>
              位置：<span className={styles.path}>{locationPath(view.inLocation)}</span>
              {view.locationSelf ? "仅当前" : "含下级"}
            </p>
          ) : null}
          {view.categoryIds.length > 0 ? (
            <div>
              <p>
                分类
                {view.categoryIds.length >= 2 ? (view.matchAll ? "（全部匹配）" : "（任意匹配）") : null}
                {view.categorySelf ? "（仅当前）" : "（含下级）"}
              </p>
              {view.categoryIds.map((id) => (
                <span key={id} className={styles.path}>
                  {categoryPath(id)}
                </span>
              ))}
            </div>
          ) : null}
          {view.uncategorized ? <p>未分类</p> : null}
          {view.unlocated ? <p>待定位</p> : null}
          <button
            className={styles.button}
            type="button"
            onClick={() => {
              setQInput("");
              setPickedCategory("");
              setParams(writeItemList(emptyItemList));
            }}
          >
            清空筛选
          </button>
        </div>
      ) : null}
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>{emptyCopy(view)}</p> : null}
      {page && page.total > 0 ? <p className={styles.resultCount}>共 {page.total} 件物品</p> : null}
      {page && page.data.length > 0 ? (
        <ul className={styles.list}>
          {page.data.map((item) => (
            <li key={item.id}>
              <Link className={styles.itemLink} to={`/items/${item.id}`}>
                <ItemThumb photoId={item.photos[0]?.id} />
                <span className={styles.itemLinkBody}>
                  <span>{item.name}</span>
                  {item.locations.length === 0 ? (
                    <span className={styles.meta}>待定位</span>
                  ) : (
                    item.locations.map((link) => (
                      <LocationPathLine key={link.location_id} path={link.path} />
                    ))
                  )}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
      {page ? (
        <Pager
          offset={page.offset}
          limit={page.limit}
          total={page.total}
          count={page.data.length}
          onPage={(offset) =>
            setParams((current) =>
              writeItemList({ ...readItemList(current), offset: offset > 0 ? String(offset) : "" }),
            )
          }
        />
      ) : null}
    </>
  );
}

export function ItemCreatePage() {
  const [params] = useSearchParams();
  const locationParam = params.get("location") ?? "";
  const categoryParam = params.get("category") ?? "";
  return (
    <ItemCreateForm
      key={`${locationParam}|${categoryParam}`}
      locationParam={locationParam}
      categoryParam={categoryParam}
    />
  );
}

function ItemCreateForm({ locationParam, categoryParam }: { locationParam: string; categoryParam: string }) {
  const navigate = useNavigate();
  const hasLocationPreset = locationParam !== "";
  const hasCategoryPreset = categoryParam !== "";
  const hasPreset = hasLocationPreset || hasCategoryPreset;
  const preset = useQuery({
    queryKey: ["location", "preset", locationParam],
    queryFn: () => getLocation(locationParam),
    enabled: hasLocationPreset,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const categoryPreset = useQuery({
    queryKey: ["category", "preset", categoryParam],
    queryFn: () => getCategory(categoryParam),
    enabled: hasCategoryPreset,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const [formReady, setFormReady] = useState(!hasPreset);
  const [draft, setDraft] = useState<ItemDraft>(emptyDraft);
  const [formError, setFormError] = useState<ApiError | null>(null);

  useEffect(() => {
    if (formReady) return;
    if (hasLocationPreset && (!preset.isFetchedAfterMount || preset.isFetching)) return;
    if (hasCategoryPreset && (!categoryPreset.isFetchedAfterMount || categoryPreset.isFetching)) return;
    setDraft((current) => {
      let next = current;
      if (hasLocationPreset && preset.isSuccess && preset.data) {
        const presetLocation = preset.data;
        next = {
          ...next,
          locations: [
            {
              location_id: presetLocation.id,
              note: null,
              path: presetLocation.path.map((node) => ({ ...node })),
            },
          ],
        };
      }
      if (hasCategoryPreset && categoryPreset.isSuccess && categoryPreset.data) {
        const presetCategory = categoryPreset.data;
        next = {
          ...next,
          categories: [
            {
              category_id: presetCategory.id,
              source: "human",
              path: presetCategory.path.map((node) => ({ ...node })),
            },
          ],
        };
      }
      return next;
    });
    setFormReady(true);
  }, [
    formReady,
    hasLocationPreset,
    hasCategoryPreset,
    preset.isFetchedAfterMount,
    preset.isFetching,
    preset.isSuccess,
    preset.data,
    categoryPreset.isFetchedAfterMount,
    categoryPreset.isFetching,
    categoryPreset.isSuccess,
    categoryPreset.data,
  ]);

  const create = useMutation({
    mutationFn: (body: ItemCreate) => createItem(body),
    onSuccess: (item) => navigate(`/items/${item.id}`),
    onError: (error) => setFormError(asApiError(error, "无法新增物品")),
  });
  useOnUnauth(preset.error);
  useOnUnauth(categoryPreset.error);
  useOnUnauth(create.error);

  async function retryPreset() {
    const result = await preset.refetch();
    if (!result.isSuccess) return;
    const presetLocation = result.data;
    if (!presetLocation) return;
    setDraft((current) => {
      if (current.locations.length > 0) return current;
      return {
        ...current,
        locations: [
          {
            location_id: presetLocation.id,
            note: null,
            path: presetLocation.path.map((node) => ({ ...node })),
          },
        ],
      };
    });
  }

  async function retryCategoryPreset() {
    const result = await categoryPreset.refetch();
    if (!result.isSuccess) return;
    const presetCategory = result.data;
    if (!presetCategory) return;
    setDraft((current) => {
      if (current.categories.length > 0) return current;
      return {
        ...current,
        categories: [
          {
            category_id: presetCategory.id,
            source: "human",
            path: presetCategory.path.map((node) => ({ ...node })),
          },
        ],
      };
    });
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setFormError(null);
    create.mutate(itemBody(draft));
  }

  if (!formReady) return <Loading />;

  return (
    <>
      <h1 className={styles.title}>新增物品</h1>
      {hasLocationPreset && preset.isError ? (
        <div>
          <p className={styles.error}>{messageOf(preset.error, "无法读取位置")}</p>
          <button className={styles.button} type="button" onClick={() => void retryPreset()} disabled={preset.isFetching}>
            重试
          </button>
        </div>
      ) : null}
      {hasCategoryPreset && categoryPreset.isError ? (
        <div>
          <p className={styles.error}>{messageOf(categoryPreset.error, "无法读取分类")}</p>
          <button
            className={styles.button}
            type="button"
            onClick={() => void retryCategoryPreset()}
            disabled={categoryPreset.isFetching}
          >
            重试
          </button>
        </div>
      ) : null}
      <form onSubmit={onSubmit}>
        <ItemFields draft={draft} onChange={setDraft} error={formError} extrasOpen={false} />
        <FormIssues error={formError} />
        <button className={styles.buttonPrimary} type="submit" disabled={create.isPending}>
          保存
        </button>
      </form>
    </>
  );
}

function ReturnTaskFields({ task, showItemName, showCreatedAt }: { task: ReturnTask; showItemName?: boolean; showCreatedAt?: boolean }) {
  return (
    <>
      {showItemName ? (
        <Link className={styles.itemLink} to={`/items/${task.item_id}`}>
          <ItemThumb photoId={task.cover_photo?.id} />
          <span className={styles.itemLinkBody}>{task.item_name}</span>
        </Link>
      ) : null}
      <p>{partLabel(task)}</p>
      {task.reason ? <p>原因：{task.reason}</p> : null}
      {task.destination_note ? <p>临时去向：{task.destination_note}</p> : null}
      {showCreatedAt ? <p>建立时间：{task.created_at}</p> : null}
    </>
  );
}

function OpenReturnTaskRow({
  task,
  showItemName,
  showCreatedAt,
  onCompleteSuccess,
  onRemoveSuccess,
  onNotFound,
  onStale,
}: {
  task: ReturnTask;
  showItemName?: boolean;
  showCreatedAt?: boolean;
  onCompleteSuccess: (task: ReturnTask) => void;
  onRemoveSuccess: (id: number) => void;
  onNotFound: () => void | Promise<void>;
  onStale: (message: string) => void;
}) {
  const [confirming, setConfirming] = useState(false);
  const [rowError, setRowError] = useState<ApiError | null>(null);
  const complete = useMutation({
    mutationFn: () => completeReturnTask(task.id, task.version),
    onSuccess: (next) => {
      setRowError(null);
      onCompleteSuccess(next);
    },
    onError: (error) => {
      if (isNotFound(error)) {
        void onNotFound();
        return;
      }
      if (isVersionConflict(error) || isAlreadyCompleted(error)) {
        onStale(messageOf(error, "无法完成归位"));
        return;
      }
      setRowError(asApiError(error, "无法完成归位"));
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteReturnTask(task.id, task.version),
    onSuccess: () => {
      setRowError(null);
      setConfirming(false);
      onRemoveSuccess(task.id);
    },
    onError: (error) => {
      if (isNotFound(error)) {
        void onNotFound();
        return;
      }
      if (isVersionConflict(error) || isAlreadyCompleted(error)) {
        setConfirming(false);
        onStale(messageOf(error, "无法去掉事项"));
        return;
      }
      setRowError(asApiError(error, "无法去掉事项"));
    },
  });
  useOnUnauth(complete.error);
  useOnUnauth(remove.error);

  return (
    <li>
      <ReturnTaskFields task={task} showItemName={showItemName} showCreatedAt={showCreatedAt} />
      <div className={styles.actions}>
        <button
          className={styles.button}
          type="button"
          onClick={() => {
            setRowError(null);
            complete.mutate();
          }}
          disabled={complete.isPending || remove.isPending}
        >
          完成归位
        </button>
        <DeleteConfirm
          confirming={confirming}
          pending={remove.isPending}
          error={rowError}
          askLabel="去掉"
          warning="去掉这条提醒"
          confirmLabel="确认去掉"
          onAsk={() => {
            setConfirming(true);
            setRowError(null);
          }}
          onCancel={() => {
            setConfirming(false);
            setRowError(null);
          }}
          onConfirm={() => remove.mutate()}
        />
      </div>
      {!confirming && rowError ? <FormIssues error={rowError} /> : null}
    </li>
  );
}

export function ItemEditPage() {
  const params = useParams();
  const id = params.id ?? "";
  return <ItemEditForm key={id} id={id} />;
}

function ItemEditForm({ id }: { id: string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["item", id],
    queryFn: () => getItem(id),
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const [formReady, setFormReady] = useState(false);
  const [draft, setDraft] = useState<ItemDraft>(emptyDraft);
  const [version, setVersion] = useState(0);
  const [conflict, setConflict] = useState(false);
  const [conflictMessage, setConflictMessage] = useState("");
  const [latestError, setLatestError] = useState("");
  const [loadingLatest, setLoadingLatest] = useState(false);
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [deleteError, setDeleteError] = useState<ApiError | null>(null);
  const [originalCategoryIds, setOriginalCategoryIds] = useState<number[]>([]);
  const [partNote, setPartNote] = useState("");
  const [reason, setReason] = useState("");
  const [destinationNote, setDestinationNote] = useState("");
  const [taskFormError, setTaskFormError] = useState<ApiError | null>(null);
  const [taskNotice, setTaskNotice] = useState("");
  const [taskVerifyError, setTaskVerifyError] = useState("");
  const [taskActionError, setTaskActionError] = useState("");
  const recordReady = query.isSuccess && query.isFetchedAfterMount && !query.isFetching && query.data != null;

  useEffect(() => {
    if (formReady || !recordReady || !query.data) return;
    setDraft(draftFromItem(query.data));
    setVersion(query.data.version);
    setOriginalCategoryIds(categoryIdsOf(query.data.categories));
    setFormReady(true);
  }, [formReady, recordReady, query.data]);

  function applyServer(next: Item) {
    setDraft(draftFromItem(next));
    setVersion(next.version);
    setOriginalCategoryIds(categoryIdsOf(next.categories));
    setConflict(false);
  }

  const save = useMutation({
    mutationFn: (body: ItemUpdate) => updateItem(id, body),
    onSuccess: (next) => {
      queryClient.setQueryData(["item", id], next);
      applyServer(next);
      setFormError(null);
    },
    onError: (error) => {
      if (isNotFound(error)) {
        navigate("/", { replace: true, state: { notice: "未找到" } });
        return;
      }
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setFormError(asApiError(error, "无法保存物品"));
    },
  });
  const remove = useMutation({
    mutationFn: (ver: number) => deleteItem(id, ver),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["item", id] });
      void queryClient.invalidateQueries({ queryKey: ["items"] });
      void queryClient.invalidateQueries({ queryKey: ["locations"] });
      void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
      void queryClient.invalidateQueries({ queryKey: ["categories"] });
      void queryClient.invalidateQueries({ queryKey: ["trash"] });
      navigate("/");
    },
    onError: (error) => {
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setDeleteError(asApiError(error, "无法删除物品"));
    },
  });
  useOnUnauth(query.error);
  useOnUnauth(save.error);
  useOnUnauth(remove.error);

  async function handleTaskNotFound() {
    try {
      const latest = await getItem(id);
      queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
        cur ? { ...cur, return_tasks: latest.return_tasks } : cur,
      );
      setTaskNotice("该事项已不存在");
      setTaskVerifyError("");
      setTaskActionError("");
    } catch (error) {
      if (isNotFound(error)) {
        navigate("/", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setTaskVerifyError(messageOf(error, "无法确认物品"));
    }
  }

  async function refreshTaskRow(message: string) {
    setTaskActionError(message);
    try {
      const latest = await getItem(id);
      queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
        cur ? { ...cur, return_tasks: latest.return_tasks } : cur,
      );
    } catch (error) {
      if (isNotFound(error)) {
        navigate("/", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setTaskVerifyError(messageOf(error, "无法确认物品"));
    }
  }

  const addTask = useMutation({
    mutationFn: (body: ReturnTaskCreate) => createReturnTask(id, body),
    onSuccess: (task) => {
      queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
        cur ? { ...cur, return_tasks: mergeTask(cur.return_tasks, task) } : cur,
      );
      setPartNote("");
      setReason("");
      setDestinationNote("");
      setTaskFormError(null);
      setTaskNotice("");
      setTaskVerifyError("");
      setTaskActionError("");
      void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
    },
    onError: (error) => {
      if (isNotFound(error)) {
        void handleTaskNotFound();
        return;
      }
      setTaskFormError(asApiError(error, "无法新增归位事项"));
    },
  });
  useOnUnauth(addTask.error);

  async function loadLatest() {
    setLoadingLatest(true);
    setLatestError("");
    try {
      const next = await getItem(id);
      queryClient.setQueryData(["item", id], next);
      applyServer(next);
      setConfirming(false);
      setFormError(null);
      setDeleteError(null);
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      if (isNotFound(error)) {
        navigate("/", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setLatestError(messageOf(error, "无法读取"));
    } finally {
      setLoadingLatest(false);
    }
  }

  if (!formReady) {
    if (query.isError) {
      if (isNotFound(query.error)) {
        return <Navigate to="/" replace state={{ notice: "未找到" }} />;
      }
      return <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} />;
    }
    return <Loading />;
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setFormError(null);
    save.mutate({ ...itemBody(draft, originalCategoryIds), version });
  }

  const extrasOpen = [draft.alias, draft.model, draft.spec, draft.quantityNote, draft.note].some((v) => v !== "");

  return (
    <>
      <h1 className={styles.title}>{draft.name}</h1>
      <form onSubmit={onSubmit}>
        <ItemFields draft={draft} onChange={setDraft} error={formError} extrasOpen={extrasOpen} />
        <FormIssues error={formError} />
        {conflict ? (
          <ConflictNotice message={conflictMessage} pending={loadingLatest} onLoad={() => void loadLatest()} error={latestError} />
        ) : null}
        <button className={styles.buttonPrimary} type="submit" disabled={save.isPending}>
          保存
        </button>
      </form>
      <ItemPhotoSection itemId={id} photos={query.data?.photos ?? []} />
      <section>
        <h2 className={styles.sectionTitle}>待归位事项</h2>
        {taskNotice ? <p>{taskNotice}</p> : null}
        {taskActionError ? <p className={styles.error}>{taskActionError}</p> : null}
        {taskVerifyError ? <p className={styles.error}>{taskVerifyError}</p> : null}
        {(query.data?.return_tasks ?? []).length > 0 ? (
          <ul className={styles.list}>
            {(query.data?.return_tasks ?? []).map((task) =>
              task.completed_at ? (
                <li key={task.id}>
                  <ReturnTaskFields task={task} showCreatedAt />
                  <p>已归位</p>
                  <p>{task.completed_at}</p>
                </li>
              ) : (
                <OpenReturnTaskRow
                  key={task.id}
                  task={task}
                  showCreatedAt
                  onCompleteSuccess={(next) => {
                    queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
                      cur ? { ...cur, return_tasks: mergeTask(cur.return_tasks, next) } : cur,
                    );
                    setTaskNotice("");
                    setTaskVerifyError("");
                    setTaskActionError("");
                    void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
                  }}
                  onRemoveSuccess={(taskId) => {
                    queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
                      cur ? { ...cur, return_tasks: cur.return_tasks.filter((item) => item.id !== taskId) } : cur,
                    );
                    setTaskNotice("");
                    setTaskVerifyError("");
                    setTaskActionError("");
                    void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
                  }}
                  onNotFound={() => handleTaskNotFound()}
                  onStale={(message) => void refreshTaskRow(message)}
                />
              ),
            )}
          </ul>
        ) : null}
        <h3 className={styles.sectionTitle}>新增归位事项</h3>
        <form
          onSubmit={(event: FormEvent) => {
            event.preventDefault();
            setTaskFormError(null);
            addTask.mutate({
              part_note: emptyToNull(partNote),
              reason: emptyToNull(reason),
              destination_note: emptyToNull(destinationNote),
            });
          }}
        >
          <TextField
            label="配件说明"
            name="part_note"
            value={partNote}
            onChange={setPartNote}
            hint="整件则留空"
            error={fieldText(taskFormError, "part_note")}
          />
          <TextField label="原因" name="reason" value={reason} onChange={setReason} error={fieldText(taskFormError, "reason")} />
          <TextField
            label="临时去向"
            name="destination_note"
            value={destinationNote}
            onChange={setDestinationNote}
            multiline
            error={fieldText(taskFormError, "destination_note")}
          />
          <FormIssues error={taskFormError} />
          <button className={styles.button} type="submit" disabled={addTask.isPending}>
            添加
          </button>
        </form>
      </section>
      <DeleteConfirm
        confirming={confirming}
        pending={remove.isPending}
        error={deleteError}
        warning="将移到回收站"
        onAsk={() => {
          setConfirming(true);
          setDeleteError(null);
        }}
        onCancel={() => {
          setConfirming(false);
          setDeleteError(null);
        }}
        onConfirm={() => remove.mutate(version)}
      />
    </>
  );
}

export function ReturnListPage() {
  const [params, setParams] = useSearchParams();
  const offsetRaw = params.get("offset");
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [notice, setNotice] = useState("");
  const [verifyError, setVerifyError] = useState("");
  const query = useQuery({
    queryKey: ["return-tasks", offsetRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams();
      if (offsetRaw) api.set("offset", offsetRaw);
      return listReturnTasks(api);
    },
    retry: false,
  });
  useOnUnauth(query.error);
  const page = query.data;

  useEffect(() => {
    if (!page) return;
    const currentOffset = Number(offsetRaw || "0");
    const clamped = clampPageOffset(page.total, currentOffset, page.limit > 0 ? page.limit : 30);
    if (clamped !== currentOffset) {
      setParams((current) => setOffsetParam(current, "offset", clamped));
    }
  }, [page, offsetRaw, setParams]);

  async function reloadReturnList() {
    try {
      const currentOffset = Number(params.get("offset") || "0");
      const api = new URLSearchParams();
      if (currentOffset > 0) api.set("offset", String(currentOffset));
      const next = await listReturnTasks(api);
      const clamped = clampPageOffset(next.total, currentOffset, next.limit > 0 ? next.limit : 30);
      if (clamped !== currentOffset) {
        setParams((current) => setOffsetParam(current, "offset", clamped));
        return;
      }
      queryClient.setQueryData(["return-tasks", offsetRaw ?? ""], next);
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      setVerifyError(messageOf(error, "无法读取待归位事项"));
    }
  }

  async function handleListTaskNotFound(itemId: number) {
    try {
      await getItem(String(itemId));
      setNotice("该事项已不存在");
      setVerifyError("");
      await reloadReturnList();
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      if (isNotFound(error)) {
        setNotice("未找到");
        setVerifyError("");
        await reloadReturnList();
        return;
      }
      setVerifyError(messageOf(error, "无法确认物品"));
    }
  }

  return (
    <>
      <h1 className={styles.title}>待归位</h1>
      {notice ? <p>{notice}</p> : null}
      {verifyError ? <p className={styles.error}>{verifyError}</p> : null}
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>没有待归位事项</p> : null}
      {page && page.data.length > 0 ? (
        <ul className={styles.list}>
          {page.data.map((task) => (
            <OpenReturnTaskRow
              key={task.id}
              task={task}
              showItemName
              onCompleteSuccess={() => {
                setNotice("");
                setVerifyError("");
                void reloadReturnList();
              }}
              onRemoveSuccess={() => {
                setNotice("");
                setVerifyError("");
                void reloadReturnList();
              }}
              onNotFound={() => handleListTaskNotFound(task.item_id)}
              onStale={(message) => {
                setNotice(message);
                void reloadReturnList();
              }}
            />
          ))}
        </ul>
      ) : null}
      {page ? (
        <Pager
          offset={page.offset}
          limit={page.limit}
          total={page.total}
          count={page.data.length}
          onPage={(offset) => setParams((current) => setOffsetParam(current, "offset", offset))}
        />
      ) : null}
    </>
  );
}

function LocationLinks({ rows }: { rows: Location[] }) {
  return (
    <ul className={`${styles.list} ${styles.treeList}`}>
      {rows.map((row) => (
        <li
          key={row.id}
          className={treeDepth(row.path.length) > 0 ? styles.treeChild : undefined}
          style={treeStyle(row.path.length)}
        >
          <Link className={styles.itemLink} to={`/locations/${row.id}`}>
            <LocationIcon name={resolvedIcon(row)} className={styles.rowIcon} />
            <span className={styles.itemLinkBody}>
              <span>{row.name}</span>
              {row.code ? <span className={styles.codeBadge}>{row.code}</span> : null}
              {row.direct_item_count > 0 ? (
                <span className={styles.countMuted}>{row.direct_item_count} 件</span>
              ) : null}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function LocationGroups({ rows }: { rows: Location[] }) {
  const areas = rows.filter((row) => row.type === "area");
  const containers = rows.filter((row) => row.type !== "area");
  return (
    <>
      {areas.length > 0 ? (
        <section>
          <h2 className={styles.sectionTitle}>区域</h2>
          <LocationLinks rows={areas} />
        </section>
      ) : null}
      {containers.length > 0 ? (
        <section>
          <h2 className={styles.sectionTitle}>尚未放入的容器</h2>
          <LocationLinks rows={containers} />
        </section>
      ) : null}
    </>
  );
}

export function LocationListPage() {
  const [params, setParams] = useSearchParams();
  const offsetRaw = params.get("offset");
  const query = useQuery({
    queryKey: ["locations", "roots", offsetRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams();
      if (offsetRaw) api.set("offset", offsetRaw);
      return listLocations(api);
    },
    retry: false,
  });
  useOnUnauth(query.error);
  const page = query.data;

  return (
    <>
      <h1 className={styles.title}>位置</h1>
      <div className={styles.actions}>
        <Link className={styles.buttonPrimary} to="/locations/new">
          新增
        </Link>
      </div>
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>还没有位置</p> : null}
      {page && page.data.length > 0 ? <LocationGroups rows={page.data} /> : null}
      {page ? (
        <Pager
          offset={page.offset}
          limit={page.limit}
          total={page.total}
          count={page.data.length}
          onPage={(offset) => setParams((current) => setOffsetParam(current, "offset", offset))}
        />
      ) : null}
    </>
  );
}

export function LocationCreatePage() {
  const [params] = useSearchParams();
  const parentParam = params.get("parent") ?? "";
  return <LocationCreateForm key={parentParam} parentParam={parentParam} />;
}

function LocationCreateForm({ parentParam }: { parentParam: string }) {
  const navigate = useNavigate();
  const hasParent = parentParam !== "";
  const parentQuery = useQuery({
    queryKey: ["location", "parent", parentParam],
    queryFn: () => getLocation(parentParam),
    enabled: hasParent,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnMount: "always",
  });
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [type, setType] = useState<LocationType | "">("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [icon, setIcon] = useState<string | null>(null);
  const [formError, setFormError] = useState<ApiError | null>(null);

  useEffect(() => {
    if (!hasParent || type !== "") return;
    if (!parentQuery.isFetchedAfterMount || parentQuery.isFetching) return;
    if (!parentQuery.data) return;
    const allowed = childTypes(parentQuery.data.type);
    if (!allowed[0]) return;
    setType(allowed[0]);
    setParentId(parentQuery.data.id);
  }, [hasParent, type, parentQuery.isFetchedAfterMount, parentQuery.isFetching, parentQuery.data]);

  const create = useMutation({
    mutationFn: (body: LocationCreate) => createLocation(body),
    onSuccess: (loc) => navigate(`/locations/${loc.id}`),
    onError: (error) => setFormError(asApiError(error, "无法新增位置")),
  });
  useOnUnauth(parentQuery.error);
  useOnUnauth(create.error);

  function changeType(value: string) {
    if (!isLocationType(value)) {
      if (!hasParent) {
        setType("");
        setParentId(null);
      }
      return;
    }
    if (hasParent && parentQuery.data && !childTypes(parentQuery.data.type).includes(value)) return;
    setType(value);
    if (!hasParent) setParentId(null);
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (hasParent && !parentQuery.data) return;
    if (!isLocationType(type)) {
      setFormError(new ApiError("invalid_fields", "有字段不符合要求", { type: "类型不正确" }));
      return;
    }
    setFormError(null);
    const chosenParent = hasParent ? (parentQuery.data?.id ?? null) : parentId;
    create.mutate(locationCreateBody(type, name, code, chosenParent, icon));
  }

  if (hasParent && parentQuery.isError) {
    return (
      <>
        <h1 className={styles.title}>新增位置</h1>
        <LoadError error={parentQuery.error} onRetry={() => void parentQuery.refetch()} pending={parentQuery.isFetching} />
        <Link className={styles.button} to="/locations/new">
          重新选择父级
        </Link>
      </>
    );
  }
  if (hasParent && parentQuery.isSuccess && childTypes(parentQuery.data.type).length === 0) {
    return (
      <>
        <h1 className={styles.title}>新增位置</h1>
        <p className={styles.error}>不能放在这个父级下</p>
        <Link className={styles.button} to="/locations/new">
          重新选择父级
        </Link>
      </>
    );
  }
  if (hasParent && (!parentQuery.data || !isLocationType(type))) return <Loading />;

  const typeOptions = hasParent && parentQuery.data ? childTypes(parentQuery.data.type) : allTypes;

  return (
    <>
      <h1 className={styles.title}>新增位置</h1>
      <form onSubmit={onSubmit}>
        <label className={styles.field}>
          类型
          <select name="type" value={type} onChange={(event) => changeType(event.target.value)}>
            {hasParent ? null : <option value="">选择类型</option>}
            {typeOptions.map((item) => (
              <option key={item} value={item}>
                {typeLabel[item]}
              </option>
            ))}
          </select>
          {fieldText(formError, "type") ? <span className={styles.error}>{fieldText(formError, "type")}</span> : null}
        </label>
        <TextField label="名称" name="name" value={name} onChange={setName} error={fieldText(formError, "name")} />
        {type === "fixed" || type === "movable" ? (
          <TextField label="编号" name="code" value={code} onChange={setCode} error={fieldText(formError, "code")} />
        ) : null}
        {hasParent && parentQuery.data ? (
          <div className={styles.field}>
            <span>父级</span>
            <span className={styles.path}>{formatPath(parentQuery.data.path)}</span>
            <Link className={styles.button} to="/locations/new">
              重新选择父级
            </Link>
            {fieldText(formError, "parent_id") ? <span className={styles.error}>{fieldText(formError, "parent_id")}</span> : null}
          </div>
        ) : null}
        {!hasParent && isLocationType(type) ? (
          <ParentField
            type={type}
            parentId={parentId}
            onChange={setParentId}
            allowNone={type !== "fixed"}
            error={fieldText(formError, "parent_id")}
          />
        ) : null}
        <IconPicker value={icon} type={type} onChange={setIcon} error={fieldText(formError, "icon")} />
        <FormIssues error={formError} />
        <button className={styles.button} type="submit" disabled={create.isPending}>
          保存
        </button>
      </form>
    </>
  );
}

export function LocationPage() {
  const params = useParams();
  const id = params.id ?? "";
  return <LocationScreen key={id} id={id} />;
}

function LocationScreen({ id }: { id: string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const childrenRaw = params.get("children");
  const itemsRaw = params.get("items");
  const loc = useQuery({
    queryKey: ["location", id],
    queryFn: () => getLocation(id),
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const children = useQuery({
    queryKey: ["locations", "children", id, childrenRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams({ parent: id });
      if (childrenRaw) api.set("offset", childrenRaw);
      return listLocations(api);
    },
    enabled: loc.isSuccess,
    retry: false,
  });
  const directItems = useQuery({
    queryKey: ["items", "at", id, itemsRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams({ location: id });
      if (itemsRaw) api.set("offset", itemsRaw);
      return listItems(api);
    },
    enabled: loc.isSuccess,
    retry: false,
  });
  const [formReady, setFormReady] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [icon, setIcon] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  const [conflict, setConflict] = useState(false);
  const [conflictMessage, setConflictMessage] = useState("");
  const [latestError, setLatestError] = useState("");
  const [loadingLatest, setLoadingLatest] = useState(false);
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [deleteError, setDeleteError] = useState<ApiError | null>(null);
  const [quickName, setQuickName] = useState("");
  const recordReady = loc.isSuccess && loc.isFetchedAfterMount && !loc.isFetching && loc.data != null;

  useEffect(() => {
    if (formReady || !recordReady || !loc.data) return;
    setName(loc.data.name);
    setCode(loc.data.code ?? "");
    setParentId(loc.data.parent_id);
    setIcon(loc.data.icon);
    setVersion(loc.data.version);
    setFormReady(true);
  }, [formReady, recordReady, loc.data]);

  function applyServer(next: Location) {
    setName(next.name);
    setCode(next.code ?? "");
    setParentId(next.parent_id);
    setIcon(next.icon);
    setVersion(next.version);
    setConflict(false);
  }

  const save = useMutation({
    mutationFn: (body: LocationUpdate) => updateLocation(id, body),
    onSuccess: (next) => {
      queryClient.setQueryData(["location", id], next);
      applyServer(next);
      setFormError(null);
    },
    onError: (error) => {
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setFormError(asApiError(error, "无法保存位置"));
    },
  });
  const remove = useMutation({
    mutationFn: (ver: number) => deleteLocation(id, ver),
    onSuccess: () => {
      const current = queryClient.getQueryData<Location>(["location", id]);
      const parent = current?.parent_id ?? null;
      queryClient.removeQueries({ queryKey: ["location", id] });
      void queryClient.invalidateQueries({ queryKey: ["items"] });
      void queryClient.invalidateQueries({ queryKey: ["locations"] });
      navigate(parent != null ? `/locations/${parent}` : "/locations");
    },
    onError: (error) => {
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setDeleteError(asApiError(error, "无法删除位置"));
    },
  });
  const quick = useMutation({
    mutationFn: (input: { name: string; locationId: number }) =>
      createItem({ name: input.name, locations: [{ location_id: input.locationId }] }),
    onSuccess: async () => {
      setQuickName("");
      await queryClient.invalidateQueries({ queryKey: ["items", "at", id] });
      await queryClient.invalidateQueries({ queryKey: ["location", id] });
    },
  });
  useOnUnauth(loc.error);
  useOnUnauth(children.error);
  useOnUnauth(directItems.error);
  useOnUnauth(save.error);
  useOnUnauth(remove.error);
  useOnUnauth(quick.error);

  async function loadLatest() {
    setLoadingLatest(true);
    setLatestError("");
    try {
      const next = await getLocation(id);
      queryClient.setQueryData(["location", id], next);
      applyServer(next);
      setConfirming(false);
      setFormError(null);
      setDeleteError(null);
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      if (isNotFound(error)) {
        navigate("/locations", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setLatestError(messageOf(error, "无法读取"));
    } finally {
      setLoadingLatest(false);
    }
  }

  if (!formReady) {
    if (loc.isError) {
      return <LoadError error={loc.error} onRetry={() => void loc.refetch()} pending={loc.isFetching} />;
    }
    return <Loading />;
  }
  if (!loc.data) return <Loading />;

  const record = loc.data;
  const heading = record.type !== "area" && code !== "" ? `${name} ${code}` : name;
  const canDelete = children.isSuccess && children.data.total === 0 && record.direct_item_count === 0;
  const parentPath = record.path.slice(0, -1);
  const fallbackLabel =
    parentId != null && parentId === record.parent_id && parentPath.length > 0 ? formatPath(parentPath) : undefined;
  const quickError = quick.error ? asApiError(quick.error, "无法新增物品") : null;

  function onSave(event: FormEvent) {
    event.preventDefault();
    setFormError(null);
    save.mutate(locationUpdateBody(record.type, version, name, code, parentId, icon));
  }

  function onQuick(event: FormEvent) {
    event.preventDefault();
    quick.mutate({ name: quickName, locationId: record.id });
  }

  function setNamedOffset(key: string, offset: number) {
    setParams((current) => setOffsetParam(current, key, offset));
  }

  return (
    <>
      <h1 className={styles.title}>{heading}</h1>
      <Breadcrumb path={record.path} />
      <section>
        <h2 className={styles.sectionTitle}>子位置</h2>
        <div className={styles.actions}>
          <Link className={styles.button} to={`/locations/new?parent=${record.id}`}>
            新增子位置
          </Link>
        </div>
        {children.isLoading ? <Loading /> : null}
        {children.isError ? (
          <LoadError error={children.error} onRetry={() => void children.refetch()} pending={children.isFetching} />
        ) : null}
        {children.data && children.data.total === 0 ? <p>这里还没有下一级位置</p> : null}
        {children.data && children.data.data.length > 0 ? <LocationLinks rows={children.data.data} /> : null}
        {children.data ? (
          <Pager
            offset={children.data.offset}
            limit={children.data.limit}
            total={children.data.total}
            count={children.data.data.length}
            onPage={(offset) => setNamedOffset("children", offset)}
          />
        ) : null}
      </section>
      <section>
        <h2 className={styles.sectionTitle}>物品</h2>
        {directItems.isLoading ? <Loading /> : null}
        {directItems.isError ? (
          <LoadError error={directItems.error} onRetry={() => void directItems.refetch()} pending={directItems.isFetching} />
        ) : null}
        {directItems.data && directItems.data.total === 0 ? (
          <>
            <p>这个位置里还没有物品</p>
            {record.direct_item_count > 0 ? <p>回收站里还有物品占用这个位置</p> : null}
          </>
        ) : null}
        {directItems.data && directItems.data.data.length > 0 ? (
          <ul className={styles.list}>
            {directItems.data.data.map((item) => (
              <li key={item.id}>
                <Link className={styles.itemLink} to={`/items/${item.id}`}>
                  <ItemThumb photoId={item.photos[0]?.id} />
                  <span className={styles.itemLinkBody}>{item.name}</span>
                </Link>
              </li>
            ))}
          </ul>
        ) : null}
        {directItems.data ? (
          <Pager
            offset={directItems.data.offset}
            limit={directItems.data.limit}
            total={directItems.data.total}
            count={directItems.data.data.length}
            onPage={(offset) => setNamedOffset("items", offset)}
          />
        ) : null}
        <form onSubmit={onQuick}>
          <TextField label="名称" name="quick-name" value={quickName} onChange={setQuickName} error={fieldText(quickError, "name")} />
          {fieldText(quickError, "locations") ? <p className={styles.error}>{fieldText(quickError, "locations")}</p> : null}
          <FormIssues error={quickError} />
          <button className={styles.button} type="submit" disabled={quick.isPending}>
            添加物品
          </button>
        </form>
      </section>
      <form onSubmit={onSave}>
        <TextField label="名称" name="name" value={name} onChange={setName} error={fieldText(formError, "name")} />
        {record.type !== "area" ? (
          <TextField label="编号" name="code" value={code} onChange={setCode} error={fieldText(formError, "code")} />
        ) : null}
        <div className={styles.field}>
          <span>类型</span>
          <span>{typeLabel[record.type]}</span>
          {fieldText(formError, "type") ? <span className={styles.error}>{fieldText(formError, "type")}</span> : null}
          {record.type === "area" && fieldText(formError, "code") ? (
            <span className={styles.error}>{fieldText(formError, "code")}</span>
          ) : null}
        </div>
        <ParentField
          type={record.type}
          excludeId={record.id}
          parentId={parentId}
          onChange={setParentId}
          allowNone={record.type !== "fixed"}
          error={fieldText(formError, "parent_id")}
          fallbackLabel={fallbackLabel}
        />
        <IconPicker value={icon} type={record.type} onChange={setIcon} error={fieldText(formError, "icon")} />
        <FormIssues error={formError} />
        {conflict ? (
          <ConflictNotice message={conflictMessage} pending={loadingLatest} onLoad={() => void loadLatest()} error={latestError} />
        ) : null}
        <button className={styles.button} type="submit" disabled={save.isPending}>
          保存
        </button>
        {canDelete ? (
          <DeleteConfirm
            confirming={confirming}
            pending={remove.isPending}
            error={deleteError}
            onAsk={() => {
              setConfirming(true);
              setDeleteError(null);
            }}
            onCancel={() => {
              setConfirming(false);
              setDeleteError(null);
            }}
            onConfirm={() => remove.mutate(version)}
          />
        ) : null}
      </form>
    </>
  );
}

function categoryCreateBody(name: string, parentId: number | null): CategoryCreate {
  const body: CategoryCreate = { name };
  if (parentId != null) body.parent_id = parentId;
  return body;
}

function CategoryLinks({ rows }: { rows: Category[] }) {
  return (
    <ul className={`${styles.list} ${styles.treeList}`}>
      {rows.map((row) => (
        <li
          key={row.id}
          className={treeDepth(row.path.length) > 0 ? styles.treeChild : undefined}
          style={treeStyle(row.path.length)}
        >
          <Link className={styles.itemLink} to={`/categories/${row.id}`}>
            <span className={styles.itemLinkBody}>
              <span>{row.name}</span>
              {row.direct_item_count > 0 ? (
                <span className={styles.countMuted}>{row.direct_item_count} 件</span>
              ) : null}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function CategoryParentField({
  excludeId,
  parentId,
  onChange,
  error,
  fallbackLabel,
}: {
  excludeId?: number;
  parentId: number | null;
  onChange: (parentId: number | null) => void;
  error?: string;
  fallbackLabel?: string;
}) {
  const options = useQuery({
    queryKey: ["categories", "eligible", excludeId ?? null],
    queryFn: () => {
      const params = new URLSearchParams({ eligible_parent: "1" });
      if (excludeId != null) params.set("exclude", String(excludeId));
      return fetchAllCategories(params);
    },
    retry: false,
  });
  useOnUnauth(options.error);
  const known = parentId != null && (options.data ?? []).some((cat) => cat.id === parentId);

  return (
    <label className={styles.field}>
      父级
      {options.isLoading ? <Loading /> : null}
      {options.isError ? (
        <LoadError error={options.error} onRetry={() => void options.refetch()} pending={options.isFetching} />
      ) : null}
      {options.data ? (
        <select
          name="parent_id"
          value={parentId == null ? "" : String(parentId)}
          onChange={(event) => {
            const value = event.target.value;
            onChange(value === "" ? null : Number(value));
          }}
        >
          <option value="">不设置父级</option>
          {parentId != null && !known ? <option value={parentId}>{fallbackLabel ?? "已选择的父级"}</option> : null}
          {options.data.map((cat) => (
            <option key={cat.id} value={cat.id}>
              {optionPrefix(cat.path.length)}
              {formatCategoryPath(cat.path)}
            </option>
          ))}
        </select>
      ) : null}
      {error ? <span className={styles.error}>{error}</span> : null}
    </label>
  );
}

export function CategoryListPage() {
  const [params, setParams] = useSearchParams();
  const offsetRaw = params.get("offset");
  const query = useQuery({
    queryKey: ["categories", "roots", offsetRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams();
      if (offsetRaw) api.set("offset", offsetRaw);
      return listCategories(api);
    },
    retry: false,
  });
  useOnUnauth(query.error);
  const page = query.data;

  return (
    <>
      <h1 className={styles.title}>分类</h1>
      <div className={styles.actions}>
        <Link className={styles.buttonPrimary} to="/categories/new">
          新增
        </Link>
      </div>
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>还没有分类</p> : null}
      {page && page.data.length > 0 ? <CategoryLinks rows={page.data} /> : null}
      {page ? (
        <Pager
          offset={page.offset}
          limit={page.limit}
          total={page.total}
          count={page.data.length}
          onPage={(offset) => setParams((current) => setOffsetParam(current, "offset", offset))}
        />
      ) : null}
    </>
  );
}

export function CategoryCreatePage() {
  const [params] = useSearchParams();
  const parentParam = params.get("parent") ?? "";
  return <CategoryCreateForm key={parentParam} parentParam={parentParam} />;
}

function CategoryCreateForm({ parentParam }: { parentParam: string }) {
  const navigate = useNavigate();
  const hasParent = parentParam !== "";
  const parentQuery = useQuery({
    queryKey: ["category", "parent", parentParam],
    queryFn: () => getCategory(parentParam),
    enabled: hasParent,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnMount: "always",
  });
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [formError, setFormError] = useState<ApiError | null>(null);

  const create = useMutation({
    mutationFn: (body: CategoryCreate) => createCategory(body),
    onSuccess: (cat) => navigate(`/categories/${cat.id}`),
    onError: (error) => setFormError(asApiError(error, "无法新增分类")),
  });
  useOnUnauth(parentQuery.error);
  useOnUnauth(create.error);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (hasParent && !parentQuery.data) return;
    setFormError(null);
    const chosenParent = hasParent ? (parentQuery.data?.id ?? null) : parentId;
    create.mutate(categoryCreateBody(name, chosenParent));
  }

  if (hasParent && parentQuery.isError) {
    return (
      <>
        <h1 className={styles.title}>新增分类</h1>
        <LoadError error={parentQuery.error} onRetry={() => void parentQuery.refetch()} pending={parentQuery.isFetching} />
        <Link className={styles.button} to="/categories/new">
          重新选择父级
        </Link>
      </>
    );
  }
  if (hasParent && !parentQuery.data) return <Loading />;

  return (
    <>
      <h1 className={styles.title}>新增分类</h1>
      <form onSubmit={onSubmit}>
        <TextField label="名称" name="name" value={name} onChange={setName} error={fieldText(formError, "name")} />
        {hasParent && parentQuery.data ? (
          <div className={styles.field}>
            <span>父级</span>
            <span className={styles.path}>{formatCategoryPath(parentQuery.data.path)}</span>
            <Link className={styles.button} to="/categories/new">
              重新选择父级
            </Link>
            {fieldText(formError, "parent_id") ? <span className={styles.error}>{fieldText(formError, "parent_id")}</span> : null}
          </div>
        ) : null}
        {!hasParent ? (
          <CategoryParentField parentId={parentId} onChange={setParentId} error={fieldText(formError, "parent_id")} />
        ) : null}
        <FormIssues error={formError} />
        <button className={styles.button} type="submit" disabled={create.isPending}>
          保存
        </button>
      </form>
    </>
  );
}

export function CategoryPage() {
  const params = useParams();
  const id = params.id ?? "";
  return <CategoryScreen key={id} id={id} />;
}

function CategoryScreen({ id }: { id: string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const childrenRaw = params.get("children");
  const itemsRaw = params.get("items");
  const selfOnly = params.get("category_descendants") === "0";
  const cat = useQuery({
    queryKey: ["category", id],
    queryFn: () => getCategory(id),
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const children = useQuery({
    queryKey: ["categories", "children", id, childrenRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams({ parent: id });
      if (childrenRaw) api.set("offset", childrenRaw);
      return listCategories(api);
    },
    enabled: cat.isSuccess,
    retry: false,
  });
  const branchItems = useQuery({
    queryKey: ["items", "in-category", id, selfOnly ? "0" : "", itemsRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams({ category: id });
      if (selfOnly) api.set("category_descendants", "0");
      if (itemsRaw) api.set("offset", itemsRaw);
      return listItems(api);
    },
    enabled: cat.isSuccess,
    retry: false,
  });
  const [formReady, setFormReady] = useState(false);
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [version, setVersion] = useState(0);
  const [conflict, setConflict] = useState(false);
  const [conflictMessage, setConflictMessage] = useState("");
  const [latestError, setLatestError] = useState("");
  const [loadingLatest, setLoadingLatest] = useState(false);
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [deleteError, setDeleteError] = useState<ApiError | null>(null);
  const recordReady = cat.isSuccess && cat.isFetchedAfterMount && !cat.isFetching && cat.data != null;

  useEffect(() => {
    if (formReady || !recordReady || !cat.data) return;
    setName(cat.data.name);
    setParentId(cat.data.parent_id);
    setVersion(cat.data.version);
    setFormReady(true);
  }, [formReady, recordReady, cat.data]);

  function applyServer(next: Category) {
    setName(next.name);
    setParentId(next.parent_id);
    setVersion(next.version);
    setConflict(false);
  }

  const save = useMutation({
    mutationFn: (body: CategoryUpdate) => updateCategory(id, body),
    onSuccess: (next) => {
      queryClient.setQueryData(["category", id], next);
      applyServer(next);
      setFormError(null);
    },
    onError: (error) => {
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setFormError(asApiError(error, "无法保存分类"));
    },
  });
  const remove = useMutation({
    mutationFn: (ver: number) => deleteCategory(id, ver),
    onSuccess: () => {
      const current = queryClient.getQueryData<Category>(["category", id]);
      const parent = current?.parent_id ?? null;
      queryClient.removeQueries({ queryKey: ["category", id] });
      void queryClient.invalidateQueries({ queryKey: ["categories"] });
      void queryClient.invalidateQueries({ queryKey: ["items"] });
      navigate(parent != null ? `/categories/${parent}` : "/categories");
    },
    onError: (error) => {
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setDeleteError(asApiError(error, "无法删除分类"));
    },
  });
  useOnUnauth(cat.error);
  useOnUnauth(children.error);
  useOnUnauth(branchItems.error);
  useOnUnauth(save.error);
  useOnUnauth(remove.error);

  async function loadLatest() {
    setLoadingLatest(true);
    setLatestError("");
    try {
      const next = await getCategory(id);
      queryClient.setQueryData(["category", id], next);
      applyServer(next);
      setConfirming(false);
      setFormError(null);
      setDeleteError(null);
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      if (isNotFound(error)) {
        navigate("/categories", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setLatestError(messageOf(error, "无法读取"));
    } finally {
      setLoadingLatest(false);
    }
  }

  if (!formReady) {
    if (cat.isError) {
      return <LoadError error={cat.error} onRetry={() => void cat.refetch()} pending={cat.isFetching} />;
    }
    return <Loading />;
  }
  if (!cat.data) return <Loading />;

  const record = cat.data;
  const canDelete = children.isSuccess && children.data.total === 0 && record.direct_item_count === 0;
  const parentPath = record.path.slice(0, -1);
  const fallbackLabel =
    parentId != null && parentId === record.parent_id && parentPath.length > 0 ? formatCategoryPath(parentPath) : undefined;

  function onSave(event: FormEvent) {
    event.preventDefault();
    setFormError(null);
    save.mutate({ version, name, parent_id: parentId });
  }

  function setNamedOffset(key: string, offset: number) {
    setParams((current) => setOffsetParam(current, key, offset));
  }

  function setSelfOnly(next: boolean) {
    setParams((current) => {
      const params = new URLSearchParams(current);
      params.delete("items");
      if (next) params.set("category_descendants", "0");
      else params.delete("category_descendants");
      return params;
    });
  }

  return (
    <>
      <h1 className={styles.title}>{name}</h1>
      <CategoryBreadcrumb path={record.path} />
      <section>
        <h2 className={styles.sectionTitle}>子分类</h2>
        <div className={styles.actions}>
          <Link className={styles.button} to={`/categories/new?parent=${record.id}`}>
            新增子分类
          </Link>
        </div>
        {children.isLoading ? <Loading /> : null}
        {children.isError ? (
          <LoadError error={children.error} onRetry={() => void children.refetch()} pending={children.isFetching} />
        ) : null}
        {children.data && children.data.total === 0 ? <p>这里还没有下一级分类</p> : null}
        {children.data && children.data.data.length > 0 ? <CategoryLinks rows={children.data.data} /> : null}
        {children.data ? (
          <Pager
            offset={children.data.offset}
            limit={children.data.limit}
            total={children.data.total}
            count={children.data.data.length}
            onPage={(offset) => setNamedOffset("children", offset)}
          />
        ) : null}
      </section>
      <section>
        <h2 className={styles.sectionTitle}>物品</h2>
        <div className={styles.actions}>
          <Link className={styles.button} to={`/items/new?category=${record.id}`}>
            在此分类下新增物品
          </Link>
          <button className={styles.button} type="button" onClick={() => setSelfOnly(!selfOnly)}>
            {selfOnly ? "含下级" : "仅当前分类"}
          </button>
        </div>
        {branchItems.isLoading ? <Loading /> : null}
        {branchItems.isError ? (
          <LoadError error={branchItems.error} onRetry={() => void branchItems.refetch()} pending={branchItems.isFetching} />
        ) : null}
        {branchItems.data && branchItems.data.total === 0 ? (
          <>
            <p>这个分类下还没有物品</p>
            {record.direct_item_count > 0 ? <p>回收站里还有物品占用这个分类</p> : null}
          </>
        ) : null}
        {branchItems.data && branchItems.data.data.length > 0 ? (
          <ul className={styles.list}>
            {branchItems.data.data.map((item) => (
              <li key={item.id}>
                <Link className={styles.itemLink} to={`/items/${item.id}`}>
                  <ItemThumb photoId={item.photos[0]?.id} />
                  <span className={styles.itemLinkBody}>{item.name}</span>
                </Link>
              </li>
            ))}
          </ul>
        ) : null}
        {branchItems.data ? (
          <Pager
            offset={branchItems.data.offset}
            limit={branchItems.data.limit}
            total={branchItems.data.total}
            count={branchItems.data.data.length}
            onPage={(offset) => setNamedOffset("items", offset)}
          />
        ) : null}
      </section>
      <form onSubmit={onSave}>
        <TextField label="名称" name="name" value={name} onChange={setName} error={fieldText(formError, "name")} />
        <CategoryParentField
          excludeId={record.id}
          parentId={parentId}
          onChange={setParentId}
          error={fieldText(formError, "parent_id")}
          fallbackLabel={fallbackLabel}
        />
        <FormIssues error={formError} />
        {conflict ? (
          <ConflictNotice message={conflictMessage} pending={loadingLatest} onLoad={() => void loadLatest()} error={latestError} />
        ) : null}
        <button className={styles.button} type="submit" disabled={save.isPending}>
          保存
        </button>
        {canDelete ? (
          <DeleteConfirm
            confirming={confirming}
            pending={remove.isPending}
            error={deleteError}
            onAsk={() => {
              setConfirming(true);
              setDeleteError(null);
            }}
            onCancel={() => {
              setConfirming(false);
              setDeleteError(null);
            }}
            onConfirm={() => remove.mutate(version)}
          />
        ) : null}
      </form>
    </>
  );
}

export function TrashListPage() {
  const [params, setParams] = useSearchParams();
  const offsetRaw = params.get("offset");
  const query = useQuery({
    queryKey: ["trash", "list", offsetRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams();
      if (offsetRaw) api.set("offset", offsetRaw);
      return listTrash(api);
    },
    retry: false,
  });
  useOnUnauth(query.error);
  const page = query.data;

  useEffect(() => {
    if (!page) return;
    const currentOffset = Number(offsetRaw || "0");
    const clamped = clampPageOffset(page.total, currentOffset, page.limit > 0 ? page.limit : 30);
    if (clamped !== currentOffset) {
      setParams((current) => setOffsetParam(current, "offset", clamped));
    }
  }, [page, offsetRaw, setParams]);

  return (
    <>
      <h1 className={styles.title}>回收站</h1>
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>回收站是空的</p> : null}
      {page && page.data.length > 0 ? (
        <ul className={styles.list}>
          {page.data.map((item) => (
            <li key={item.id}>
              <Link className={styles.itemLink} to={`/trash/${item.id}`}>
                <ItemThumb photoId={item.photos[0]?.id} />
                <span className={styles.itemLinkBody}>
                  <span>{item.name}</span>
                  <span className={styles.meta}>{item.deleted_at ?? ""}</span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
      {page ? (
        <Pager
          offset={page.offset}
          limit={page.limit}
          total={page.total}
          count={page.data.length}
          onPage={(offset) => setParams((current) => setOffsetParam(current, "offset", offset))}
        />
      ) : null}
    </>
  );
}

export function TrashItemPage() {
  const params = useParams();
  const id = params.id ?? "";
  return <TrashItemScreen key={id} id={id} />;
}

function TrashItemScreen({ id }: { id: string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["trash", "item", id],
    queryFn: () => getTrashItem(id),
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const [conflict, setConflict] = useState(false);
  const [conflictMessage, setConflictMessage] = useState("");
  const [latestError, setLatestError] = useState("");
  const [loadingLatest, setLoadingLatest] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [restoreError, setRestoreError] = useState<ApiError | null>(null);
  const [purgeError, setPurgeError] = useState<ApiError | null>(null);

  function invalidateAfterTrashChange() {
    void queryClient.invalidateQueries({ queryKey: ["trash"] });
    void queryClient.invalidateQueries({ queryKey: ["items"] });
    void queryClient.invalidateQueries({ queryKey: ["locations"] });
    void queryClient.invalidateQueries({ queryKey: ["categories"] });
    void queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
  }

  const restore = useMutation({
    mutationFn: (ver: number) => restoreTrashItem(id, ver),
    onSuccess: (next) => {
      queryClient.setQueryData(["item", id], next);
      queryClient.removeQueries({ queryKey: ["trash", "item", id] });
      invalidateAfterTrashChange();
      navigate(`/items/${id}`);
    },
    onError: (error) => {
      if (isNotFound(error)) {
        navigate("/trash", { replace: true, state: { notice: "未找到" } });
        return;
      }
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setRestoreError(asApiError(error, "无法恢复物品"));
    },
  });
  const purge = useMutation({
    mutationFn: (ver: number) => purgeTrashItem(id, ver),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["trash", "item", id] });
      queryClient.removeQueries({ queryKey: ["item", id] });
      invalidateAfterTrashChange();
      navigate("/trash");
    },
    onError: (error) => {
      if (isNotFound(error)) {
        navigate("/trash", { replace: true, state: { notice: "未找到" } });
        return;
      }
      if (isVersionConflict(error)) {
        setConflict(true);
        setConflictMessage(messageOf(error, "记录已被修改"));
        return;
      }
      setPurgeError(asApiError(error, "无法永久删除"));
    },
  });
  useOnUnauth(query.error);
  useOnUnauth(restore.error);
  useOnUnauth(purge.error);

  async function loadLatest() {
    setLoadingLatest(true);
    setLatestError("");
    try {
      const next = await getTrashItem(id);
      queryClient.setQueryData(["trash", "item", id], next);
      setConflict(false);
      setConfirming(false);
      setRestoreError(null);
      setPurgeError(null);
    } catch (error) {
      if (isUnauthenticated(error)) {
        queryClient.setQueryData(["me"], null);
        navigate("/login", { replace: true });
        return;
      }
      if (isNotFound(error)) {
        navigate("/trash", { replace: true, state: { notice: "未找到" } });
        return;
      }
      setLatestError(messageOf(error, "无法读取"));
    } finally {
      setLoadingLatest(false);
    }
  }

  if (query.isError) {
    if (isNotFound(query.error)) {
      return <Navigate to="/trash" replace state={{ notice: "未找到" }} />;
    }
    return <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} />;
  }
  if (!query.data) return <Loading />;

  const item = query.data;

  return (
    <>
      <h1 className={styles.title}>{item.name}</h1>
      <div className={styles.field}>
        <span>名称</span>
        <span>{item.name}</span>
      </div>
      <div className={styles.field}>
        <span>别名</span>
        <span>{item.alias ?? ""}</span>
      </div>
      <div className={styles.field}>
        <span>型号</span>
        <span>{item.model ?? ""}</span>
      </div>
      <div className={styles.field}>
        <span>规格</span>
        <span>{item.spec ?? ""}</span>
      </div>
      <div className={styles.field}>
        <span>数量说明</span>
        <span>{item.quantity_note ?? ""}</span>
      </div>
      <div className={styles.field}>
        <span>备注</span>
        <span>{item.note ?? ""}</span>
      </div>
      <div className={styles.field}>
        <span>存放位置</span>
        {item.locations.map((link) => (
          <div key={link.location_id}>
            {link.path.length > 0 ? <LocationPathLine path={link.path} /> : <span className={styles.path}>{String(link.location_id)}</span>}
            {link.note ? <span className={styles.meta}>{link.note}</span> : null}
          </div>
        ))}
      </div>
      <div className={styles.field}>
        <span>分类</span>
        {item.categories.map((link) => (
          <span key={link.category_id} className={styles.path}>
            {link.path.length > 0 ? formatCategoryPath(link.path) : String(link.category_id)}
          </span>
        ))}
      </div>
      <ItemPhotoSection itemId={id} photos={item.photos} readOnly />
      <section>
        <h2 className={styles.sectionTitle}>待归位事项</h2>
        {item.return_tasks.length > 0 ? (
          <ul className={styles.list}>
            {item.return_tasks.map((task) => (
              <li key={task.id}>
                <ReturnTaskFields task={task} showCreatedAt />
                {task.completed_at ? (
                  <>
                    <p>已归位</p>
                    <p>{task.completed_at}</p>
                  </>
                ) : null}
              </li>
            ))}
          </ul>
        ) : null}
      </section>
      {conflict ? (
        <ConflictNotice message={conflictMessage} pending={loadingLatest} onLoad={() => void loadLatest()} error={latestError} />
      ) : null}
      <FormIssues error={restoreError} />
      <div className={styles.actions}>
        <button
          className={styles.button}
          type="button"
          onClick={() => {
            setRestoreError(null);
            restore.mutate(item.version);
          }}
          disabled={restore.isPending || purge.isPending}
        >
          恢复
        </button>
        <DeleteConfirm
          confirming={confirming}
          pending={purge.isPending}
          error={purgeError}
          askLabel="永久删除"
          onAsk={() => {
            setConfirming(true);
            setPurgeError(null);
          }}
          onCancel={() => {
            setConfirming(false);
            setPurgeError(null);
          }}
          onConfirm={() => purge.mutate(item.version)}
        />
      </div>
    </>
  );
}

function AppearanceFields() {
  const [theme, setThemeState] = useState<Theme>(() => readTheme());
  const [accent, setAccentState] = useState<Accent>(() => readAccent());

  function pickTheme(next: Theme) {
    setTheme(next);
    setThemeState(next);
  }

  function pickAccent(next: Accent) {
    setAccent(next);
    setAccentState(next);
  }

  return (
    <section>
      <h2 className={styles.sectionTitle}>外观</h2>
      <fieldset className={styles.field}>
        <legend>亮度</legend>
        <div className={styles.choiceRow}>
          <label className={styles.filterChoice}>
            <input type="radio" name="theme" checked={theme === "dark"} onChange={() => pickTheme("dark")} />
            深色
          </label>
          <label className={styles.filterChoice}>
            <input type="radio" name="theme" checked={theme === "light"} onChange={() => pickTheme("light")} />
            浅色
          </label>
        </div>
      </fieldset>
      <fieldset className={styles.field}>
        <legend>强调色</legend>
        <div className={styles.choiceRow}>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "moss"} onChange={() => pickAccent("moss")} />
            苔绿
          </label>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "clay"} onChange={() => pickAccent("clay")} />
            陶土
          </label>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "ink"} onChange={() => pickAccent("ink")} />
            墨蓝
          </label>
        </div>
      </fieldset>
    </section>
  );
}

export function AccountPage() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe, retry: false });
  const tokens = useQuery({ queryKey: ["tokens"], queryFn: listTokens, retry: false });
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [username, setUsername] = useState(me.data?.username ?? "");
  const [usernameError, setUsernameError] = useState<ApiError | null>(null);
  const [usernameNotice, setUsernameNotice] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState<ApiError | null>(null);
  const [passwordMismatch, setPasswordMismatch] = useState("");
  const [passwordNotice, setPasswordNotice] = useState("");
  const [tokenName, setTokenName] = useState("");
  const [scopeRead, setScopeRead] = useState(true);
  const [scopeOrganize, setScopeOrganize] = useState(true);
  const [scopeWrite, setScopeWrite] = useState(false);
  const [tokenError, setTokenError] = useState<ApiError | null>(null);
  const [tokenScopeError, setTokenScopeError] = useState("");
  const [plaintext, setPlaintext] = useState("");
  const [revokeError, setRevokeError] = useState<ApiError | null>(null);

  const saveUsername = useMutation({
    mutationFn: (value: string) => updateUsername(value),
    onSuccess: (updated) => {
      queryClient.setQueryData(["me"], { username: updated.username });
      setUsername(updated.username);
      setUsernameNotice("用户名已保存");
    },
    onError: (error) => setUsernameError(asApiError(error, "无法保存用户名")),
  });
  useOnUnauth(saveUsername.error);

  const savePassword = useMutation({
    mutationFn: ({ current, next }: { current: string; next: string }) => changePassword(current, next),
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setPasswordNotice("密码已修改");
    },
    onError: (error) => setPasswordError(asApiError(error, "无法修改密码")),
  });
  useOnUnauth(savePassword.error);

  const createTok = useMutation({
    mutationFn: ({ name, scopes }: { name: string; scopes: AccessTokenScope[] }) => createToken(name, scopes),
    onSuccess: (created) => {
      setPlaintext(created.token);
      setTokenError(null);
      setTokenScopeError("");
      void queryClient.invalidateQueries({ queryKey: ["tokens"] });
    },
    onError: (error) => setTokenError(asApiError(error, "无法创建接入令牌")),
  });
  useOnUnauth(createTok.error);
  useOnUnauth(tokens.error);

  const revoke = useMutation({
    mutationFn: (id: number) => revokeToken(id),
    onSuccess: () => {
      setRevokeError(null);
      void queryClient.invalidateQueries({ queryKey: ["tokens"] });
    },
    onError: (error) => setRevokeError(asApiError(error, "无法撤销接入令牌")),
  });
  useOnUnauth(revoke.error);

  const signOut = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.setQueryData(["me"], null);
      navigate("/login");
    },
  });
  useOnUnauth(signOut.error);

  function onUsernameSubmit(event: FormEvent) {
    event.preventDefault();
    setUsernameError(null);
    setUsernameNotice("");
    saveUsername.mutate(username);
  }

  function onPasswordSubmit(event: FormEvent) {
    event.preventDefault();
    setPasswordError(null);
    setPasswordMismatch("");
    setPasswordNotice("");
    if (newPassword !== confirmPassword) {
      setPasswordMismatch("两次输入的新密码不一致");
      return;
    }
    savePassword.mutate({ current: currentPassword, next: newPassword });
  }

  function onTokenSubmit(event: FormEvent) {
    event.preventDefault();
    setTokenError(null);
    setTokenScopeError("");
    if (!scopeRead) {
      setTokenScopeError("至少需要读取权限");
      return;
    }
    const scopes: AccessTokenScope[] = ["read"];
    if (scopeOrganize) scopes.push("organize");
    if (scopeWrite) scopes.push("write");
    createTok.mutate({ name: tokenName, scopes });
  }

  return (
    <>
      <h1 className={styles.title}>账号</h1>
      <AppearanceFields />
      <h2 className={styles.sectionTitle}>用户名</h2>
      <form onSubmit={onUsernameSubmit}>
        <label className={styles.field}>
          用户名
          <input
            name="username"
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
          {fieldText(usernameError, "username") ? (
            <span className={styles.error}>{fieldText(usernameError, "username")}</span>
          ) : null}
        </label>
        <FormIssues error={usernameError} />
        <button className={styles.buttonPrimary} type="submit" disabled={saveUsername.isPending}>
          保存用户名
        </button>
      </form>
      {usernameNotice ? <p>{usernameNotice}</p> : null}
      <h2 className={styles.sectionTitle}>密码</h2>
      <form onSubmit={onPasswordSubmit}>
        <label className={styles.field}>
          当前密码
          <input
            name="current_password"
            type="password"
            autoComplete="current-password"
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
          />
          {fieldText(passwordError, "current_password") ? (
            <span className={styles.error}>{fieldText(passwordError, "current_password")}</span>
          ) : null}
        </label>
        <label className={styles.field}>
          新密码
          <input
            name="new_password"
            type="password"
            autoComplete="new-password"
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
          />
          {fieldText(passwordError, "new_password") ? (
            <span className={styles.error}>{fieldText(passwordError, "new_password")}</span>
          ) : null}
        </label>
        <label className={styles.field}>
          再输入新密码
          <input
            name="new_password_confirm"
            type="password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
          />
          {passwordMismatch ? <span className={styles.error}>{passwordMismatch}</span> : null}
        </label>
        <FormIssues error={passwordError} />
        <button className={styles.buttonPrimary} type="submit" disabled={savePassword.isPending}>
          修改密码
        </button>
      </form>
      {passwordNotice ? <p>{passwordNotice}</p> : null}
      <div className={styles.tokenBlock}>
        <h2 className={styles.sectionTitle}>接入令牌</h2>
        {tokens.isLoading ? <Loading /> : null}
        {tokens.isError ? (
          <LoadError error={tokens.error} onRetry={() => void tokens.refetch()} pending={tokens.isFetching} />
        ) : null}
        {tokens.data && tokens.data.length === 0 ? <p>还没有接入令牌</p> : null}
        {tokens.data && tokens.data.length > 0 ? (
          <ul className={styles.list}>
            {tokens.data.map((token) => (
              <li key={token.id}>
                <p className={styles.path}>{token.name}</p>
                <p className={styles.path}>{token.token_prefix}</p>
                <p>{formatTokenScopes(token.scopes)}</p>
                <p className={styles.meta}>创建时间：{token.created_at}</p>
                <button
                  className={styles.button}
                  type="button"
                  onClick={() => {
                    setRevokeError(null);
                    revoke.mutate(token.id);
                  }}
                  disabled={revoke.isPending && revoke.variables === token.id}
                >
                  撤销
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        {revokeError ? <p className={styles.error}>{revokeError.message}</p> : null}
        <form onSubmit={onTokenSubmit}>
          <label className={styles.field}>
            名称
            <input name="name" autoComplete="off" value={tokenName} onChange={(event) => setTokenName(event.target.value)} />
            {fieldText(tokenError, "name") ? <span className={styles.error}>{fieldText(tokenError, "name")}</span> : null}
          </label>
          <label className={styles.filterChoice}>
            <input type="checkbox" name="scope_read" checked={scopeRead} onChange={(event) => setScopeRead(event.target.checked)} />
            读取
          </label>
          <label className={styles.filterChoice}>
            <input
              type="checkbox"
              name="scope_organize"
              checked={scopeOrganize}
              onChange={(event) => setScopeOrganize(event.target.checked)}
            />
            归类
          </label>
          <label className={styles.filterChoice}>
            <input type="checkbox" name="scope_write" checked={scopeWrite} onChange={(event) => setScopeWrite(event.target.checked)} />
            写入
          </label>
          {tokenScopeError ? <p className={styles.error}>{tokenScopeError}</p> : null}
          {fieldText(tokenError, "scopes") ? <p className={styles.error}>{fieldText(tokenError, "scopes")}</p> : null}
          <FormIssues error={tokenError} />
          <button className={styles.buttonPrimary} type="submit" disabled={createTok.isPending}>
            创建令牌
          </button>
        </form>
        {plaintext ? (
          <div className={styles.field}>
            <input className={styles.tokenPlain} readOnly value={plaintext} />
            <p>请立刻复制，关闭或刷新后无法再看</p>
          </div>
        ) : null}
      </div>
      <p>
        <Link to="/trash">回收站</Link>
      </p>
      {signOut.isError ? <p className={styles.error}>{messageOf(signOut.error, "退出失败")}</p> : null}
      <button className={styles.button} type="button" onClick={() => signOut.mutate()} disabled={signOut.isPending}>
        退出
      </button>
    </>
  );
}
