import { useEffect, useMemo } from "react";
import styles from "./styles.module.css";

export function PendingPhotos({ files, onRemove }: { files: File[]; onRemove: (index: number) => void }) {
  const urls = useMemo(() => files.map((file) => URL.createObjectURL(file)), [files]);
  useEffect(() => {
    return () => {
      for (const url of urls) URL.revokeObjectURL(url);
    };
  }, [urls]);
  if (files.length === 0) return null;
  return (
    <ul className={styles.photoThumbs}>
      {files.map((file, index) => (
        <li key={`${file.name}-${file.size}-${file.lastModified}-${index}`}>
          <img className={styles.itemThumb} src={urls[index]} alt="" />
          <button className={styles.button} type="button" onClick={() => onRemove(index)}>
            去掉
          </button>
        </li>
      ))}
    </ul>
  );
}
