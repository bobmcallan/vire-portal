export interface ApiClient {
  getProfile(): Promise<Record<string, unknown>>;
  updateProfile(data: Record<string, unknown>): Promise<Record<string, unknown>>;
  deleteProfile(): Promise<Record<string, unknown>>;
  getKeys(): Promise<Record<string, unknown>>;
  updateKeys(keys: Record<string, string>): Promise<Record<string, unknown>>;
  deleteKey(keyId: string): Promise<Record<string, unknown>>;
  provision(): Promise<Record<string, unknown>>;
  getMcpConfig(): Promise<Record<string, unknown>>;
  getProxyStatus(): Promise<Record<string, unknown>>;
  getUsage(): Promise<Record<string, unknown>>;
  createCheckout(): Promise<Record<string, unknown>>;
  createBillingPortal(): Promise<Record<string, unknown>>;
}

export function createApiClient(
  apiUrl: string,
  getJwt: () => string | null,
  onUnauthorized: () => void,
  onRefreshed?: (token: string) => void,
): ApiClient {
  let refreshPromise: Promise<string> | null = null;

  async function doRefresh(): Promise<string> {
    const res = await fetch(`${apiUrl}/api/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error('Refresh failed');
    }
    const data = await res.json();
    return data.token;
  }

  async function request(
    path: string,
    options: RequestInit = {},
    isRetry = false,
  ): Promise<Record<string, unknown>> {
    const token = getJwt();
    const res = await fetch(`${apiUrl}${path}`, {
      ...options,
      credentials: 'include',
      headers: {
        ...((options.headers as Record<string, string>) ?? {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      },
    });

    if (res.status === 401 && !isRetry) {
      try {
        if (!refreshPromise) {
          refreshPromise = doRefresh();
        }
        const newToken = await refreshPromise;
        refreshPromise = null;
        onRefreshed?.(newToken);
        return request(path, options, true);
      } catch {
        refreshPromise = null;
        onUnauthorized();
        throw new Error('Authentication failed');
      }
    }

    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.error?.message ?? `Request failed: ${res.status}`);
    }

    return res.json();
  }

  return {
    getProfile: () => request('/api/profile', { method: 'GET' }),
    updateProfile: (data) =>
      request('/api/profile', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    deleteProfile: () => request('/api/profile', { method: 'DELETE' }),
    getKeys: () => request('/api/profile/keys', { method: 'GET' }),
    updateKeys: (keys) =>
      request('/api/profile/keys', {
        method: 'PUT',
        body: JSON.stringify(keys),
      }),
    deleteKey: (keyId) =>
      request(`/api/profile/keys/${keyId}`, { method: 'DELETE' }),
    provision: () =>
      request('/api/profile/provision', { method: 'POST' }),
    getMcpConfig: () => request('/api/profile/mcp', { method: 'GET' }),
    getProxyStatus: () => request('/api/profile/status', { method: 'GET' }),
    getUsage: () => request('/api/usage', { method: 'GET' }),
    createCheckout: () =>
      request('/api/billing/checkout', { method: 'POST' }),
    createBillingPortal: () =>
      request('/api/billing/portal', { method: 'POST' }),
  };
}
