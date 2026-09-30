import type {
  Device,
  SecretKeyItem,
  CreatedSecretKeyResponse,
  SendMessagePayload,
  SendMessageResponse,
  SystemSettings,
  CurrentKeyInfo,
  PingResponse,
} from "./types";
import { request } from "./client";
import { API_ENDPOINTS, apiEndpoint } from "./endpoints";

export const serviceApi = {
  getPing: () => request<PingResponse>(API_ENDPOINTS.ping),
  login: (token: string) =>
    request<CurrentKeyInfo>(API_ENDPOINTS.login, {
      method: "POST",
      body: JSON.stringify({ token }),
    }),
  logout: () => request<{ logout: boolean }>(API_ENDPOINTS.logout, { method: "POST" }),
  getMe: () => request<CurrentKeyInfo>(API_ENDPOINTS.me),
  getDevices: () => request<Device[]>(API_ENDPOINTS.devices),
  deleteDevice: (id: string) =>
    request<{ id: string; deleted: number }>(apiEndpoint.device(id), {
      method: "DELETE",
    }),
  getKeys: () => request<SecretKeyItem[]>(API_ENDPOINTS.keys),
  getTokens: () => request<SecretKeyItem[]>(API_ENDPOINTS.keys),
  getSettings: () => request<SystemSettings>(API_ENDPOINTS.settings),
  updateSettings: (settings: Partial<SystemSettings>) =>
    request<SystemSettings>(API_ENDPOINTS.settings, {
      method: "PUT",
      body: JSON.stringify(settings),
    }),
  createKey: (name: string, role = "user") =>
    request<CreatedSecretKeyResponse>(API_ENDPOINTS.keys, {
      method: "POST",
      body: JSON.stringify({ name, role }),
    }),
  createToken: (name: string) =>
    request<CreatedSecretKeyResponse>(API_ENDPOINTS.keys, {
      method: "POST",
      body: JSON.stringify({ name, role: "user" }),
    }),
  updateKey: (
    id: string,
    updates: { name?: string; enabled?: number; role?: string },
  ) =>
    request<SecretKeyItem>(apiEndpoint.key(id), {
      method: "PUT",
      body: JSON.stringify(updates),
    }),
  updateToken: (
    id: string,
    updates: { name?: string; enabled?: boolean | number },
  ) =>
    request<SecretKeyItem>(apiEndpoint.key(id), {
      method: "PUT",
      body: JSON.stringify({
        ...updates,
        enabled:
          typeof updates.enabled === "boolean"
            ? updates.enabled
              ? 1
              : 0
            : updates.enabled,
      }),
    }),
  deleteKey: (id: string) =>
    request<{ id: string; deleted: number }>(apiEndpoint.key(id), {
      method: "DELETE",
    }),
  deleteToken: (id: string) =>
    request<{ id: string; deleted: number }>(apiEndpoint.key(id), {
      method: "DELETE",
    }),
  sendMessage: (payload: SendMessagePayload) =>
    request<SendMessageResponse>(API_ENDPOINTS.messages, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
