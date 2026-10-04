import { ApiError, type Location, type LocationCreate } from "./api";

export type BatchFailure<T> = { row: T; message: string };

export type BatchOutcome<T> = { done: T[]; failed: BatchFailure<T>[] };

export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

export async function runSequential<T>(rows: T[], act: (row: T) => Promise<void>, fallback: string): Promise<BatchOutcome<T>> {
  const done: T[] = [];
  const failed: BatchFailure<T>[] = [];
  for (const row of rows) {
    try {
      await act(row);
      done.push(row);
    } catch (error) {
      failed.push({ row, message: errorMessage(error, fallback) });
    }
  }
  return { done, failed };
}

export function batchSummary(ok: number, failed: BatchFailure<unknown>[]): string {
  if (failed.length === 0) return `成功 ${ok}`;
  const reasons = failed.map((item) => item.message).join("；");
  return `成功 ${ok}，失败 ${failed.length}：${reasons}`;
}

export function locationCreateFrom(loc: Location): LocationCreate {
  const body: LocationCreate = { name: loc.name, type: loc.type };
  if (loc.code != null) body.code = loc.code;
  if (loc.parent_id != null) body.parent_id = loc.parent_id;
  if (loc.icon != null) body.icon = loc.icon;
  if (loc.custom_icon_id != null) body.custom_icon_id = loc.custom_icon_id;
  return body;
}
