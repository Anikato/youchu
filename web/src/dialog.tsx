import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import styles from './styles.module.css';

let openDialogs = 0;
let savedOverflow = '';

export function Dialog({title, children, onClose, busy = false, returnFocusLabel}: {title: string; children: ReactNode; onClose: () => void; busy?: boolean; returnFocusLabel?: string}) {
  const ref = useRef<HTMLDialogElement>(null);
  const trigger = useRef(document.activeElement instanceof HTMLElement ? document.activeElement : null);
  const triggerText = useRef(trigger.current?.textContent?.trim());
  const titleId = useId();
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    const previous = trigger.current;
    if (openDialogs === 0) savedOverflow = document.documentElement.style.overflow;
    openDialogs += 1;
    document.documentElement.style.overflow = 'hidden';
    dialog.showModal();
    dialog.querySelector<HTMLElement>('[data-dialog-cancel]')?.focus();
    return () => {
      dialog.close();
      openDialogs -= 1;
      if (openDialogs === 0) document.documentElement.style.overflow = savedOverflow;
      requestAnimationFrame(() => {
        const top = Array.from(document.querySelectorAll('dialog[open]')).at(-1);
        if (previous?.isConnected && (!top || top.contains(previous))) previous.focus();
        else {
          const scope = top ?? document;
          const buttons = Array.from(scope.querySelectorAll('button'));
          const next = buttons.find(button=>button.textContent?.trim()===(returnFocusLabel ?? triggerText.current));
          const fallback = scope.querySelector<HTMLElement>('[data-dialog-cancel], button[data-dialog-fallback], main h1[tabindex]');
          (next ?? fallback)?.focus({preventScroll:true});
        }
      });
    };
  }, []);
  return createPortal(<dialog ref={ref} className={styles.dialog} aria-labelledby={titleId} aria-busy={busy} onCancel={event=>{event.preventDefault();event.stopPropagation();if (!busy) onClose();}}>
    <div className={styles.dialogHeading}><h2 id={titleId}>{title}</h2><button type="button" className={styles.dialogClose} aria-label="关闭弹窗" disabled={busy} onClick={onClose}>×</button></div>
    <div className={styles.dialogBody}>{children}</div>
  </dialog>,document.body);
}
