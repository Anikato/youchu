import { useEffect } from "react";
import { useBlocker } from "react-router";
import styles from "./styles.module.css";

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
    <div className={styles.leaveGuard} role="dialog" aria-modal="true">
      <div className={styles.leaveGuardPanel}>
        <p>尚未保存，要离开吗？</p>
        <div className={styles.stack}>
          <button className={styles.buttonDanger} type="button" onClick={() => blocker.proceed()}>
            离开
          </button>
          <button className={styles.button} type="button" onClick={() => blocker.reset()}>
            留下
          </button>
        </div>
      </div>
    </div>
  );
}
