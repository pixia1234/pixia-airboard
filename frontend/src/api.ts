import { message } from "antd";

import type { ApiEnvelope } from "./types";

export const AUTH_STORAGE_KEY = "airboard.auth_data";

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  authData?: string;
  silent?: boolean;
}

export function readStoredAuth() {
  return localStorage.getItem(AUTH_STORAGE_KEY) || "";
}

export function storeAuth(value: string) {
  if (!value) {
    localStorage.removeItem(AUTH_STORAGE_KEY);
    return;
  }
  localStorage.setItem(AUTH_STORAGE_KEY, value);
}

export async function requestJSON<T>(path: string, options: RequestOptions = {}) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (options.authData) {
    headers.Authorization = options.authData;
  }

  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  let payload: unknown = null;
  try {
    payload = await response.json();
  } catch (_) {
    payload = null;
  }

  if (!response.ok) {
    const messageText =
      (payload as { message?: string } | null)?.message || `请求失败 (${response.status})`;
    if (!options.silent) {
      message.error(messageText);
    }
    throw new Error(messageText);
  }

  return payload as T;
}

export function unwrapData<T>(payload: ApiEnvelope<T> | { data: T }) {
  return payload.data;
}
