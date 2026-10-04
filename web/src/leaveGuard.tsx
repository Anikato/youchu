import { useEffect } from "react";
import { useBlocker } from "react-router";
import styles from "./styles.module.css";
import { Dialog } from './dialog';

export function LeaveGuard({ dirty }: { dirty: boolean }) {
  const blocker = useBlocker(dirty);
  useEffect(() => {
    if (!dirty) return;
    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);
  if (blocker.state !== "blocked") return null;
  return (
    <Dialog title="离开前，保存好了吗？" onClose={()=>blocker.reset()}>
        <p>尚未保存的内容会丢失。你可以留下继续编辑。</p>
        <div className={styles.stack}>
          <button className={styles.buttonDanger} type="button" onClick={() => blocker.proceed()}>
            放弃并离开
          </button>
          <button className={styles.button} data-dialog-cancel type="button" onClick={() => blocker.reset()}>
            继续编辑
          </button>
        </div>
    </Dialog>
  );
}
