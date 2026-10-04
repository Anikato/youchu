import { pageCount, pageIndex } from "./filters";
import styles from "./styles.module.css";

export function Pager({
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
  if (total <= 0) return null;
  const pages = pageCount(total, step);
  const current = pageIndex(offset, step);
  return (
    <div className={styles.pager}>
      <p className={styles.pageMeta}>
        第 {current} / {pages} 页
      </p>
      {hasPrev || hasNext ? (
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
      ) : null}
    </div>
  );
}
