import dayjs from "dayjs";

import type { SubscriptionLink } from "./types";

export function readBootstrap() {
  return (
    window.__AIRBOARD_BOOTSTRAP__ || {
      page: "user" as const,
      title: "Pixia Airboard",
      description: "Subscription management panel",
      adminPath: "admin",
      apiBase: "/api/v1",
      appUrl: window.location.origin,
    }
  );
}

export function formatBytes(value: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let current = Number(value || 0);
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${current.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

export function formatPrice(value: number) {
  return `¥ ${Number(value || 0).toFixed(2)}`;
}

export function formatDateTime(value: number) {
  if (!value) {
    return "长期有效";
  }
  return dayjs.unix(value).format("YYYY-MM-DD HH:mm:ss");
}

export function formatRelativeStamp(value: number) {
  if (!value) {
    return "未使用";
  }
  return formatDateTime(value);
}

export function trafficSummary(u: number, d: number, total: number) {
  const used = Number(u || 0) + Number(d || 0);
  return `${formatBytes(Math.max(total - used, 0))} / ${formatBytes(total)}`;
}

export function primarySubscription(links: SubscriptionLink[]) {
  return links.find((item) => item.is_primary) || links[0] || null;
}

export function initialsFromEmail(email: string) {
  const value = (email || "A").trim();
  return value.slice(0, 1).toUpperCase();
}
