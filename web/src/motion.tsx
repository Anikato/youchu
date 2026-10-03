import { useGSAP } from "@gsap/react";
import gsap from "gsap";
import { type ReactNode, type RefObject, useRef } from "react";

const duration = 0.35;
const ease = "power2.out";
const listStagger = 0.03;

function tweenEnter(target: gsap.TweenTarget, y: number, stagger?: number) {
  const mm = gsap.matchMedia();
  mm.add(
    {
      reduce: "(prefers-reduced-motion: reduce)",
      motion: "(prefers-reduced-motion: no-preference)",
    },
    (context) => {
      const reduce = Boolean(context.conditions?.reduce);
      gsap.fromTo(
        target,
        { autoAlpha: 0, y },
        {
          autoAlpha: 1,
          y: 0,
          duration: reduce ? 0 : duration,
          ease,
          stagger: reduce ? 0 : stagger,
        },
      );
    },
  );
  return () => mm.revert();
}

export function useEnter(ref: RefObject<HTMLElement | null>, deps: unknown[] = []) {
  useGSAP(
    () => {
      const el = ref.current;
      if (!el) return;
      return tweenEnter(el, 12);
    },
    { scope: ref, dependencies: deps, revertOnUpdate: deps.length > 0 },
  );
}

export function EnterBox({ className, children }: { className?: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  useEnter(ref);
  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}

export function PageEnter({ pathname, className, children }: { pathname: string; className?: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  useEnter(ref, [pathname]);
  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}

export function StaggerList({ className, children }: { className?: string; children: ReactNode }) {
  const ref = useRef<HTMLUListElement>(null);
  const played = useRef(false);
  useGSAP(
    () => {
      const root = ref.current;
      if (!root || played.current) return;
      const items = root.querySelectorAll(":scope > li");
      if (items.length === 0) return;
      played.current = true;
      return tweenEnter(items, 8, listStagger);
    },
    { scope: ref },
  );
  return (
    <ul ref={ref} className={className}>
      {children}
    </ul>
  );
}
