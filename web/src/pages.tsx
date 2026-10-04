import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useEffect, useRef, useState, type CSSProperties, type Ref } from "react";
import { Link, Navigate, Outlet, ScrollRestoration, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
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
  type LocationIconRecord,
  type LocationType,
  type LocationUpdate,
  type PathNode,
  type Photo,
  type ReturnTask,
  type ReturnTaskCreate,
  changePassword,
  cloneLocation,
  completeReturnTask,
  createCategory,
  createItem,
  createLocation,
  createLocationIcon,
  createReturnTask,
  createToken,
  deleteCategory,
  deleteItem,
  deleteLocation,
  deleteLocationIcon,
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
  isIconInUse,
  isNotFound,
  isUnauthenticated,
  isVersionConflict,
  listCategories,
  listItems,
  listLocationIcons,
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
  updateLocationIcon,
  updateUsername,
  uploadItemPhoto,
} from "./api";
import { clampPageOffset, ITEM_PAGE_SIZES, readItemList, writeItemList, type ItemListState, type ItemPageSize } from "./filters";
import { Pager } from "./pager";
import { locationDeletable } from "./select";
import {
  BatchBar,
  LocationPickPanel,
  RowCheck,
  SelectToggle,
  UndoBanner,
  useItemListBatch,
  useLocationListBatch,
  useReturnListBatch,
  useTrashListBatch,
} from "./selectUi";
import { ICON_GROUPS, CustomSVG, LocationGlyph, defaultIconLabel } from "./icons";
import { LeaveGuard } from "./leaveGuard";
import { PageEnter, StaggerList } from "./motion";
import { PendingPhotos } from "./pendingPhotos";
import { PhotoAddButtons } from "./photos";
import { readLastCategoryIds, readLastLocationId, rememberItemPlacement } from "./prefs";
import { StickySave, saveButtonLabel } from "./stickySave";
import styles from "./styles.module.css";
import { type Accent, type Theme, readAccent, readTheme, setAccent, setTheme } from "./theme";
import { ItemSummary, NavIcon, PageHeading } from './catalogUi';
import { searchLocations } from './uiModel';
import { pendingUpload, uploadBatch } from './uploadBatch';
import { formatAddedAt } from './relativeTime';
import { Dialog } from './dialog';

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
  "custom_icon_id",
  "svg",
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

function useIconLibrary() {
  return useQuery({
    queryKey: ["location-icons"],
    queryFn: listLocationIcons,
    retry: false,
  });
}

function LocationPathLine({ path }: { path: PathNode[] }) {
  const library = useIconLibrary();
  const last = path[path.length - 1];
  return (
    <span className={`${styles.path} ${styles.pathWithIcon}`}>
      {last ? <LocationGlyph node={last} library={library.data ?? []} /> : null}
      <span>{path.length > 0 ? formatPath(path) : ""}</span>
    </span>
  );
}

function IconPicker({
  value,
  customIconId,
  type,
  onChange,
  error,
}: {
  value: string | null;
  customIconId: number | null;
  type?: LocationType | "";
  onChange: (icon: string | null, customIconId: number | null) => void;
  error?: string;
}) {
  const library = useIconLibrary();
  const queryClient = useQueryClient();
  const [uploadError, setUploadError] = useState("");
  const [uploadDraft, setUploadDraft] = useState<{name: string; svg: string} | null>(null);
  const upload = useMutation({
    mutationFn: (input: { name: string; svg: string }) => createLocationIcon(input.name, input.svg),
    onSuccess: (created) => {
      void queryClient.invalidateQueries({ queryKey: ["location-icons"] });
      onChange(null, created.id);
      setUploadError("");
      setUploadDraft(null);
    },
    onError: (error) => {
      const api = asApiError(error, "无法上传图标");
      setUploadError(fieldText(api, "svg") || fieldText(api, "name") || api.message);
    },
  });
  const previewType = type && isLocationType(type) ? type : "";
  const defaultOn = value == null && customIconId == null;
  const mine = library.data ?? [];

  async function onFile(file: File | undefined) {
    if (!file) return;
    setUploadError('');
    try { setUploadDraft({name: file.name.replace(/\.svg$/i, "").trim() || "图标", svg: await file.text()}); }
    catch { setUploadError('无法读取文件，请重新选择'); }
  }

  return (
    <fieldset className={styles.iconPicker}>
      <legend>图标</legend>
      <label className={`${styles.iconChoice} ${defaultOn ? styles.iconChoiceOn : ""}`}>
        <input type="radio" name="icon" checked={defaultOn} onChange={() => onChange(null, null)} />
        {previewType ? <LocationGlyph node={{ icon: null, custom_icon_id: null, type: previewType }} library={[]} /> : null}
        <span>默认（{defaultIconLabel(previewType)}）</span>
      </label>
      {ICON_GROUPS.map((group) => (
        <div key={group.label}>
          <p className={styles.iconGroupLabel}>{group.label}</p>
          <div className={styles.iconGrid}>
            {group.icons.map((icon) => (
              <label
                key={icon.slug}
                className={`${styles.iconChoice} ${value === icon.slug && customIconId == null ? styles.iconChoiceOn : ""}`}
              >
                <input
                  type="radio"
                  name="icon"
                  checked={value === icon.slug && customIconId == null}
                  onChange={() => onChange(icon.slug, null)}
                />
                <LocationGlyph node={{ icon: icon.slug, type: previewType || "area" }} library={[]} />
                <span>{icon.label}</span>
              </label>
            ))}
          </div>
        </div>
      ))}
      <p className={styles.iconGroupLabel}>自定义图标</p>
      <div className={styles.iconGrid}>
        {mine.map((icon) => (
          <label
            key={icon.id}
            className={`${styles.iconChoice} ${customIconId === icon.id ? styles.iconChoiceOn : ""}`}
          >
            <input
              type="radio"
              name="icon"
              checked={customIconId === icon.id}
              onChange={() => onChange(null, icon.id)}
            />
            <CustomSVG svg={icon.svg} />
            <span>{icon.name}</span>
          </label>
        ))}
      </div>
      <label className={styles.iconUpload}>
        上传 SVG
        <input
          type="file"
          accept="image/svg+xml,.svg"
          disabled={upload.isPending}
          onChange={(event) => {
            const file = event.target.files?.[0];
            event.target.value = "";
            void onFile(file);
          }}
        />
      </label>
      {uploadError ? <span className={styles.error}>{uploadError}</span> : null}
      {error ? <span className={styles.error}>{error}</span> : null}
      {uploadDraft ? <Dialog title="预览上传图标" onClose={()=>setUploadDraft(null)} busy={upload.isPending}>
        <img className={styles.uploadPreview} src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(uploadDraft.svg)}`} alt="待上传图标预览"/>
        <label className={styles.field}>图标名称<input required disabled={upload.isPending} value={uploadDraft.name} onChange={event=>setUploadDraft({...uploadDraft,name:event.target.value})}/></label>
        {uploadError ? <p className={styles.error} role="alert">{uploadError}</p> : null}
        <div className={styles.actions}><button className={styles.button} data-dialog-cancel type="button" disabled={upload.isPending} onClick={()=>setUploadDraft(null)}>取消</button><button className={styles.buttonPrimary} type="button" disabled={upload.isPending || !uploadDraft.name.trim()} onClick={()=>upload.mutate(uploadDraft)}>上传并选用</button></div>
      </Dialog> : null}
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
  customIconId: number | null,
): LocationCreate {
  const body: LocationCreate = { name, type };
  if (type !== "area") body.code = code;
  if (parentId != null) body.parent_id = parentId;
  if (customIconId != null) body.custom_icon_id = customIconId;
  else body.icon = icon;
  return body;
}

function locationUpdateBody(
  type: LocationType,
  version: number,
  name: string,
  code: string,
  parentId: number | null,
  icon: string | null,
  customIconId: number | null,
): LocationUpdate {
  const body: LocationUpdate = { version, name, parent_id: parentId };
  if (type !== "area") body.code = code;
  if (customIconId != null) body.custom_icon_id = customIconId;
  else body.icon = icon;
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
  if (me.isError) return <div className={styles.page}><LoadError error={me.error} onRetry={() => void me.refetch()} pending={me.isFetching}/></div>;
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
            <NavIcon name="items"/><span>找东西</span>
          </Link>
          <Link to="/locations" aria-current={locHere ? "page" : undefined}>
            <NavIcon name="locations"/><span>位置</span>
          </Link>
          <Link to="/categories" aria-current={catHere ? "page" : undefined}>
            <NavIcon name="categories"/><span>分类</span>
          </Link>
          <Link to="/returns" aria-current={retHere ? "page" : undefined}>
            <NavIcon name="returns"/><span>待归位</span>
          </Link>
        </nav>
        <Link className={styles.user} to="/account" aria-label="设置" aria-current={pathname.startsWith("/account") ? "page" : undefined}>
          <NavIcon name="account"/><span>{username} · 设置</span>
        </Link>
      </header>
      {notice ? <p className={styles.inlineNotice} role="status">{notice}</p> : null}
      <PageEnter pathname={pathname} className={styles.pageEnter}>
        <Outlet />
      </PageEnter>
      <ScrollRestoration getKey={loc=>loc.pathname+loc.search}/>
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
    <div role="alert">
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
  inputRef,
}: {
  label: string;
  name: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  multiline?: boolean;
  hint?: string;
  inputRef?: Ref<HTMLInputElement>;
}) {
  return (
    <label className={styles.field}>
      {label}
      {hint ? <span className={styles.meta}>{hint}</span> : null}
      {multiline ? (
        <textarea name={name} value={value} autoComplete="off" onChange={(event) => onChange(event.target.value)} />
      ) : (
        <input
          ref={inputRef}
          name={name}
          value={value}
          autoComplete="off"
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {error ? <span className={styles.error}>{error}</span> : null}
    </label>
  );
}

function scrollToFormError() {
  requestAnimationFrame(() => {
    document.querySelector(`.${styles.error}`)?.scrollIntoView({ block: "center" });
  });
}

function Breadcrumb({ path, from }: { path: PathNode[]; from?: string }) {
  return (
    <p className={styles.path}>
      {path.map((node, index) => {
        const last = index === path.length - 1;
        return (
          <span key={node.id}>
            {index > 0 ? " / " : null}
            {last ? formatNode(node) : <Link to={`/locations/${node.id}`} state={from ? {from} : undefined}>{formatNode(node)}</Link>}
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
    <Dialog title={askLabel === '删除' ? '确认删除' : askLabel} onClose={onCancel} busy={pending}>
      <p>{warning}</p>
      <div className={styles.actions}>
        <button className={styles.buttonDanger} type="button" onClick={onConfirm} disabled={pending}>
          {pending ? '正在处理…' : confirmLabel}
        </button>
        <button className={styles.button} data-dialog-cancel type="button" disabled={pending} onClick={onCancel}>
          取消
        </button>
      </div>
      <FormIssues error={error} />
    </Dialog>
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
  const [uploadLabel, setUploadLabel] = useState("");
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
    for (let i = 0; i < fileList.length; i++) {
      const file = fileList[i];
      setUploadLabel(`上传 ${i + 1}/${fileList.length}`);
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
    setUploadLabel("");
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
      {uploadLabel ? <p>{uploadLabel}</p> : null}
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
        <PhotoAddButtons disabled={busy} remaining={20 - ordered.length} onFiles={(files) => void handleFiles(files)} />
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
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [activeIndex,setActiveIndex] = useState(-1);
  const rootRef = useRef<HTMLDivElement>(null);
  const chosen = new Set(links.map((link) => link.location_id));
  const available = (options.data ?? []).filter((loc) => !chosen.has(loc.id));
  const matches = searchLocations(query, available);
  useEffect(()=>{if(activeIndex>=0) rootRef.current?.querySelector(`#location-choice-${matches[activeIndex]?.id}`)?.scrollIntoView({block:'nearest'});},[activeIndex,matches]);

  useEffect(() => {
    if (!open) return;
    function onPointer(event: PointerEvent) {
      if (rootRef.current?.contains(event.target as Node)) return;
      setOpen(false);
    }
    document.addEventListener("pointerdown", onPointer);
    return () => document.removeEventListener("pointerdown", onPointer);
  }, [open]);

  function add(loc: Location) {
    if (chosen.has(loc.id)) return;
    onChange([
      ...links,
      {
        location_id: loc.id,
        note: null,
        path: loc.path.map((node) => ({ ...node })),
      },
    ]);
    setQuery("");
    setOpen(false);
    setActiveIndex(-1);
  }

  return (
    <div className={styles.field} ref={rootRef}>
      <span>存放位置</span>
      {links.map((link) => (
        <div key={link.location_id}>
          {link.path.length > 0 ? <LocationPathLine path={link.path} /> : <span className={styles.path}>{String(link.location_id)}</span>}
          <details className={styles.filters} open={Boolean(link.note) || undefined}><summary>分放备注（可选）</summary>
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
          </details>
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
        <>
          <input
            type="search"
            name="location_q"
            value={query}
            placeholder="找位置"
            aria-label="找存放位置"
            role="combobox"
            autoComplete="off"
            aria-autocomplete="list"
            aria-expanded={open}
            aria-controls="location-choices"
            aria-activedescendant={open && activeIndex>=0 && matches[activeIndex] ? `location-choice-${matches[activeIndex].id}` : undefined}
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(-1);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            onKeyDown={(event) => {
              if(event.key==='ArrowDown' || event.key==='ArrowUp') {event.preventDefault();setOpen(true);setActiveIndex(current=>Math.max(0,Math.min(matches.length-1,current+(event.key==='ArrowDown' ? 1 : -1))));return;}
              if (event.key === "Escape") {
                event.preventDefault();
                setOpen(false);
                return;
              }
              if (event.key === "Enter") {
                event.preventDefault();
                if (open && activeIndex>=0 && matches[activeIndex]) add(matches[activeIndex]);
                else if (open && query.trim() && matches.length===1) add(matches[0]);
              }
            }}
          />
          {open ? (
            <ul id="location-choices" className={styles.locationChoices} role="listbox">
              {matches.length === 0 ? (
                <li className={styles.locationChoicesEmpty}>没有符合的位置</li>
              ) : (
                matches.map((loc,index) => (
                  <li key={loc.id} role="option" id={`location-choice-${loc.id}`} aria-selected={index===activeIndex}>
                    <button
                      className={styles.button}
                      type="button"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => add(loc)}
                    >
                      {formatPath(loc.path)}
                    </button>
                  </li>
                ))
              )}
            </ul>
          ) : null}
        </>
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
  const queryClient = useQueryClient();
  const options = useQuery({
    queryKey: ["categories", "flat"],
    queryFn: () => fetchAllCategories(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  useOnUnauth(options.error);
  const [picked, setPicked] = useState("");
  const [categorySearch,setCategorySearch] = useState('');
  const [newName, setNewName] = useState("");
  const [createError, setCreateError] = useState<ApiError | null>(null);
  const chosen = new Set(links.map((link) => link.category_id));
  const available = (options.data ?? []).filter((cat) => !chosen.has(cat.id) && formatCategoryPath(cat.path).toLowerCase().includes(categorySearch.trim().toLowerCase()));
  const create = useMutation({
    mutationFn: (name: string) => createCategory({ name }),
    onSuccess: (cat) => {
      void queryClient.invalidateQueries({ queryKey: ["categories"] });
      setNewName("");
      setCreateError(null);
      if (chosen.has(cat.id)) return;
      onChange([
        ...links,
        {
          category_id: cat.id,
          source: "human",
          path: cat.path.map((node) => ({ ...node })),
        },
      ]);
    },
    onError: (err) => setCreateError(asApiError(err, "无法新增分类")),
  });
  useOnUnauth(create.error);

  function submitNew() {
    const name = newName.trim();
    if (name === "" || create.isPending) return;
    setCreateError(null);
    create.mutate(name);
  }

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
          <input type="search" aria-label="找分类" placeholder="搜索分类名称" value={categorySearch} onChange={event=>{setCategorySearch(event.target.value);setPicked('');}}/>
          <select aria-label="选择分类" name="category_id" value={picked} onChange={(event) => setPicked(event.target.value)}>
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
      <details className={styles.filters}><summary>没有合适的分类？新建</summary>
      <div className={styles.stack}>
        <input
          name="new_category"
          value={newName}
          autoComplete="off"
          onChange={(event) => setNewName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            submitNew();
          }}
        />
        <button className={styles.button} type="button" disabled={newName.trim() === "" || create.isPending} onClick={submitNew}>
          新建
        </button>
      </div>
      {fieldText(createError, "name") ? <span className={styles.error}>{fieldText(createError, "name")}</span> : null}
      {createError && !fieldText(createError, "name") ? <span className={styles.error}>{createError.message}</span> : null}
      </details>
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
  nameRef,
}: {
  draft: ItemDraft;
  onChange: (draft: ItemDraft) => void;
  error: ApiError | null;
  extrasOpen: boolean;
  nameRef?: Ref<HTMLInputElement>;
}) {
  // React 19 types omit defaultOpen on <details>; seed once so open={extrasOpen} cannot trap it.
  const [extrasShown, setExtrasShown] = useState(extrasOpen);
  function set<K extends keyof ItemDraft>(key: K, value: ItemDraft[K]) {
    onChange({ ...draft, [key]: value });
  }
  return (
    <>
      <TextField
        label="名称"
        name="name"
        value={draft.name}
        onChange={(value) => set("name", value)}
        error={fieldText(error, "name")}
        inputRef={nameRef}
      />
      <ItemLocationsField links={draft.locations} onChange={(locations) => set("locations", locations)} error={fieldText(error, "locations")} />
      <details className={styles.filters}><summary>分类{draft.categories.length ? ` · 已选 ${draft.categories.length} 个` : '（可选）'}</summary>
      <ItemCategoriesField
        links={draft.categories}
        onChange={(categories) => set("categories", categories)}
        error={fieldText(error, "categories")}
      />
      </details>
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
  sort: "created_at",
  offset: "",
  limit: 30,
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
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [pickedCategory, setPickedCategory] = useState("");
  useEffect(() => {
    if(searchTimer.current) clearTimeout(searchTimer.current);
    setQInput(state.q);
    return ()=>{if(searchTimer.current) clearTimeout(searchTimer.current);};
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

  const batch = useItemListBatch(listQuery);
  const selectedItems = (page?.data ?? []).filter((item) => batch.selection.selected.has(item.id));

  useEffect(() => {
    if (!page) return;
    const currentOffset = Number(view.offset || "0");
    const clamped = clampPageOffset(page.total, currentOffset, page.limit > 0 ? page.limit : view.limit);
    if (clamped !== currentOffset) {
      setParams((current) => writeItemList({ ...readItemList(current), offset: clamped > 0 ? String(clamped) : "" }));
    }
  }, [page, view.offset, view.limit, setParams]);

  function applyFilters(patch: Partial<ItemListState>, replace=false) {
    setParams((current) => writeItemList({ ...readItemList(current), ...patch, offset: "" }),{replace});
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
      <PageHeading title="家里的东西" description="知道有什么，也知道在哪里。" action={<Link className={styles.buttonPrimary} to="/items/new">＋ 记一件</Link>}/>
      {batch.undo.message ? (
        <UndoBanner message={batch.undo.message} onUndo={batch.canUndo ? () => void batch.undoTrash() : undefined} onDismiss={() => batch.undo.clear()} />
      ) : null}
      <form
        className={styles.searchRow}
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          if(searchTimer.current) clearTimeout(searchTimer.current);
          applyFilters({ q: qInput });
        }}
      >
        <input
          type="search"
          name="q"
          value={qInput}
          autoComplete="off"
          aria-label="找家里的东西"
          placeholder="搜名称、型号，或找位置编号"
          onChange={(event) => {const value=event.target.value;setQInput(value);if(searchTimer.current) clearTimeout(searchTimer.current);searchTimer.current=setTimeout(()=>applyFilters({q:value},true),300);}}
        />
        <button className={styles.buttonPrimary} type="submit">
          查找
        </button>
      </form>
      <div className={styles.toolbar}>
        <div className={styles.chips}>
          <button className={!view.unlocated && !view.uncategorized ? styles.chipActive : styles.chip} onClick={() => applyFilters({unlocated:false,uncategorized:false})}>全部</button>
          <button className={view.unlocated ? styles.chipActive : styles.chip} onClick={() => applyFilters({unlocated:true,uncategorized:false})}>待定位</button>
          <button className={view.uncategorized ? styles.chipActive : styles.chip} onClick={() => applyFilters({uncategorized:true,unlocated:false})}>未分类</button>
        </div>
        <details className={styles.filters}>
          <summary>筛选与排序</summary>
      <label className={styles.field}>
        排序
        <select
          name="sort"
          value={view.sort}
          onChange={(event) => applyFilters({ sort: event.target.value === "name" ? "name" : "created_at" })}
        >
          <option value="created_at">最新在前</option>
          <option value="name">按名称</option>
        </select>
      </label>
      <label className={styles.field}>
        每页
        <select
          name="limit"
          value={String(view.limit)}
          onChange={(event) => applyFilters({ limit: Number(event.target.value) as ItemPageSize })}
        >
          {ITEM_PAGE_SIZES.map((size) => (
            <option key={size} value={size}>
              {size} 条
            </option>
          ))}
        </select>
      </label>
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
      </div>
      {hasFilter ? (
        <div className={styles.filterSummary}>
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
            className={styles.chip}
            type="button"
            onClick={() => {
              setQInput("");
              setPickedCategory("");
              setParams(writeItemList({ ...emptyItemList, limit: view.limit }));
            }}
          >
            清除条件 ×
          </button>
        </div>
      ) : null}
      {view.q.trim() && locationsQuery.data && searchLocations(view.q, locations).length > 0 ? (
        <section className={styles.locationResults}>
          <h2 className={styles.sectionTitle}>匹配的位置</h2>
          {searchLocations(view.q, locations).slice(0, 5).map(loc => <Link key={loc.id} className={styles.locationCard} to={`/locations/${loc.id}`}><NavIcon name="locations"/><span><strong>{loc.name}</strong>{loc.code ? <span className={styles.codeBadge}>{loc.code}</span> : null}<span className={styles.path}>{formatPath(loc.path)}</span></span><span>›</span></Link>)}
          {searchLocations(view.q, locations).length > 5 ? <Link className={styles.subtleLink} to={`/locations?q=${encodeURIComponent(view.q)}`}>查看全部匹配位置</Link> : null}
        </section>
      ) : null}
      <div className={styles.resultsHeader}>
        <p className={styles.resultCount}>{page ? `${page.total} 件物品` : '物品'}</p>
        <SelectToggle selecting={batch.selection.selecting} onToggle={() => batch.selection.toggleMode()} allSelected={batch.selection.pageAllSelected((page?.data ?? []).map(item=>item.id))} onSelectAll={() => batch.selection.selectAll((page?.data ?? []).map(item=>item.id))} selectAllDisabled={!page?.data.length}/>
      </div>
      {batch.error ? <p className={styles.inlineNotice} role="alert">{batch.error}</p> : null}
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>{emptyCopy(view)}</p> : null}
      {page && page.data.length > 0 ? (
        <StaggerList className={styles.list}>
          {page.data.map((item) => (
            <li key={item.id} className={styles.selectRow}>
              <RowCheck
                label={item.name}
                selecting={batch.selection.selecting}
                checked={batch.selection.selected.has(item.id)}
                onToggle={() => batch.selection.toggle(item.id)}
              />
              {batch.selection.selecting ? (
                <button className={styles.itemLink} type="button" onClick={() => batch.selection.toggle(item.id)}>
                  <ItemSummary item={item}/>
                </button>
              ) : (
                <Link className={styles.itemLink} to={`/items/${item.id}`} state={{from:`/${listQuery ? `?${listQuery}` : ''}`}}>
                  <ItemSummary item={item}/>
                </Link>
              )}
            </li>
          ))}
        </StaggerList>
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
      {batch.selection.selecting && batch.picking ? (
        <LocationPickPanel
          onPick={(loc) => {
            batch.setDest(loc);
            batch.setPicking(false);
            batch.setConfirm("move");
          }}
          onCancel={() => {
            batch.setPicking(false);
            batch.setDest(null);
          }}
        />
      ) : null}
      {batch.selection.selecting && !batch.picking ? (
        <BatchBar
          count={selectedItems.length}
          pending={batch.pending}
          error={batch.error}
          confirming={batch.confirm === "trash" ? "trash" : batch.confirm === "move" ? "move" : null}
          confirmText={
            batch.confirm === "trash"
              ? `将 ${selectedItems.length} 件物品移到回收站`
              : batch.confirm === "move" && batch.dest
                ? `将 ${selectedItems.length} 件搬到 ${batch.dest.name}${batch.dest.code ? ` ${batch.dest.code}` : ""}，原来的位置清掉`
                : undefined
          }
          onConfirm={() => {
            if (batch.confirm === "trash") void batch.trash(selectedItems);
            if (batch.confirm === "move" && batch.dest) void batch.move(selectedItems, batch.dest);
          }}
          onCancel={() => {
            batch.setConfirm(null);
            batch.setDest(null);
          }}
        >
          <button
            className={styles.buttonDanger}
            type="button"
            disabled={selectedItems.length === 0 || batch.pending}
            onClick={() => {
              batch.setError("");
              batch.setConfirm("trash");
            }}
          >
            移到回收站
          </button>
          <button
            className={styles.button}
            type="button"
            disabled={selectedItems.length === 0 || batch.pending}
            onClick={() => {
              batch.setError("");
              batch.setPicking(true);
            }}
          >
            更换存放位置
          </button>
        </BatchBar>
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
  const queryClient = useQueryClient();
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
  const [pendingPhotos, setPendingPhotos] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadIndex, setUploadIndex] = useState(0);
  const [uploadTotal, setUploadTotal] = useState(0);
  const [saveNotice, setSaveNotice] = useState<{ id: number; name: string; photoFail: boolean } | null>(null);
  const [failedUpload, setFailedUpload] = useState<{id:number;name:string;files:File[]} | null>(()=>pendingUpload.current);
  useEffect(()=>{pendingUpload.current=failedUpload;},[failedUpload]);
  const [retryingPhotos,setRetryingPhotos] = useState(false);
  const [retryPhotoError,setRetryPhotoError] = useState('');
  const [continueAdding,setContinueAdding] = useState(true);
  const [savedItemId,setSavedItemId] = useState<number|null>(null);
  const [justSaved, setJustSaved] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const appliedLast = useRef(false);
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

  useEffect(() => {
    if (!formReady || appliedLast.current) return;
    if (locationsQuery.isLoading || categoriesQuery.isLoading) return;
    appliedLast.current = true;
    setDraft((current) => {
      let next = current;
      if (!hasLocationPreset && next.locations.length === 0) {
        const lastId = readLastLocationId();
        const loc = locationsQuery.data?.find((item) => item.id === lastId);
        if (loc) {
          next = {
            ...next,
            locations: [{ location_id: loc.id, note: null, path: loc.path.map((node) => ({ ...node })) }],
          };
        }
      }
      if (!hasCategoryPreset && next.categories.length === 0) {
        const lastIds = new Set(readLastCategoryIds());
        const cats = (categoriesQuery.data ?? []).filter((cat) => lastIds.has(cat.id));
        if (cats.length > 0) {
          next = {
            ...next,
            categories: cats.map((cat) => ({
              category_id: cat.id,
              source: "human",
              path: cat.path.map((node) => ({ ...node })),
            })),
          };
        }
      }
      return next;
    });
  }, [
    formReady,
    hasLocationPreset,
    hasCategoryPreset,
    locationsQuery.isLoading,
    categoriesQuery.isLoading,
    locationsQuery.data,
    categoriesQuery.data,
  ]);

  const create = useMutation({
    mutationFn: (body: ItemCreate) => createItem(body),
    onError: (error) => {
      setFormError(asApiError(error, "无法新增物品"));
      scrollToFormError();
    },
  });
  useOnUnauth(preset.error);
  useOnUnauth(categoryPreset.error);
  useOnUnauth(locationsQuery.error);
  useOnUnauth(categoriesQuery.error);
  useOnUnauth(create.error);
  const saving = create.isPending || uploading || retryingPhotos;
  useEffect(()=>{
    if(savedItemId && !continueAdding && !saving && !failedUpload && draft.name === '') navigate(`/items/${savedItemId}`);
  },[savedItemId,continueAdding,saving,failedUpload,draft.name,navigate]);

  async function retryPhotos() {
    if (!failedUpload || retryingPhotos) return;
    setRetryingPhotos(true);
    setRetryPhotoError('');
    const failed = await uploadBatch(failedUpload.files, file=>uploadItemPhoto(String(failedUpload.id),file));
    setFailedUpload(failed.length ? {...failedUpload,files:failed.map(row=>row.file)} : null);
    if (failed.length) setRetryPhotoError(photoFailureText(failed[0].error,'照片仍未上传，请重试'));
    else setSaveNotice({id:failedUpload.id,name:failedUpload.name,photoFail:false});
    setRetryingPhotos(false);
    void queryClient.invalidateQueries({queryKey:['items']});
    void queryClient.invalidateQueries({queryKey:['item',String(failedUpload.id)]});
  }

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

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (saving || failedUpload) return;
    setFormError(null);
    setSaveNotice(null);
    const savedName = draft.name;
    const savedLocations = draft.locations.map((link) => link.location_id);
    const savedCategories = draft.categories.map((link) => link.category_id);
    let item: Item;
    try {
      item = await create.mutateAsync(itemBody(draft));
    } catch {
      return;
    }
    rememberItemPlacement(savedLocations, savedCategories);
    const files = pendingPhotos.slice(0, 20);
    let failedFiles: File[] = [];
    if (files.length > 0) {
      setUploading(true);
      setUploadTotal(files.length);
      const failed = await uploadBatch(files, (file,index)=>{setUploadIndex(index+1);return uploadItemPhoto(String(item.id),file);});
      failedFiles = failed.map(row=>row.file);
      if (failed.length) setRetryPhotoError(photoFailureText(failed[0].error,'照片未上传'));
      setUploading(false);
      setUploadTotal(0);
    }
    void queryClient.invalidateQueries({ queryKey: ["items"] });
    void queryClient.invalidateQueries({queryKey:['locations']});
    void queryClient.invalidateQueries({queryKey:['categories']});
    setPendingPhotos([]);
    setDraft((current) => ({ ...current, name: "", alias: "", model: "", spec: "", quantityNote: "", note: "", locations: current.locations.map(link => ({ ...link, note: null })) }));
    setFailedUpload(failedFiles.length ? {id:item.id,name:savedName,files:failedFiles} : null);
    setSaveNotice({ id: item.id, name: savedName, photoFail:failedFiles.length > 0 });
    setSavedItemId(continueAdding ? null : item.id);
    setJustSaved(true);
    window.setTimeout(() => setJustSaved(false), 1500);
    nameRef.current?.focus();
  }

  if (!formReady) return <Loading />;

  const saveLabel = failedUpload ? '先处理未上传照片' : !saving && !justSaved && continueAdding ? '保存并继续' : saveButtonLabel(saving, uploadIndex, uploadTotal, justSaved);
  const barError = formError && !fieldText(formError, "name") && !fieldText(formError, "locations") && !fieldText(formError, "categories")
    ? formError.message
    : fieldText(formError, "name") || fieldText(formError, "locations") || fieldText(formError, "categories") || undefined;

  return (
    <>
      <LeaveGuard dirty={draft.name.trim() !== "" || pendingPhotos.length > 0 || Boolean(failedUpload)} />
      <PageHeading title="记下家里的东西" description="只填名称就能保存，其他信息慢慢补。" action={<Link className={styles.button} to="/">返回目录</Link>}/>
      {failedUpload ? <div className={styles.uploadNotice} role="alert"><p><Link to={`/items/${failedUpload.id}`}>{failedUpload.name}</Link>已保存，{failedUpload.files.length} 张照片未上传。</p>{retryPhotoError ? <p className={styles.error}>{retryPhotoError}</p> : null}<div className={styles.actions}><button className={styles.button} type="button" disabled={saving} onClick={()=>void retryPhotos()}>{retryingPhotos ? '正在重试…' : '重试未上传照片'}</button><button className={styles.chip} type="button" disabled={saving} onClick={()=>setFailedUpload(null)}>放弃这几张照片</button><button className={styles.chip} type="button" disabled={saving} onClick={()=>{pendingUpload.current=failedUpload;queryClient.setQueryData(['me'],null);navigate('/login');}}>重新登录</button></div><p className={styles.meta}>照片暂存在当前标签页；重新登录后打开「记一件」可继续上传。刷新或关闭页面前请先处理。</p></div> : null}
      {saveNotice ? (
        <p className={styles.inlineNotice} role="status">
          已保存：
          <Link to={`/items/${saveNotice.id}`}>{saveNotice.name}</Link>
          {saveNotice.photoFail ? "，有照片没传上" : null}
        </p>
      ) : null}
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
      <form id="item-save-form" className={styles.editor} onSubmit={(event) => void onSubmit(event)}>
        {draft.locations.length || draft.categories.length ? <div className={styles.resultsHeader}><p className={styles.meta}>已选位置/分类会沿用到下一件</p><button type="button" className={styles.chip} disabled={saving} onClick={()=>{setDraft(current=>({...current,locations:[],categories:[]}));rememberItemPlacement([],[]);}}>清空预设</button></div> : null}
        <ItemFields draft={draft} onChange={setDraft} error={formError} extrasOpen={false} nameRef={nameRef} />
        <FormIssues error={formError} />
        <PendingPhotos files={pendingPhotos} onRemove={(index) => setPendingPhotos((current) => current.filter((_, i) => i !== index))} />
        {pendingPhotos.length > 0 ? <p>已选 {pendingPhotos.length} 张</p> : null}
        {pendingPhotos.length >= 20 ? (
          <p>一件物品最多 20 张照片</p>
        ) : (
          <PhotoAddButtons
            disabled={saving}
            remaining={20 - pendingPhotos.length}
            onFiles={(files) =>
              setPendingPhotos((current) => {
                const room = 20 - current.length;
                if (room <= 0) return current;
                return [...current, ...files.slice(0, room)];
              })
            }
          />
        )}
        <label className={styles.filterChoice}><input type="checkbox" checked={continueAdding} onChange={e=>setContinueAdding(e.target.checked)} disabled={saving}/>连续录入，保存后继续记下一件</label>
        <button className={`${styles.buttonPrimary} ${styles.formSave}`} type="submit" disabled={saving || Boolean(failedUpload)}>
          {saveLabel}
        </button>
      </form>
      <div className={styles.stickySaveSpacer} />
      <StickySave form="item-save-form" disabled={saving || Boolean(failedUpload)} label={saveLabel} error={barError} />
    </>
  );
}

function ReturnTaskFields({ task, showItemName, showCreatedAt }: { task: ReturnTask; showItemName?: boolean; showCreatedAt?: boolean }) {
  const current=useLocation();
  return (
    <>
      {showItemName ? (
        <Link className={styles.itemLink} to={`/items/${task.item_id}`} state={{from:current.pathname+current.search}}>
          <ItemThumb photoId={task.cover_photo?.id} />
          <span className={styles.itemLinkBody}>{task.item_name}</span>
        </Link>
      ) : null}
      <p>{partLabel(task)}</p>
      {task.reason ? <p>原因：{task.reason}</p> : null}
      {task.destination_note ? <p>临时去向：{task.destination_note}</p> : null}
      {showCreatedAt ? <p className={styles.meta}>登记于 {formatAddedAt(task.created_at)}</p> : null}
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
  const route=useLocation();
  const from=(route.state as {from?:unknown}|null)?.from;
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
  const [saveNotice, setSaveNotice] = useState("");
  const [justSaved, setJustSaved] = useState(false);
  const [baseline, setBaseline] = useState("");
  const nameRef = useRef<HTMLInputElement>(null);
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
    const nextDraft = draftFromItem(query.data);
    setDraft(nextDraft);
    setBaseline(JSON.stringify(nextDraft));
    setVersion(query.data.version);
    setOriginalCategoryIds(categoryIdsOf(query.data.categories));
    setFormReady(true);
  }, [formReady, recordReady, query.data]);

  function applyServer(next: Item) {
    const nextDraft = draftFromItem(next);
    setDraft(nextDraft);
    setBaseline(JSON.stringify(nextDraft));
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
      setSaveNotice("已保存");
      setJustSaved(true);
      void queryClient.invalidateQueries({queryKey:['items']});
      void queryClient.invalidateQueries({queryKey:['locations']});
      void queryClient.invalidateQueries({queryKey:['categories']});
      window.setTimeout(() => setJustSaved(false), 1500);
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
      scrollToFormError();
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
  const saveLabel = saveButtonLabel(save.isPending, 0, 0, justSaved);
  const barError =
    formError && !fieldText(formError, "name") && !fieldText(formError, "locations") && !fieldText(formError, "categories")
      ? formError.message
      : fieldText(formError, "name") || fieldText(formError, "locations") || fieldText(formError, "categories") || undefined;

  return (
    <>
      <LeaveGuard dirty={JSON.stringify(draft) !== baseline} />
      <PageHeading title="编辑物品" description={draft.name} action={<Link className={styles.button} to={`/items/${id}`} state={{from}}>返回详情</Link>}/>
      {saveNotice ? <p className={styles.inlineNotice} role="status">{saveNotice}</p> : null}
      <form id="item-save-form" className={styles.editor} onSubmit={onSubmit}>
        <ItemFields draft={draft} onChange={setDraft} error={formError} extrasOpen={extrasOpen} nameRef={nameRef} />
        <FormIssues error={formError} />
        {conflict ? (
          <ConflictNotice message={conflictMessage} pending={loadingLatest} onLoad={() => void loadLatest()} error={latestError} />
        ) : null}
        <button className={`${styles.buttonPrimary} ${styles.formSave}`} type="submit" disabled={save.isPending}>
          {saveLabel}
        </button>
      </form>
      <div className={styles.stickySaveSpacer} />
      <StickySave form="item-save-form" disabled={save.isPending} label={saveLabel} error={barError} />
      <ItemPhotoSection itemId={id} photos={query.data?.photos ?? []} />
      <section id="returns">
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

  const batch = useReturnListBatch(offsetRaw ?? "");
  const selectedTasks = (page?.data ?? []).filter((task) => batch.selection.selected.has(task.id));

  return (
    <>
      <PageHeading title="待归位" description="临时取出的东西，记得放回它的去处。"/>
      {batch.error ? <p role="alert" className={styles.inlineNotice}>{batch.error}</p> : null}
      {notice ? <p>{notice}</p> : null}
      {verifyError ? <p className={styles.error}>{verifyError}</p> : null}
      <SelectToggle
        selecting={batch.selection.selecting}
        onToggle={() => batch.selection.toggleMode()}
        allSelected={batch.selection.pageAllSelected((page?.data ?? []).map((task) => task.id))}
        onSelectAll={() => batch.selection.selectAll((page?.data ?? []).map((task) => task.id))}
        selectAllDisabled={!page || page.data.length === 0}
      />
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>没有待归位事项</p> : null}
      {page && page.data.length > 0 ? (
        <ul className={styles.list}>
          {page.data.map((task) =>
            batch.selection.selecting ? (
              <li key={task.id} className={styles.selectRow}>
                <RowCheck
                  label={task.item_name}
                  selecting
                  checked={batch.selection.selected.has(task.id)}
                  onToggle={() => batch.selection.toggle(task.id)}
                />
                <button className={styles.itemLink} type="button" onClick={() => batch.selection.toggle(task.id)}>
                  <span className={styles.itemLinkBody}>
                    <span>{task.item_name}</span>
                    <span className={styles.meta}>{task.part_note ?? ""}</span>
                  </span>
                </button>
              </li>
            ) : (
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
            ),
          )}
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
      {batch.selection.selecting ? (
        <BatchBar
          count={selectedTasks.length}
          pending={batch.pending}
          error={batch.error}
          confirming={batch.confirm ? "complete" : null}
          confirmText={`完成 ${selectedTasks.length} 条归位，不能撤销`}
          onConfirm={() => void batch.complete(selectedTasks)}
          onCancel={() => batch.setConfirm(false)}
        >
          <button
            className={styles.button}
            type="button"
            disabled={selectedTasks.length === 0 || batch.pending}
            onClick={() => batch.setConfirm(true)}
          >
            完成归位
          </button>
        </BatchBar>
      ) : null}
    </>
  );
}

function LocationLinks({
  rows,
  enter,
  from,
  selecting,
  selected,
  onToggle,
}: {
  rows: Location[];
  enter?: boolean;
  from?: string;
  selecting?: boolean;
  selected?: Set<number>;
  onToggle?: (id: number) => void;
}) {
  const library = useIconLibrary();
  const items = rows.map((row) => {
    const canDelete = locationDeletable(row);
    const checked = selected?.has(row.id) ?? false;
    const body = (
      <>
        <LocationGlyph node={row} library={library.data ?? []} className={styles.rowIcon} />
        <span className={styles.itemLinkBody}>
          <span>{row.name}</span>
          {row.code ? <span className={styles.codeBadge}>{row.code}</span> : null}
          {row.direct_item_count > 0 ? <span className={styles.countMuted}>{row.direct_item_count} 件</span> : null}
        </span>
      </>
    );
    return (
      <li
        key={row.id}
        className={`${styles.selectRow} ${treeDepth(row.path.length) > 0 ? styles.treeChild : ""}`}
        style={treeStyle(row.path.length)}
      >
        <RowCheck
          label={row.name}
          selecting={Boolean(selecting)}
          checked={checked}
          disabled={!canDelete}
          onToggle={() => {
            if (canDelete) onToggle?.(row.id);
          }}
        />
        {selecting && canDelete ? (
          <button className={styles.itemLink} type="button" onClick={() => onToggle?.(row.id)}>
            {body}
          </button>
        ) : (
          <Link className={styles.itemLink} to={`/locations/${row.id}`} state={from ? {from} : undefined}>
            {body}
          </Link>
        )}
      </li>
    );
  });
  const className = `${styles.list} ${styles.treeList}`;
  if (enter) return <StaggerList className={className}>{items}</StaggerList>;
  return <ul className={className}>{items}</ul>;
}

function LocationGroups({
  rows,
  selecting,
  selected,
  onToggle,
}: {
  rows: Location[];
  selecting?: boolean;
  selected?: Set<number>;
  onToggle?: (id: number) => void;
}) {
  const areas = rows.filter((row) => row.type === "area");
  const containers = rows.filter((row) => row.type !== "area");
  return (
    <>
      {areas.length > 0 ? (
        <section>
          <h2 className={styles.sectionTitle}>区域</h2>
          <LocationLinks rows={areas} enter selecting={selecting} selected={selected} onToggle={onToggle} />
        </section>
      ) : null}
      {containers.length > 0 ? (
        <section>
          <h2 className={styles.sectionTitle}>尚未放入的容器</h2>
          <LocationLinks rows={containers} enter selecting={selecting} selected={selected} onToggle={onToggle} />
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
  const batch = useLocationListBatch(offsetRaw ?? "");
  const deletableIds = (page?.data ?? []).filter(locationDeletable).map((row) => row.id);
  const selectedRows = (page?.data ?? []).filter((row) => batch.selection.selected.has(row.id));

  return (
    <>
      <h1 className={styles.title}>位置</h1>
      {batch.undo.message ? (
        <UndoBanner message={batch.undo.message} onUndo={() => void batch.undoDelete()} onDismiss={() => batch.undo.clear()} />
      ) : null}
      <div className={styles.actions}>
        <Link className={styles.buttonPrimary} to="/locations/new">
          新增
        </Link>
        <SelectToggle
          selecting={batch.selection.selecting}
          onToggle={() => batch.selection.toggleMode()}
          allSelected={batch.selection.pageAllSelected(deletableIds)}
          onSelectAll={() => batch.selection.selectAll(deletableIds)}
          selectAllDisabled={deletableIds.length === 0}
        />
      </div>
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>还没有位置</p> : null}
      {batch.error ? <p role="alert" className={styles.inlineNotice}>{batch.error}</p> : null}
      {page && page.data.length > 0 ? (
        <LocationGroups
          rows={page.data}
          selecting={batch.selection.selecting}
          selected={batch.selection.selected}
          onToggle={(id) => batch.selection.toggle(id)}
        />
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
      {batch.selection.selecting ? (
        <BatchBar
          count={selectedRows.length}
          pending={batch.pending}
          error={batch.error}
          confirming={batch.confirm ? "delete" : null}
          confirmText={`永久删除 ${selectedRows.length} 个位置，随后可短时撤销`}
          onConfirm={() => void batch.remove(selectedRows)}
          onCancel={() => batch.setConfirm(false)}
        >
          <button
            className={styles.buttonDanger}
            type="button"
            disabled={selectedRows.length === 0 || batch.pending}
            onClick={() => {
              batch.setError("");
              batch.setConfirm(true);
            }}
          >
            删除
          </button>
        </BatchBar>
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
  const [customIconId, setCustomIconId] = useState<number | null>(null);
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
    create.mutate(locationCreateBody(type, name, code, chosenParent, icon, customIconId));
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
        <IconPicker
          value={icon}
          customIconId={customIconId}
          type={type}
          onChange={(nextIcon, nextCustom) => {
            setIcon(nextIcon);
            setCustomIconId(nextCustom);
          }}
          error={fieldText(formError, "icon") || fieldText(formError, "custom_icon_id")}
        />
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
  const route = useLocation();
  const from = typeof route.state?.from === 'string' && route.state.from.startsWith('/locations?') ? route.state.from : '/locations';
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const childrenRaw = params.get("children");
  const itemsRaw = params.get("items");
  const selfOnly = params.get('scope') === 'direct';
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
    queryKey: ["items", "at", id, selfOnly, itemsRaw ?? ""],
    queryFn: () => {
      const api = new URLSearchParams(selfOnly ? { location: id } : { in_location: id });
      if (itemsRaw) api.set("offset", itemsRaw);
      return listItems(api);
    },
    enabled: loc.isSuccess,
    retry: false,
  });
  const childBatch = useLocationListBatch(`loc-children:${id}:${childrenRaw ?? ""}`);
  const itemBatch = useItemListBatch(`loc-items:${id}:${selfOnly}:${itemsRaw ?? ""}`);
  const [formReady, setFormReady] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [icon, setIcon] = useState<string | null>(null);
  const [customIconId, setCustomIconId] = useState<number | null>(null);
  const [version, setVersion] = useState(0);
  const [conflict, setConflict] = useState(false);
  const [conflictMessage, setConflictMessage] = useState("");
  const [latestError, setLatestError] = useState("");
  const [loadingLatest, setLoadingLatest] = useState(false);
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [deleteError, setDeleteError] = useState<ApiError | null>(null);
  const [quickName, setQuickName] = useState("");
  const [cloneNotice, setCloneNotice] = useState("");
  const [cloneError, setCloneError] = useState("");
  const recordReady = loc.isSuccess && loc.isFetchedAfterMount && !loc.isFetching && loc.data != null;

  useEffect(() => {
    if (formReady || !recordReady || !loc.data) return;
    setName(loc.data.name);
    setCode(loc.data.code ?? "");
    setParentId(loc.data.parent_id);
    setIcon(loc.data.icon);
    setCustomIconId(loc.data.custom_icon_id);
    setVersion(loc.data.version);
    setFormReady(true);
  }, [formReady, recordReady, loc.data]);

  function applyServer(next: Location) {
    setName(next.name);
    setCode(next.code ?? "");
    setParentId(next.parent_id);
    setIcon(next.icon);
    setCustomIconId(next.custom_icon_id);
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
  const cloneBox = useMutation({
    mutationFn: () => cloneLocation(id),
    onSuccess: (created) => {
      setCloneNotice(`已克隆为 ${created.code ?? created.name}`);
      setCloneError("");
      void queryClient.invalidateQueries({ queryKey: ["locations"] });
    },
    onError: (error) => {
      setCloneNotice("");
      setCloneError(messageOf(error, "无法克隆"));
    },
  });
  useOnUnauth(loc.error);
  useOnUnauth(children.error);
  useOnUnauth(directItems.error);
  useOnUnauth(save.error);
  useOnUnauth(remove.error);
  useOnUnauth(quick.error);
  useOnUnauth(cloneBox.error);

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
    save.mutate(locationUpdateBody(record.type, version, name, code, parentId, icon, customIconId));
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
      <Link className={styles.subtleLink} to={from}>← 返回位置目录</Link>
      <PageHeading title={heading} action={<Link className={styles.buttonPrimary} to={`/items/new?location=${id}`}>＋ 在这里记一件</Link>}/>
      <Breadcrumb path={record.path} from={from}/>
      {childBatch.undo.message ? (
        <UndoBanner message={childBatch.undo.message} onUndo={() => void childBatch.undoDelete()} onDismiss={() => childBatch.undo.clear()} />
      ) : null}
      {itemBatch.undo.message ? (
        <UndoBanner message={itemBatch.undo.message} onUndo={itemBatch.canUndo ? () => void itemBatch.undoTrash() : undefined} onDismiss={() => itemBatch.undo.clear()} />
      ) : null}
      {record.type === "movable" ? (
        <div className={styles.actions}>
          <button
            className={styles.button}
            type="button"
            disabled={cloneBox.isPending}
            onClick={() => cloneBox.mutate()}
          >
            新建同款空盒
          </button>
        </div>
      ) : null}
      {cloneNotice ? <p className={styles.inlineNotice} role="status">{cloneNotice}</p> : null}
      {childBatch.error ? <p role="alert" className={styles.inlineNotice}>{childBatch.error}</p> : null}
      {cloneError ? <p className={styles.error}>{cloneError}</p> : null}
      <section>
        <h2 className={styles.sectionTitle}>子位置</h2>
        <div className={styles.actions}>
          <Link className={styles.button} to={`/locations/new?parent=${record.id}`}>
            新增子位置
          </Link>
          <SelectToggle
            selecting={childBatch.selection.selecting}
            onToggle={() => childBatch.selection.toggleMode()}
            allSelected={childBatch.selection.pageAllSelected(
              (children.data?.data ?? []).filter(locationDeletable).map((row) => row.id),
            )}
            onSelectAll={() =>
              childBatch.selection.selectAll((children.data?.data ?? []).filter(locationDeletable).map((row) => row.id))
            }
            selectAllDisabled={(children.data?.data ?? []).filter(locationDeletable).length === 0}
          />
        </div>
        {children.isLoading ? <Loading /> : null}
        {children.isError ? (
          <LoadError error={children.error} onRetry={() => void children.refetch()} pending={children.isFetching} />
        ) : null}
        {children.data && children.data.total === 0 ? <p>这里还没有下一级位置</p> : null}
        {children.data && children.data.data.length > 0 ? (
          <LocationLinks
            rows={children.data.data}
            from={from}
            selecting={childBatch.selection.selecting}
            selected={childBatch.selection.selected}
            onToggle={(locId) => childBatch.selection.toggle(locId)}
          />
        ) : null}
        {children.data ? (
          <Pager
            offset={children.data.offset}
            limit={children.data.limit}
            total={children.data.total}
            count={children.data.data.length}
            onPage={(offset) => setNamedOffset("children", offset)}
          />
        ) : null}
        {childBatch.selection.selecting ? (
          <BatchBar
            count={(children.data?.data ?? []).filter((row) => childBatch.selection.selected.has(row.id)).length}
            pending={childBatch.pending}
            error={childBatch.error}
            confirming={childBatch.confirm ? "delete" : null}
            confirmText={`永久删除 ${
              (children.data?.data ?? []).filter((row) => childBatch.selection.selected.has(row.id)).length
            } 个位置，随后可短时撤销`}
            onConfirm={() =>
              void childBatch.remove((children.data?.data ?? []).filter((row) => childBatch.selection.selected.has(row.id)))
            }
            onCancel={() => childBatch.setConfirm(false)}
          >
            <button
              className={styles.buttonDanger}
              type="button"
              disabled={(children.data?.data ?? []).filter((row) => childBatch.selection.selected.has(row.id)).length === 0}
              onClick={() => {
                childBatch.setError("");
                childBatch.setConfirm(true);
              }}
            >
              删除
            </button>
          </BatchBar>
        ) : null}
      </section>
      <section>
        <h2 className={styles.sectionTitle}>物品</h2>
        <div className={styles.chips}>
          <button className={!selfOnly ? styles.chipActive : styles.chip} onClick={()=>setParams(current=>{const next=new URLSearchParams(current);next.delete('scope');next.delete('items');return next;})}>包含下级</button>
          <button className={selfOnly ? styles.chipActive : styles.chip} onClick={()=>setParams(current=>{const next=new URLSearchParams(current);next.set('scope','direct');next.delete('items');return next;})}>仅直接存放</button>
        </div>
        {itemBatch.error ? <p role="alert" className={styles.inlineNotice}>{itemBatch.error}</p> : null}
        <SelectToggle
          selecting={itemBatch.selection.selecting}
          onToggle={() => itemBatch.selection.toggleMode()}
          allSelected={itemBatch.selection.pageAllSelected((directItems.data?.data ?? []).map((item) => item.id))}
          onSelectAll={() => itemBatch.selection.selectAll((directItems.data?.data ?? []).map((item) => item.id))}
          selectAllDisabled={(directItems.data?.data ?? []).length === 0}
        />
        {directItems.isLoading ? <Loading /> : null}
        {directItems.isError ? (
          <LoadError error={directItems.error} onRetry={() => void directItems.refetch()} pending={directItems.isFetching} />
        ) : null}
        {directItems.data && directItems.data.total === 0 ? (
          <>
            <p>{selfOnly ? '这里没有直接存放的物品，可以切换「包含下级」查看盒内物品。' : '这个位置及下级位置还没有登记物品。'}</p>
            {record.direct_item_count > 0 ? <p>回收站里还有物品占用这个位置</p> : null}
          </>
        ) : null}
        {directItems.data && directItems.data.data.length > 0 ? (
          <ul className={styles.list}>
            {directItems.data.data.map((item) => (
              <li key={item.id} className={styles.selectRow}>
                <RowCheck
                  label={item.name}
                  selecting={itemBatch.selection.selecting}
                  checked={itemBatch.selection.selected.has(item.id)}
                  onToggle={() => itemBatch.selection.toggle(item.id)}
                />
                {itemBatch.selection.selecting ? (
                  <button className={styles.itemLink} type="button" onClick={() => itemBatch.selection.toggle(item.id)}>
                    <ItemSummary item={item}/>
                  </button>
                ) : (
                  <Link className={styles.itemLink} to={`/items/${item.id}`} state={{from:`/locations/${id}${params.toString() ? `?${params}` : ''}`}}>
                    <ItemSummary item={item}/>
                  </Link>
                )}
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
        {itemBatch.selection.selecting && itemBatch.picking ? (
          <LocationPickPanel
            onPick={(loc) => {
              itemBatch.setDest(loc);
              itemBatch.setPicking(false);
              itemBatch.setConfirm("move");
            }}
            onCancel={() => {
              itemBatch.setPicking(false);
              itemBatch.setDest(null);
            }}
          />
        ) : null}
        {itemBatch.selection.selecting && !itemBatch.picking ? (
          <BatchBar
            count={(directItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length}
            pending={itemBatch.pending}
            error={itemBatch.error}
            confirming={itemBatch.confirm === "trash" ? "trash" : itemBatch.confirm === "move" ? "move" : null}
            confirmText={
              itemBatch.confirm === "trash"
                ? `将 ${(directItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length} 件物品移到回收站`
                : itemBatch.confirm === "move" && itemBatch.dest
                  ? `将物品搬到 ${itemBatch.dest.name}${itemBatch.dest.code ? ` ${itemBatch.dest.code}` : ""}，原来的位置清掉`
                  : undefined
            }
            onConfirm={() => {
              const picked = (directItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id));
              if (itemBatch.confirm === "trash") void itemBatch.trash(picked);
              if (itemBatch.confirm === "move" && itemBatch.dest) void itemBatch.move(picked, itemBatch.dest);
            }}
            onCancel={() => {
              itemBatch.setConfirm(null);
              itemBatch.setDest(null);
            }}
          >
            <button
              className={styles.buttonDanger}
              type="button"
              disabled={(directItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length === 0}
              onClick={() => {
                itemBatch.setError("");
                itemBatch.setConfirm("trash");
              }}
            >
              移到回收站
            </button>
            <button
              className={styles.button}
              type="button"
              disabled={(directItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length === 0}
              onClick={() => {
                itemBatch.setError("");
                itemBatch.setPicking(true);
              }}
            >
              更换存放位置
            </button>
          </BatchBar>
        ) : null}
        <details className={styles.filters}><summary>只记名称，快速添加</summary>
        <form onSubmit={onQuick}>
          <TextField label="名称" name="quick-name" value={quickName} onChange={setQuickName} error={fieldText(quickError, "name")} />
          {fieldText(quickError, "locations") ? <p className={styles.error}>{fieldText(quickError, "locations")}</p> : null}
          <FormIssues error={quickError} />
          <button className={styles.button} type="submit" disabled={quick.isPending}>
            添加物品
          </button>
        </form>
        </details>
      </section>
      <details className={styles.editor}><summary>编辑位置资料</summary>
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
        <IconPicker
          value={icon}
          customIconId={customIconId}
          type={record.type}
          onChange={(nextIcon, nextCustom) => {
            setIcon(nextIcon);
            setCustomIconId(nextCustom);
          }}
          error={fieldText(formError, "icon") || fieldText(formError, "custom_icon_id")}
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
      </details>
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
  const itemBatch = useItemListBatch(`cat-items:${id}:${selfOnly ? "0" : ""}:${itemsRaw ?? ""}`);
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
      {itemBatch.error ? <p role="alert" className={styles.inlineNotice}>{itemBatch.error}</p> : null}
      {itemBatch.undo.message ? (
        <UndoBanner message={itemBatch.undo.message} onUndo={itemBatch.canUndo ? () => void itemBatch.undoTrash() : undefined} onDismiss={() => itemBatch.undo.clear()} />
      ) : null}
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
          <SelectToggle
            selecting={itemBatch.selection.selecting}
            onToggle={() => itemBatch.selection.toggleMode()}
            allSelected={itemBatch.selection.pageAllSelected((branchItems.data?.data ?? []).map((item) => item.id))}
            onSelectAll={() => itemBatch.selection.selectAll((branchItems.data?.data ?? []).map((item) => item.id))}
            selectAllDisabled={(branchItems.data?.data ?? []).length === 0}
          />
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
              <li key={item.id} className={styles.selectRow}>
                <RowCheck
                  label={item.name}
                  selecting={itemBatch.selection.selecting}
                  checked={itemBatch.selection.selected.has(item.id)}
                  onToggle={() => itemBatch.selection.toggle(item.id)}
                />
                {itemBatch.selection.selecting ? (
                  <button className={styles.itemLink} type="button" onClick={() => itemBatch.selection.toggle(item.id)}>
                    <ItemSummary item={item}/>
                  </button>
                ) : (
                  <Link className={styles.itemLink} to={`/items/${item.id}`} state={{from:`/categories/${id}${params.toString() ? `?${params}` : ''}`}}>
                    <ItemSummary item={item}/>
                  </Link>
                )}
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
        {itemBatch.selection.selecting && itemBatch.picking ? (
          <LocationPickPanel
            onPick={(loc) => {
              itemBatch.setDest(loc);
              itemBatch.setPicking(false);
              itemBatch.setConfirm("move");
            }}
            onCancel={() => {
              itemBatch.setPicking(false);
              itemBatch.setDest(null);
            }}
          />
        ) : null}
        {itemBatch.selection.selecting && !itemBatch.picking ? (
          <BatchBar
            count={(branchItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length}
            pending={itemBatch.pending}
            error={itemBatch.error}
            confirming={itemBatch.confirm === "trash" ? "trash" : itemBatch.confirm === "move" ? "move" : null}
            confirmText={
              itemBatch.confirm === "trash"
                ? `将 ${(branchItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length} 件物品移到回收站`
                : itemBatch.confirm === "move" && itemBatch.dest
                  ? `将物品搬到 ${itemBatch.dest.name}${itemBatch.dest.code ? ` ${itemBatch.dest.code}` : ""}，原来的位置清掉`
                  : undefined
            }
            onConfirm={() => {
              const picked = (branchItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id));
              if (itemBatch.confirm === "trash") void itemBatch.trash(picked);
              if (itemBatch.confirm === "move" && itemBatch.dest) void itemBatch.move(picked, itemBatch.dest);
            }}
            onCancel={() => {
              itemBatch.setConfirm(null);
              itemBatch.setDest(null);
            }}
          >
            <button
              className={styles.buttonDanger}
              type="button"
              disabled={(branchItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length === 0}
              onClick={() => {
                itemBatch.setError("");
                itemBatch.setConfirm("trash");
              }}
            >
              移到回收站
            </button>
            <button
              className={styles.button}
              type="button"
              disabled={(branchItems.data?.data ?? []).filter((item) => itemBatch.selection.selected.has(item.id)).length === 0}
              onClick={() => {
                itemBatch.setError("");
                itemBatch.setPicking(true);
              }}
            >
              更换存放位置
            </button>
          </BatchBar>
        ) : null}
      </section>
      <details className={styles.editor}><summary>编辑分类资料</summary>
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
      </details>
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

  const batch = useTrashListBatch(offsetRaw ?? "");
  const selectedItems = (page?.data ?? []).filter((item) => batch.selection.selected.has(item.id));

  return (
    <>
      <PageHeading title="回收站" description="暂时不需要的记录，仍可以恢复。"/>
      {batch.error ? <p role="alert" className={styles.inlineNotice}>{batch.error}</p> : null}
      <SelectToggle
        selecting={batch.selection.selecting}
        onToggle={() => batch.selection.toggleMode()}
        allSelected={batch.selection.pageAllSelected((page?.data ?? []).map((item) => item.id))}
        onSelectAll={() => batch.selection.selectAll((page?.data ?? []).map((item) => item.id))}
        selectAllDisabled={!page || page.data.length === 0}
      />
      {query.isLoading ? <Loading /> : null}
      {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
      {page && page.total === 0 ? <p>回收站是空的</p> : null}
      {page && page.data.length > 0 ? (
        <ul className={styles.list}>
          {page.data.map((item) => (
            <li key={item.id} className={styles.selectRow}>
              <RowCheck
                label={item.name}
                selecting={batch.selection.selecting}
                checked={batch.selection.selected.has(item.id)}
                onToggle={() => batch.selection.toggle(item.id)}
              />
              {batch.selection.selecting ? (
                <button className={styles.itemLink} type="button" onClick={() => batch.selection.toggle(item.id)}>
                  <ItemThumb photoId={item.photos[0]?.id} />
                  <span className={styles.itemLinkBody}>
                    <span>{item.name}</span>
                    <span className={styles.meta}>{item.deleted_at ?? ""}</span>
                  </span>
                </button>
              ) : (
                <Link className={styles.itemLink} to={`/trash/${item.id}`}>
                  <ItemThumb photoId={item.photos[0]?.id} />
                  <span className={styles.itemLinkBody}>
                    <span>{item.name}</span>
                    <span className={styles.meta}>{item.deleted_at ?? ""}</span>
                  </span>
                </Link>
              )}
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
      {batch.selection.selecting ? (
        <BatchBar
          count={selectedItems.length}
          pending={batch.pending}
          error={batch.error}
          confirming={batch.confirm ? "restore" : null}
          confirmText={`恢复 ${selectedItems.length} 件物品`}
          onConfirm={() => void batch.restore(selectedItems)}
          onCancel={() => batch.setConfirm(false)}
        >
          <button
            className={styles.button}
            type="button"
            disabled={selectedItems.length === 0 || batch.pending}
            onClick={() => batch.setConfirm(true)}
          >
            恢复
          </button>
        </BatchBar>
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
      <p className={styles.meta}>选一种看着舒服的外观。设置自动保存在当前浏览器。</p>
      <fieldset className={styles.field}>
        <legend>亮度</legend>
        <div className={styles.appearanceChoices}>
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
        <div className={styles.appearanceChoices}>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "moss"} onChange={() => pickAccent("moss")} />
            <span className={styles.colorSwatch} style={{background:'#27654c'}} aria-hidden="true"/>
            苔绿
          </label>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "clay"} onChange={() => pickAccent("clay")} />
            <span className={styles.colorSwatch} style={{background:'#8f4d36'}} aria-hidden="true"/>
            陶土
          </label>
          <label className={styles.filterChoice}>
            <input type="radio" name="accent" checked={accent === "ink"} onChange={() => pickAccent("ink")} />
            <span className={styles.colorSwatch} style={{background:'#3d5a73'}} aria-hidden="true"/>
            墨蓝
          </label>
        </div>
      </fieldset>
    </section>
  );
}

function IconLibrarySection({ visible }: {visible: boolean}) {
  const query = useIconLibrary();
  const queryClient = useQueryClient();
  const locations = useQuery({ queryKey: ["locations", "icon-usage"], queryFn: () => fetchAllLocations(new URLSearchParams({ flat: "1" })) });
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<number | null>(null);
  const [names, setNames] = useState<Record<number, string>>({});
  const [draft, setDraft] = useState<{ name: string; svg: string } | null>(null);
  const [uploadError, setUploadError] = useState("");
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [notice, setNotice] = useState('');
  const upload = useMutation({
    mutationFn: (input: { name: string; svg: string }) => createLocationIcon(input.name, input.svg),
    onSuccess: created => { setDraft(null); setUploadError(""); setNotice(`已上传「${created.name}」`); void queryClient.invalidateQueries({ queryKey: ["location-icons"] }); },
    onError: error => { const api = asApiError(error, "无法上传图标"); setUploadError(fieldText(api, "svg") || fieldText(api, "name") || api.message); },
  });
  const saveName = useMutation({
    mutationFn: (icon: LocationIconRecord) => updateLocationIcon(icon.id, { version: icon.version, name: names[icon.id] ?? icon.name }),
    onSuccess: updated => { setFormError(null); setNames(current => ({ ...current, [updated.id]: updated.name })); setSelected(null); setNotice('图标名称已保存'); void queryClient.invalidateQueries({ queryKey: ["location-icons"] }); },
    onError: error => setFormError(asApiError(error, "无法保存图标")),
  });
  const remove = useMutation({
    mutationFn: (icon: LocationIconRecord) => deleteLocationIcon(icon.id, icon.version),
    onSuccess: () => { setFormError(null); setDeleting(null); setSelected(null); setNotice('图标已删除'); void queryClient.invalidateQueries({ queryKey: ["location-icons"] }); },
    onError: error => { setFormError(asApiError(error, isIconInUse(error) ? "有位置正在使用" : "无法删除图标")); },
  });
  useOnUnauth(query.error);
  useOnUnauth(locations.error);
  useOnUnauth(upload.error);
  useOnUnauth(saveName.error);
  useOnUnauth(remove.error);
  async function onFile(file: File | undefined) {
    if (!file) return;
    setUploadError("");
    try { setDraft({ name: file.name.replace(/\.svg$/i, "").trim() || "图标", svg: await file.text() }); }
    catch { setUploadError("无法读取文件，请重新选择"); }
  }
  const icons = query.data ?? [];
  const active = icons.find(icon => icon.id === selected);
  const used = active ? (locations.data ?? []).filter(location => location.custom_icon_id === active.id) : [];
  const shown = icons.filter(icon => icon.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()));
  return <section>
    {notice ? <p className={styles.inlineNotice} role="status">{notice}</p> : null}
    <div className={styles.actions}>
      <label className={styles.field}>搜索图标<input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="输入图标名称" /></label>
      <label className={styles.iconUpload}>上传 SVG<input type="file" accept="image/svg+xml,.svg" disabled={upload.isPending} onChange={event => { const file = event.target.files?.[0]; event.target.value = ""; void onFile(file); }} /></label>
    </div>
    {query.isLoading ? <Loading /> : null}
    {query.isError ? <LoadError error={query.error} onRetry={() => void query.refetch()} pending={query.isFetching} /> : null}
    {query.isSuccess && !icons.length ? <p>还没有自己的图标，上传 SVG 后可用于位置。</p> : null}
    {icons.length > 0 && !shown.length ? <p>没有匹配的图标</p> : null}
    <ul className={styles.iconLibraryGrid}>{shown.map(icon => <li key={icon.id}>
      <button className={styles.iconLibraryCard} type="button" onClick={() => { setSelected(icon.id); setFormError(null); void locations.refetch(); }}>
        <CustomSVG svg={icon.svg} className={styles.iconLibraryPreview} /><span>{icon.name}</span><small>查看与修改</small>
      </button>
    </li>)}</ul>
    {active && visible ? <Dialog title={active.name} onClose={() => setSelected(null)} busy={saveName.isPending || remove.isPending}>
      <div className={styles.iconDetailPanel}>
        <CustomSVG svg={active.svg} className={styles.iconLibraryPreview} />
        <form onSubmit={event => { event.preventDefault(); setFormError(null); saveName.mutate(active); }}>
          <label className={styles.field}>图标名称<input required disabled={saveName.isPending || remove.isPending} value={names[active.id] ?? active.name} onChange={event => setNames(current => ({ ...current, [active.id]: event.target.value }))} /></label>
          <button className={styles.buttonPrimary} disabled={saveName.isPending || remove.isPending}>保存名称</button>
        </form>
        <h3>使用位置</h3>
        {locations.isLoading ? <Loading /> : null}
        {locations.isError ? <LoadError error={locations.error} onRetry={() => void locations.refetch()} pending={locations.isFetching} /> : null}
        {used.length ? <><ul>{used.map(location => <li key={location.id}><Link to={`/locations/${location.id}`}>{location.path.map(node => node.name).join(" / ") || location.name}</Link></li>)}</ul><p>正在使用的图标不能删除，请先为这些位置更换图标。</p></> : locations.isSuccess ? <p>目前没有位置使用此图标。</p> : null}
        {locations.isSuccess && !locations.isFetching && !used.length ? <DeleteConfirm confirming={deleting === active.id} pending={remove.isPending} error={deleting === active.id ? formError : null} onAsk={() => { setDeleting(active.id); setFormError(null); }} onCancel={() => { setDeleting(null); setFormError(null); }} onConfirm={() => remove.mutate(active)} /> : null}
        {deleting == null ? <FormIssues error={formError} /> : null}
        <button data-dialog-cancel className={styles.button} type="button" disabled={saveName.isPending || remove.isPending} onClick={() => setSelected(null)}>关闭</button>
      </div>
    </Dialog> : null}
    {draft && visible ? <Dialog title="预览上传图标" onClose={() => setDraft(null)} busy={upload.isPending}>
      <form onSubmit={event => { event.preventDefault(); setUploadError(""); upload.mutate(draft); }}>
        <img className={styles.uploadPreview} src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(draft.svg)}`} alt="待上传图标预览" />
        <label className={styles.field}>图标名称<input required disabled={upload.isPending} value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} /></label>
        <p>确认后上传，系统会检查 SVG 内容。</p>
        {uploadError ? <p className={styles.error} role="alert">{uploadError}</p> : null}
        <div className={styles.actions}><button data-dialog-cancel className={styles.button} type="button" disabled={upload.isPending} onClick={() => setDraft(null)}>取消</button><button className={styles.buttonPrimary} disabled={upload.isPending}>确认上传</button></div>
      </form>
    </Dialog> : uploadError ? <p className={styles.error}>{uploadError}</p> : null}
  </section>;
}

export function AccountPage() {
  const { pathname } = useLocation();
  const section = pathname.split("/")[2] ?? "";
  const titles: Record<string, string> = { appearance: "外观", icons: "图标库", security: "账号安全", integrations: "助手接入" };
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

  if (section && !titles[section]) return <Navigate to="/account" replace />;

  return (
    <>
      {section ? <Link className={styles.back} to="/account">← 返回设置</Link> : null}
      <h1 className={styles.title} tabIndex={-1}>{titles[section] ?? "设置"}</h1>
      {!section ? <div className={styles.settingsGrid}>{[
        ["appearance", "外观", "调整亮度与强调色"], ["icons", "图标库", "上传与管理位置图标"],
        ["security", "账号安全", "用户名、密码与退出登录"], ["integrations", "助手接入", "创建与管理接入令牌"],
        ["trash", "回收站", "查看已删除物品与恢复"],
      ].map(([key, title, description]) => <Link className={styles.settingsCard} key={key} to={key === "trash" ? "/trash" : `/account/${key}`}><h2>{title}</h2><p>{description}</p><span aria-hidden="true">›</span></Link>)}</div> : null}
      <div className={styles.settingsSection} hidden={section !== "appearance"}><AppearanceFields /></div>
      <div className={styles.settingsSection} hidden={section !== "icons"}><IconLibrarySection visible={section === 'icons'}/></div>
      <div className={styles.settingsSection} hidden={section !== "security"}>
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
      {usernameNotice ? <p className={styles.inlineNotice} role="status">{usernameNotice}</p> : null}
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
      {passwordNotice ? <p className={styles.inlineNotice} role="status">{passwordNotice}</p> : null}
      </div>
      <div className={styles.settingsSection} hidden={section !== "integrations"}>
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
      </div>
      <div hidden={section !== "security"}>
      {signOut.isError ? <p className={styles.error}>{messageOf(signOut.error, "退出失败")}</p> : null}
      <button className={styles.button} type="button" onClick={() => signOut.mutate()} disabled={signOut.isPending}>
        退出登录
      </button>
      </div>
    </>
  );
}
