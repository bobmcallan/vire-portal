import type { AuthCallbackResponse, RefreshResponse } from './types';

let jwt: string | null = null;

export function getJwt(): string | null {
  return jwt;
}

export function setJwt(token: string | null): void {
  jwt = token;
}

export function decodeJwtPayload(token: string): Record<string, unknown> {
  const parts = token.split('.');
  const payload = parts[1];
  if (!payload) throw new Error('Invalid JWT format');
  // Convert base64url to base64: replace - with +, _ with /
  const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
  // Pad to multiple of 4
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  const decoded = atob(padded);
  return JSON.parse(decoded);
}

export function isAuthenticated(): boolean {
  if (!jwt) return false;
  try {
    const payload = decodeJwtPayload(jwt);
    const exp = payload.exp as number | undefined;
    if (!exp) return false;
    return exp > Math.floor(Date.now() / 1000);
  } catch {
    return false;
  }
}

export function storeProvider(provider: string): void {
  sessionStorage.setItem('oauth_provider', provider);
}

export function getStoredProvider(): string {
  const provider = sessionStorage.getItem('oauth_provider');
  sessionStorage.removeItem('oauth_provider');
  return provider ?? 'unknown';
}

export async function handleCallback(
  apiUrl: string,
  code: string,
  state: string,
  provider: string,
): Promise<AuthCallbackResponse> {
  const res = await fetch(`${apiUrl}/api/auth/callback`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider, code, state }),
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message ?? 'Authentication failed');
  }

  return res.json();
}

export async function refreshToken(apiUrl: string): Promise<RefreshResponse> {
  const res = await fetch(`${apiUrl}/api/auth/refresh`, {
    method: 'POST',
    credentials: 'include',
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message ?? 'Token refresh failed');
  }

  return res.json();
}

export async function logout(apiUrl: string): Promise<void> {
  await fetch(`${apiUrl}/api/auth/logout`, {
    method: 'POST',
    credentials: 'include',
  });
}
