import { type ChangeEvent } from "react";
import styles from "./styles.module.css";

const accept = "image/jpeg,image/png,image/webp";

export function PhotoAddButtons({
  disabled,
  remaining,
  onFiles,
}: {
  disabled: boolean;
  remaining: number;
  onFiles: (files: File[]) => void;
}) {
  if (remaining <= 0) return null;

  function pick(event: ChangeEvent<HTMLInputElement>, max: number) {
    const files = Array.from(event.currentTarget.files ?? []).slice(0, max);
    event.currentTarget.value = "";
    if (files.length === 0) return;
    onFiles(files);
  }

  return (
    <div className={styles.actions}>
      <label className={styles.fileButton}>
        拍照
        <input type="file" accept={accept} capture="environment" disabled={disabled} onChange={(event) => pick(event, 1)} />
      </label>
      <label className={styles.fileButton}>
        相册
        <input type="file" accept={accept} multiple disabled={disabled} onChange={(event) => pick(event, remaining)} />
      </label>
    </div>
  );
}
