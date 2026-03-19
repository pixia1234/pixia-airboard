export interface ApiEnvelope<T> {
  status?: string;
  message?: string;
  data: T;
  error?: unknown;
  total?: number;
}

export interface UserInfo {
  email: string;
  transfer_enable: number;
  last_login_at: number;
  created_at: number;
  banned: boolean;
  remind_expire: boolean;
  remind_traffic: boolean;
  expired_at: number;
  balance: number;
  commission_balance: number;
  plan_id: number;
  uuid: string;
  avatar_url: string;
}

export interface Plan {
  id: number;
  name: string;
  price: number;
  transfer_enable: number;
  speed_limit: number;
  show: boolean;
  sort: number;
  group_id: number;
  content: string;
  created_at: number;
  updated_at: number;
  count?: number;
}

export interface SubscriptionLink {
  id: number;
  user_id?: number;
  name: string;
  suffix: string;
  is_primary: boolean;
  enabled: boolean;
  last_used_at: number;
  urls: Record<string, string>;
}

export interface SubscribePayload {
  plan_id: number;
  token: string;
  expired_at: number;
  u: number;
  d: number;
  transfer_enable: number;
  email: string;
  uuid: string;
  plan?: Plan | null;
  subscribe_url: string;
  subscribe_urls: SubscriptionLink[];
  reset_day: number;
}

export interface ServerNode {
  id: number;
  type: string;
  version: number;
  name: string;
  rate: number;
  tags: string[];
  is_online: boolean;
  cache_key?: string;
  last_check_at: number;
  host?: string;
  port?: number;
  network?: string;
  show?: boolean;
  plan_ids?: number[];
}

export interface Notice {
  id: number;
  title: string;
  content: string;
  show: boolean;
  created_at: number;
  updated_at: number;
}

export interface UserRow {
  id: number;
  email: string;
  uuid: string;
  token: string;
  is_admin: boolean;
  is_staff: boolean;
  banned: boolean;
  transfer_enable: number;
  u: number;
  d: number;
  total_used: number;
  expired_at: number;
  plan_id: number;
  plan_name: string;
  subscribe_url: string;
  subscribe_suffix: string;
  created_at: number;
  last_login_at: number;
  balance: number;
  commission_balance: number;
}

export interface SessionCheck {
  is_login: boolean;
  is_admin?: boolean;
}

export interface DashboardStats {
  users: number;
  plans: number;
  servers: number;
  notices: number;
}

export interface UserStatsResponse {
  data: number[];
}

export type SettingsRecord = Record<string, string>;
