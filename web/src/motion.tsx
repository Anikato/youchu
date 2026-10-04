import { type ReactNode } from "react";
import styles from './styles.module.css';

export function EnterBox({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={`${className ?? ''} ${styles.surfaceEnter}`}>
      {children}
    </div>
  );
}

export function PageEnter({ pathname, className, children }: { pathname: string; className?: string; children: ReactNode }) {
  // Fixed action bars and dialogs must keep the viewport as their containing block.
  return (
    <div className={className} data-page={pathname}>
      {children}
    </div>
  );
}

export function StaggerList({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <ul className={className}>
      {children}
    </ul>
  );
}
