export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
}

export interface Device {
  id: string;
  key_id?: string;
  token_id?: string; // 兼容字段
  name: string;
  status: 0 | 1;
  created_at: number;
  last_connected_at: number;
  last_disconnected_at: number;
}

export interface CurrentKeyInfo {
  id: string;
  name: string;
  role: "admin" | "user";
  is_master?: boolean; // 兼容旧字段
  enabled: number; // 1: 启用, 0: 禁用
  created_at: number;
}

// 别名兼容
export type CurrentTokenInfo = CurrentKeyInfo;

export interface SecretKeyItem {
  id: string;
  name: string;
  role: "admin" | "user";
  is_master?: boolean; // 兼容旧字段
  enabled: number; // 1: 启用, 0: 禁用
  created_at: number;
  last_used?: number;
}

// 别名兼容
export type TokenItem = SecretKeyItem;

export interface CreatedSecretKeyResponse {
  token: string;
  secret_key?: string;
  id: string;
  name: string;
  role: "admin" | "user";
  is_master?: boolean;
  enabled: number;
  created_at: number;
}

// 别名兼容
export type CreatedTokenResponse = CreatedSecretKeyResponse;

export interface ServerHealth {
  status: string;
  message?: string;
  version?: string;
  st?: number;
}

export interface PingResponse {
  message: string;
  version: string;
  build?: string;
  st: number;
}

export interface SystemSettings {
  bark_relay_enabled: boolean;
  bark_server_url: string;
  bark_route_path: string;
}

export interface SendMessagePayload {
  title: string;
  body: string;
  url?: string;
  sound?: string;
  group?: string;
  devices?: string[];
  bark_devices?: string[];
  bark_params?: Record<string, any>;
}

export interface SendMessageResponse {
  id?: number;
  dispatched_tink: number;
  dispatched_bark: number;
  bark_errors?: string[];
  created_at?: number;
}
