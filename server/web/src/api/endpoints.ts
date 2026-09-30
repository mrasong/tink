export const API_ENDPOINTS = {
  ping: "/api/v1/ping",
  login: "/api/v1/login",
  logout: "/api/v1/logout",
  me: "/api/v1/me",
  devices: "/api/v1/devices",
  keys: "/api/v1/keys",
  settings: "/api/v1/settings",
  messages: "/api/v1/messages",
} as const;

export const apiEndpoint = {
  device: (id: string) => `${API_ENDPOINTS.devices}/${id}`,
  key: (id: string) => `${API_ENDPOINTS.keys}/${id}`,
} as const;
