import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  completeReturnTask,
  createLocation,
  deleteItem,
  deleteLocation,
  fetchAllLocations,
  restoreTrashItem,
  updateItem,
  type Item,
  type Location,
  type ReturnTask,
} from "./api";
import { batchSummary, locationCreateFrom, runSequential } from "./batch";
import { filterLocations } from "./locationSearch";
import { pageAllSelected, selectPageIds, toggleSelected } from "./select";
import styles from "./styles.module.css";
import { Dialog } from './dialog';
import { QueryError } from './catalogUi';

export function useListSelection(pageKey: string) {
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  useEffect(() => {
    setSelecting(false);
    setSelected(new Set());
  }, [pageKey]);

  function toggleMode() {
    if (selecting) {
      setSelecting(false);
      setSelected(new Set());
      return;
    }
    setSelecting(true);
  }

  function toggle(id: number) {
    setSelected((current) => toggleSelected(current, id));
  }

  function selectAll(ids: number[]) {
    setSelected((current) => selectPageIds(ids, current));
  }

  function clear() {
    setSelected(new Set());
    setSelecting(false);
  }

  function retainFailed(ids: number[]) {
    setSelected(new Set(ids));
    setSelecting(ids.length > 0);
  }

  return { selecting, selected, toggleMode, toggle, selectAll, clear, retainFailed, pageAllSelected: (ids: number[]) => pageAllSelected(ids, selected) };
}

export function SelectToggle({
  selecting,
  onToggle,
  allSelected,
  onSelectAll,
  selectAllDisabled,
}: {
  selecting: boolean;
  onToggle: () => void;
  allSelected: boolean;
  onSelectAll: () => void;
  selectAllDisabled?: boolean;
}) {
  return (
    <div className={styles.actions}>
      <button className={styles.button} data-dialog-fallback type="button" onClick={onToggle}>
        {selecting ? "完成" : "选择"}
      </button>
      {selecting ? (
        <button className={styles.button} type="button" onClick={onSelectAll} disabled={selectAllDisabled}>
          {allSelected ? "取消全选" : "本页全选"}
        </button>
      ) : null}
    </div>
  );
}

export function RowCheck({
  selecting,
  checked,
  disabled,
  onToggle,
  label,
}: {
  selecting: boolean;
  checked: boolean;
  disabled?: boolean;
  onToggle: () => void;
  label?: string;
}) {
  if (!selecting) return null;
  return (
    <input
      type="checkbox"
      checked={checked}
      disabled={disabled}
      onChange={() => {
        if (disabled) return;
        onToggle();
      }}
      aria-label={`${disabled ? '不能选择' : '选择'}${label ? ` ${label}` : ''}`}
    />
  );
}

export function UndoBanner({ message, onUndo, onDismiss }: { message: string; onUndo?: () => void; onDismiss: () => void }) {
  return (
    <div className={styles.undoBanner} role="status" aria-live="polite">
      <p>{message}</p>
      <div className={styles.actions}>
        {onUndo ? <button className={styles.button} type="button" onClick={onUndo}>
          撤销
        </button> : null}
        <button className={styles.button} type="button" onClick={onDismiss}>
          关闭
        </button>
      </div>
    </div>
  );
}

export function BatchBar({
  count,
  pending,
  error,
  confirming,
  confirmText,
  onConfirm,
  onCancel,
  children,
}: {
  count: number;
  pending: boolean;
  error: string;
  confirming: string | null;
  confirmText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  children: ReactNode;
}) {
  if (count === 0 && !confirming) return null;
  return (
    <>
      <div className={styles.batchBarSpacer} />
      <div className={styles.batchBar}>
        {error ? <p className={styles.error}>{error}</p> : null}
        {confirming ? (
          <Dialog title="确认批量操作" onClose={onCancel} busy={pending} returnFocusLabel={confirming === 'move' ? '更换存放位置' : undefined}>
            <p>{confirmText ?? confirming}</p>
            <div className={styles.actions}>
              <button className={confirming === 'trash' || /删除|回收站/.test(confirmText ?? confirming ?? '') ? styles.buttonDanger : styles.buttonPrimary} type="button" onClick={onConfirm} disabled={pending}>
                {pending ? '正在处理…' : '确认操作'}
              </button>
              <button className={styles.button} data-dialog-cancel type="button" onClick={onCancel} disabled={pending}>
                取消
              </button>
            </div>
          </Dialog>
        ) : (
          <>
            <p>已选 {count}</p>
            <div className={styles.actions}>{children}</div>
          </>
        )}
      </div>
    </>
  );
}

export function LocationPickPanel({ onPick, onCancel }: { onPick: (loc: Location) => void; onCancel: () => void }) {
  const options = useQuery({
    queryKey: ["locations", "flat"],
    queryFn: () => fetchAllLocations(new URLSearchParams({ flat: "1" })),
    retry: false,
  });
  const [query, setQuery] = useState("");
  const matches = filterLocations(query, options.data ?? []);
  return (
    <Dialog title="更换存放位置" onClose={onCancel} returnFocusLabel="更换存放位置">
      <label className={styles.field}>
        选择新的位置
        <input
          type="search"
          name="move_location_q"
          value={query}
          autoComplete="off"
          placeholder="找位置"
          onChange={(event) => setQuery(event.target.value)}
        />
      </label>
      {options.isLoading ? <p>正在读取位置</p> : null}
      {options.isError ? <QueryError error={options.error} retry={()=>void options.refetch()} pending={options.isFetching}/> : null}
      {options.data ? (
        <ul className={styles.locationChoices}>
          {matches.length === 0 ? <li>没有符合的位置</li> : null}
          {matches.map((loc) => (
            <li key={loc.id}>
              <button className={styles.button} type="button" onClick={() => onPick(loc)}>
                {loc.path.map((node) => (node.code ? `${node.name} ${node.code}` : node.name)).join(" / ")}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      <button className={styles.button} data-dialog-cancel type="button" onClick={onCancel}>
        取消
      </button>
    </Dialog>
  );
}

export function useUndoBanner(timeoutMs = 10000) {
  const [message, setMessage] = useState("");
  const [token, setToken] = useState(0);
  useEffect(() => {
    if (!message) return;
    const timer = window.setTimeout(() => setMessage(""), timeoutMs);
    return () => window.clearTimeout(timer);
  }, [message, token, timeoutMs]);
  return {
    message,
    show(next: string) {
      setMessage(next);
      setToken((n) => n + 1);
    },
    clear() {
      setMessage("");
    },
  };
}

export function useItemListBatch(pageKey: string) {
  const queryClient = useQueryClient();
  const selection = useListSelection(pageKey);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState<null | "trash" | "move">(null);
  const [picking, setPicking] = useState(false);
  const [dest, setDest] = useState<Location | null>(null);
  const undo = useUndoBanner();
  const [undoItems, setUndoItems] = useState<{ id: number; version: number }[]>([]);

  async function refresh() {
    await queryClient.invalidateQueries({ queryKey: ["items"] });
    await queryClient.invalidateQueries({ queryKey: ["locations"] });
    await queryClient.invalidateQueries({ queryKey: ["trash"] });
    await queryClient.invalidateQueries({queryKey:['categories']});
    await queryClient.invalidateQueries({queryKey:['item']});
    await queryClient.invalidateQueries({queryKey:['location']});
    await queryClient.invalidateQueries({queryKey:['category']});
    await queryClient.invalidateQueries({queryKey:['return-tasks']});
  }

  async function trash(items: Item[]) {
    setPending(true);
    setError("");
    const result = await runSequential(items, (item) => deleteItem(String(item.id), item.version), "无法删除");
    setPending(false);
    setConfirm(null);
    selection.retainFailed(result.failed.map(({row})=>row.id));
    setUndoItems(result.done.map((item) => ({ id: item.id, version: item.version + 1 })));
    if(result.failed.length) setError(batchSummary(result.done.length,result.failed));
    undo.show(result.failed.length === 0 ? `已将 ${result.done.length} 件移到回收站` : batchSummary(result.done.length, result.failed));
    await refresh();
  }

  async function move(items: Item[], loc: Location) {
    setPending(true);
    setError("");
    const result = await runSequential(
      items,
      (item) =>
        updateItem(String(item.id), {
          name: item.name,
          version: item.version,
          locations: [{ location_id: loc.id }],
        }).then(() => undefined),
      "无法搬家",
    );
    setPending(false);
    setConfirm(null);
    setPicking(false);
    setDest(null);
    selection.retainFailed(result.failed.map(({row})=>row.id));
    undo.clear();
    setUndoItems([]);
    setError(result.failed.length === 0 ? "" : batchSummary(result.done.length, result.failed));
    if (result.failed.length === 0) undo.show(`已更换 ${result.done.length} 件物品的存放位置`);
    await refresh();
  }

  async function undoTrash() {
    if (undoItems.length === 0) return;
    setPending(true);
    const result = await runSequential(
      undoItems,
      (row) => restoreTrashItem(String(row.id), row.version).then(() => undefined),
      "无法恢复",
    );
    setPending(false);
    setUndoItems([]);
    undo.clear();
    if (result.failed.length > 0) setError(batchSummary(result.done.length, result.failed));
    await refresh();
  }

  return {
    selection,
    pending,
    error,
    setError,
    confirm,
    setConfirm,
    picking,
    setPicking,
    dest,
    setDest,
    undo,
    trash,
    move,
    undoTrash,
    canUndo: undoItems.length > 0,
  };
}

export function useLocationListBatch(pageKey: string) {
  const queryClient = useQueryClient();
  const selection = useListSelection(pageKey);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  const undo = useUndoBanner();
  const [snapshots, setSnapshots] = useState<Location[]>([]);

  async function refresh() {
    await queryClient.invalidateQueries({ queryKey: ["locations"] });
  }

  async function remove(rows: Location[]) {
    setPending(true);
    setError("");
    const result = await runSequential(rows, (loc) => deleteLocation(String(loc.id), loc.version), "无法删除");
    setPending(false);
    setConfirm(false);
    selection.retainFailed(result.failed.map(({row})=>row.id));
    setSnapshots(result.done);
    if(result.failed.length) setError(batchSummary(result.done.length,result.failed));
    undo.show(result.failed.length === 0 ? `已删除 ${result.done.length} 个位置` : batchSummary(result.done.length, result.failed));
    await refresh();
  }

  async function undoDelete() {
    if (snapshots.length === 0) return;
    setPending(true);
    const result = await runSequential(
      [...snapshots].reverse(),
      (loc) => createLocation(locationCreateFrom(loc)).then(() => undefined),
      "无法恢复位置",
    );
    setPending(false);
    setSnapshots([]);
    undo.clear();
    if (result.failed.length > 0) setError(batchSummary(result.done.length, result.failed));
    await refresh();
  }

  return { selection, pending, error, setError, confirm, setConfirm, undo, remove, undoDelete };
}

export function useTrashListBatch(pageKey: string) {
  const queryClient = useQueryClient();
  const selection = useListSelection(pageKey);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);

  async function restore(items: Item[]) {
    setPending(true);
    setError("");
    const result = await runSequential(
      items,
      (item) => restoreTrashItem(String(item.id), item.version).then(() => undefined),
      "无法恢复",
    );
    setPending(false);
    setConfirm(false);
    selection.retainFailed(result.failed.map(({row})=>row.id));
    if (result.failed.length > 0) setError(batchSummary(result.done.length, result.failed));
    await queryClient.invalidateQueries({ queryKey: ["trash"] });
    await queryClient.invalidateQueries({ queryKey: ["items"] });
    await queryClient.invalidateQueries({ queryKey: ["item"] });
    await queryClient.invalidateQueries({ queryKey: ["locations"] });
    await queryClient.invalidateQueries({ queryKey: ["location"] });
    await queryClient.invalidateQueries({ queryKey: ["categories"] });
    await queryClient.invalidateQueries({ queryKey: ["category"] });
  }

  return { selection, pending, error, confirm, setConfirm, restore };
}

export function useReturnListBatch(pageKey: string) {
  const queryClient = useQueryClient();
  const selection = useListSelection(pageKey);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);

  async function complete(tasks: ReturnTask[]) {
    setPending(true);
    setError("");
    const result = await runSequential(
      tasks,
      (task) => completeReturnTask(task.id, task.version).then(() => undefined),
      "无法完成归位",
    );
    setPending(false);
    setConfirm(false);
    selection.retainFailed(result.failed.map(({row})=>row.id));
    if (result.failed.length > 0) setError(batchSummary(result.done.length, result.failed));
    await queryClient.invalidateQueries({ queryKey: ["return-tasks"] });
    await queryClient.invalidateQueries({ queryKey: ["items"] });
    await queryClient.invalidateQueries({ queryKey: ["item"] });
  }

  return { selection, pending, error, confirm, setConfirm, complete };
}
