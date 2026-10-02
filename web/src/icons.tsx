import type { ReactNode } from "react";

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.7,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

function g(nodes: ReactNode) {
  return nodes;
}

const glyphs: Record<string, ReactNode> = {
  home: g(
    <>
      <path d="M3.5 11 12 3.5 20.5 11" {...stroke} />
      <path d="M6 10.5V20h12v-9.5" {...stroke} />
      <path d="M10 20v-6h4v6" {...stroke} />
    </>,
  ),
  kitchen: g(
    <>
      <path d="M8 10h8l-.8 9.2H8.8Z" {...stroke} />
      <path d="M9 10V8.2c0-1.6 1.3-3 3-3s3 1.4 3 3V10" {...stroke} />
      <path d="M9.5 14h5" {...stroke} />
    </>,
  ),
  living: g(
    <>
      <path d="M4 14.5h16v5H4z" {...stroke} />
      <path d="M6 14.5V11h12v3.5" {...stroke} />
      <path d="M4 19.5v1.5M20 19.5v1.5" {...stroke} />
      <path d="M8 11V9h8v2" {...stroke} />
    </>,
  ),
  bedroom: g(
    <>
      <path d="M3.5 18V10.5h8V8h9v10" {...stroke} />
      <path d="M3.5 14.5h17" {...stroke} />
      <path d="M5 18v1.5M19 18v1.5" {...stroke} />
    </>,
  ),
  bathroom: g(
    <>
      <path d="M6 4.5v6.5h12" {...stroke} />
      <path d="M8 7.5h2M8 10h2" {...stroke} />
      <path d="M7 14.5c0 2.8 2.2 5 5 5s5-2.2 5-5" {...stroke} />
      <path d="M12 11.5v3" {...stroke} />
    </>,
  ),
  balcony: g(
    <>
      <path d="M4 20V10l8-5 8 5v10" {...stroke} />
      <path d="M4 14.5h16" {...stroke} />
      <path d="M8 14.5V20M12 14.5V20M16 14.5V20" {...stroke} />
    </>,
  ),
  garage: g(
    <>
      <path d="M3.5 11 12 4.5 20.5 11" {...stroke} />
      <path d="M5.5 10.2V20h13V10.2" {...stroke} />
      <path d="M8 13h8M8 16h8M8 19h8" {...stroke} />
    </>,
  ),
  cabinet: g(
    <>
      <path d="M5 4.5h14v16H5z" {...stroke} />
      <path d="M12 4.5v16" {...stroke} />
      <path d="M9.2 12.5h.1M14.8 12.5h.1" {...stroke} />
    </>,
  ),
  drawer: g(
    <>
      <path d="M5 4.5h14v16H5z" {...stroke} />
      <path d="M5 9.5h14M5 14.5h14" {...stroke} />
      <path d="M10.5 7h3M10.5 12h3M10.5 17h3" {...stroke} />
    </>,
  ),
  shelf: g(
    <>
      <path d="M4 6.5h16M4 12h16M4 17.5h16" {...stroke} />
      <path d="M5.5 6.5V17.5M18.5 6.5V17.5" {...stroke} />
    </>,
  ),
  fridge: g(
    <>
      <path d="M7 3.5h10v18H7z" {...stroke} />
      <path d="M7 10h10" {...stroke} />
      <path d="M9.2 6.5v1.5M9.2 13v3" {...stroke} />
    </>,
  ),
  washer: g(
    <>
      <path d="M5 4h14v16H5z" {...stroke} />
      <circle cx="12" cy="13" r="4.2" {...stroke} />
      <path d="M8 6.5h2M14.5 6.5h1.5" {...stroke} />
    </>,
  ),
  wardrobe: g(
    <>
      <path d="M5 3.5h14v18H5z" {...stroke} />
      <path d="M12 3.5v18" {...stroke} />
      <path d="M8 8.5h.1M16 8.5h.1" {...stroke} />
      <path d="M8.5 11.5v4M15.5 11.5v4" {...stroke} />
    </>,
  ),
  bookcase: g(
    <>
      <path d="M4.5 4h15v16h-15z" {...stroke} />
      <path d="M4.5 12h15" {...stroke} />
      <path d="M7 6.5v4M10.5 6.5v4M14 6.5v4" {...stroke} />
      <path d="M8 14v4M12 14v4M16 14v4" {...stroke} />
    </>,
  ),
  box: g(
    <>
      <path d="M4.5 8.5 12 4.5l7.5 4V18L12 21.5 4.5 18Z" {...stroke} />
      <path d="M4.5 8.5 12 12.5 19.5 8.5M12 12.5V21.5" {...stroke} />
    </>,
  ),
  crate: g(
    <>
      <path d="M5 6.5h14v12H5z" {...stroke} />
      <path d="M5 6.5 19 18.5M19 6.5 5 18.5" {...stroke} />
    </>,
  ),
  basket: g(
    <>
      <path d="M6 10h12l-1.2 8.5H7.2Z" {...stroke} />
      <path d="M8 10c0-2.4 1.8-4.2 4-4.2s4 1.8 4 4.2" {...stroke} />
    </>,
  ),
  toolbox: g(
    <>
      <path d="M4.5 10h15v9.5h-15z" {...stroke} />
      <path d="M9 10V7.5h6V10" {...stroke} />
      <path d="M4.5 14h15" {...stroke} />
    </>,
  ),
  bin: g(
    <>
      <path d="M7 8.5h10l-.8 11H7.8Z" {...stroke} />
      <path d="M6 8.5h12" {...stroke} />
      <path d="M10 6.5h4" {...stroke} />
      <path d="M10 12v5M14 12v5" {...stroke} />
    </>,
  ),
  safe: g(
    <>
      <path d="M5 5h14v14H5z" {...stroke} />
      <circle cx="12" cy="12" r="3.2" {...stroke} />
      <path d="M12 8.8v1.6M15.2 12h-1.6M12 15.2v-1.6M8.8 12h1.6" {...stroke} />
    </>,
  ),
};

export const ICON_GROUPS: { label: string; icons: { slug: string; label: string }[] }[] = [
  {
    label: "区域",
    icons: [
      { slug: "home", label: "房子" },
      { slug: "kitchen", label: "厨房" },
      { slug: "living", label: "客厅" },
      { slug: "bedroom", label: "卧室" },
      { slug: "bathroom", label: "卫生间" },
      { slug: "balcony", label: "阳台" },
      { slug: "garage", label: "储藏/车位" },
    ],
  },
  {
    label: "柜格",
    icons: [
      { slug: "cabinet", label: "柜子" },
      { slug: "drawer", label: "抽屉" },
      { slug: "shelf", label: "层板" },
      { slug: "fridge", label: "冰箱" },
      { slug: "washer", label: "洗衣机" },
      { slug: "wardrobe", label: "衣柜" },
      { slug: "bookcase", label: "书架" },
    ],
  },
  {
    label: "盒子",
    icons: [
      { slug: "box", label: "盒子" },
      { slug: "crate", label: "箱子" },
      { slug: "basket", label: "篮子" },
      { slug: "toolbox", label: "工具箱" },
      { slug: "bin", label: "桶" },
      { slug: "safe", label: "保险箱" },
    ],
  },
];

export function resolvedIcon(node: { icon?: string | null; type: string }): string {
  if (node.icon) return node.icon;
  if (node.type === "area") return "home";
  if (node.type === "fixed") return "cabinet";
  return "box";
}

export function defaultIconLabel(type: string): string {
  if (type === "area") return "房子";
  if (type === "fixed") return "柜子";
  if (type === "movable") return "盒子";
  return "按类型";
}

export function LocationIcon({ name, className }: { name: string; className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" width="1.5rem" height="1.5rem" aria-hidden="true">
      {glyphs[name] ?? glyphs.home}
    </svg>
  );
}
