import { useEffect, useState } from "react";
import styles from "./styles.module.css";

export function StickySave({
  form,
  disabled,
  label,
  error,
}: {
  form: string;
  disabled: boolean;
  label: string;
  error?: string;
}) {
  const [lift, setLift] = useState(0);
  useEffect(() => {
    const view = window.visualViewport;
    if (!view) return;
    const port = view;
    function update() {
      const keyboard = Math.max(0, window.innerHeight - port.height - port.offsetTop);
      setLift(keyboard);
    }
    update();
    port.addEventListener("resize", update);
    port.addEventListener("scroll", update);
    return () => {
      port.removeEventListener("resize", update);
      port.removeEventListener("scroll", update);
    };
  }, []);
  const style = lift > 40 ? { bottom: `${lift}px` } : undefined;
  return (
    <div className={styles.stickySave} style={style}>
      {error ? <p className={styles.error}>{error}</p> : null}
      <button className={styles.buttonPrimary} type="submit" form={form} disabled={disabled}>
        {label}
      </button>
    </div>
  );
}

export function saveButtonLabel(saving: boolean, uploadIndex: number, uploadTotal: number, justSaved: boolean): string {
  if (justSaved) return "已保存";
  if (saving && uploadTotal > 0) return `上传 ${uploadIndex}/${uploadTotal}`;
  if (saving) return "保存中";
  return "保存";
}
