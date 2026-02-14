import { describe, it, expect, vi, beforeEach } from 'vitest';

/**
 * Tests for src/auth.ts - Authentication module
 *
 * Exports:
 * - handleCallback(code, state, provider) - POST to gateway /api/auth/callback
 * - refreshToken(apiUrl) - POST to gateway /api/auth/refresh
 * - logout(apiUrl) - POST to gateway /api/auth/logout
 * - isAuthenticated() - check JWT exists and not expired
 * - getJwt() - return current JWT
 * - setJwt(token) - store JWT in memory
 * - decodeJwtPayload(token) - decode base64 JWT payload
 * - storeProvider(provider) - save provider to sessionStorage
 * - getStoredProvider() - read and clear provider from sessionStorage
 */

// Helper: create a fake JWT with given payload
function createFakeJwt(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify(payload));
  return `${header}.${body}.fake-signature`;
}

describe('auth', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
    sessionStorage.clear();
  });

  describe('storeProvider / getStoredProvider', () => {
    it('stores provider in sessionStorage', async () => {
      const { storeProvider } = await import('../auth');
      storeProvider('google');
      expect(sessionStorage.getItem('oauth_provider')).toBe('google');
    });

    it('retrieves and clears provider from sessionStorage', async () => {
      const { storeProvider, getStoredProvider } = await import('../auth');
      storeProvider('github');
      const provider = getStoredProvider();
      expect(provider).toBe('github');
      expect(sessionStorage.getItem('oauth_provider')).toBeNull();
    });

    it('returns "unknown" when no provider stored', async () => {
      const { getStoredProvider } = await import('../auth');
      const provider = getStoredProvider();
      expect(provider).toBe('unknown');
    });
  });

  describe('handleCallback', () => {
    it('sends code, state, and provider to gateway callback endpoint', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            token: 'jwt-token',
            user: {
              user_id: 'u1',
              email: 'test@test.com',
              display_name: 'Test',
              avatar_url: '',
              auth_provider: 'google',
              created_at: '2026-01-01T00:00:00Z',
              status: 'active',
              keys_configured: false,
              plan: 'free',
            },
          }),
      });
      vi.stubGlobal('fetch', mockFetch);

      const { handleCallback } = await import('../auth');
      const result = await handleCallback(
        'https://api.test',
        'auth-code',
        'state-token',
        'google',
      );

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/auth/callback',
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({
            provider: 'google',
            code: 'auth-code',
            state: 'state-token',
          }),
        }),
      );

      expect(result.token).toBe('jwt-token');
      expect(result.user.email).toBe('test@test.com');
    });

    it('throws on non-ok response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            error: { code: 'invalid_code', message: 'Auth code expired' },
          }),
      });
      vi.stubGlobal('fetch', mockFetch);

      const { handleCallback } = await import('../auth');
      await expect(
        handleCallback('https://api.test', 'bad-code', 'state', 'google'),
      ).rejects.toThrow();
    });
  });

  describe('refreshToken', () => {
    it('calls POST /api/auth/refresh with credentials include', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ token: 'new-jwt' }),
      });
      vi.stubGlobal('fetch', mockFetch);

      const { refreshToken } = await import('../auth');
      const result = await refreshToken('https://api.test');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/auth/refresh',
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
        }),
      );

      expect(result.token).toBe('new-jwt');
    });

    it('throws on 401 (refresh token expired)', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            error: { code: 'expired', message: 'Refresh token expired' },
          }),
      });
      vi.stubGlobal('fetch', mockFetch);

      const { refreshToken } = await import('../auth');
      await expect(refreshToken('https://api.test')).rejects.toThrow();
    });
  });

  describe('logout', () => {
    it('calls POST /api/auth/logout with credentials include', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ status: 'ok' }),
      });
      vi.stubGlobal('fetch', mockFetch);

      const { logout } = await import('../auth');
      await logout('https://api.test');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/auth/logout',
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
        }),
      );
    });
  });

  describe('JWT management', () => {
    it('getJwt returns null when no token set', async () => {
      const { getJwt } = await import('../auth');
      expect(getJwt()).toBeNull();
    });

    it('setJwt stores token and getJwt retrieves it', async () => {
      const { setJwt, getJwt } = await import('../auth');
      setJwt('my-token');
      expect(getJwt()).toBe('my-token');
    });

    it('setJwt with null clears the token', async () => {
      const { setJwt, getJwt } = await import('../auth');
      setJwt('my-token');
      setJwt(null);
      expect(getJwt()).toBeNull();
    });
  });

  describe('decodeJwtPayload', () => {
    it('decodes a base64url JWT payload', async () => {
      const { decodeJwtPayload } = await import('../auth');
      const token = createFakeJwt({ sub: 'user-1', exp: 9999999999 });
      const payload = decodeJwtPayload(token);
      expect(payload.sub).toBe('user-1');
      expect(payload.exp).toBe(9999999999);
    });
  });

  describe('isAuthenticated', () => {
    it('returns false when no token set', async () => {
      const { isAuthenticated } = await import('../auth');
      expect(isAuthenticated()).toBe(false);
    });

    it('returns true when token is set and not expired', async () => {
      const { setJwt, isAuthenticated } = await import('../auth');
      const futureExp = Math.floor(Date.now() / 1000) + 3600;
      const token = createFakeJwt({ exp: futureExp });
      setJwt(token);
      expect(isAuthenticated()).toBe(true);
    });

    it('returns false when token is expired', async () => {
      const { setJwt, isAuthenticated } = await import('../auth');
      const pastExp = Math.floor(Date.now() / 1000) - 100;
      const token = createFakeJwt({ exp: pastExp });
      setJwt(token);
      expect(isAuthenticated()).toBe(false);
    });
  });
});
