import { describe, it, expect, vi, beforeEach } from 'vitest';

/**
 * Tests for src/api.ts - Gateway API client
 *
 * Exports:
 * - createApiClient(apiUrl, getJwt, onUnauthorized) - factory returning typed API client
 *
 * The client:
 * - Attaches Authorization: Bearer <jwt> header
 * - Sets credentials: 'include' on all requests
 * - On 401, attempts token refresh then retries the original request
 * - On refresh failure, calls onUnauthorized callback
 * - Provides typed methods: getProfile, updateProfile, getUsage, etc.
 */

describe('api client', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  describe('request headers', () => {
    it('attaches Bearer token to requests', async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user_id: 'u1' }),
      });

      const { createApiClient } = await import('../api');
      const client = createApiClient(
        'https://api.test',
        () => 'my-jwt-token',
        vi.fn(),
      );

      await client.getProfile();

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer my-jwt-token',
          }),
        }),
      );
    });

    it('includes credentials: include on all requests', async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });

      const { createApiClient } = await import('../api');
      const client = createApiClient(
        'https://api.test',
        () => 'token',
        vi.fn(),
      );

      await client.getProfile();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          credentials: 'include',
        }),
      );
    });
  });

  describe('401 handling with refresh retry', () => {
    it('retries request after successful token refresh', async () => {
      let callCount = 0;
      mockFetch.mockImplementation((url: string) => {
        if (url.endsWith('/api/profile')) {
          callCount++;
          if (callCount === 1) {
            return Promise.resolve({
              ok: false,
              status: 401,
              json: () =>
                Promise.resolve({
                  error: { code: 'expired', message: 'Token expired' },
                }),
            });
          }
          return Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve({ user_id: 'u1', email: 'test@test.com' }),
          });
        }
        if (url.endsWith('/api/auth/refresh')) {
          return Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve({ token: 'refreshed-jwt' }),
          });
        }
        return Promise.resolve({ ok: false, status: 404 });
      });

      const onRefreshed = vi.fn();
      const { createApiClient } = await import('../api');
      const client = createApiClient(
        'https://api.test',
        () => 'old-token',
        vi.fn(),
        onRefreshed,
      );

      const result = await client.getProfile();
      expect(result.user_id).toBe('u1');
      expect(onRefreshed).toHaveBeenCalledWith('refreshed-jwt');
    });

    it('calls onUnauthorized when refresh fails', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.endsWith('/api/profile')) {
          return Promise.resolve({
            ok: false,
            status: 401,
            json: () =>
              Promise.resolve({
                error: { code: 'expired', message: 'Token expired' },
              }),
          });
        }
        if (url.endsWith('/api/auth/refresh')) {
          return Promise.resolve({
            ok: false,
            status: 401,
            json: () =>
              Promise.resolve({
                error: { code: 'expired', message: 'Refresh token expired' },
              }),
          });
        }
        return Promise.resolve({ ok: false, status: 404 });
      });

      const onUnauthorized = vi.fn();
      const { createApiClient } = await import('../api');
      const client = createApiClient(
        'https://api.test',
        () => 'old-token',
        onUnauthorized,
      );

      await expect(client.getProfile()).rejects.toThrow();
      expect(onUnauthorized).toHaveBeenCalled();
    });

    it('prevents concurrent refresh attempts (mutex)', async () => {
      let refreshCallCount = 0;
      mockFetch.mockImplementation((url: string) => {
        if (url.endsWith('/api/auth/refresh')) {
          refreshCallCount++;
          return Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve({ token: 'refreshed-jwt' }),
          });
        }
        return Promise.resolve({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({
              error: { code: 'expired', message: 'Expired' },
            }),
        });
      });

      const { createApiClient } = await import('../api');
      const client = createApiClient(
        'https://api.test',
        () => 'old-token',
        vi.fn(),
        vi.fn(),
      );

      // Fire two requests simultaneously - both get 401
      // Only one refresh call should happen
      await Promise.allSettled([
        client.getProfile(),
        client.getUsage(),
      ]);

      // At most 1 refresh call (mutex prevents duplicates)
      expect(refreshCallCount).toBe(1);
    });
  });

  describe('API methods', () => {
    beforeEach(() => {
      mockFetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });
    });

    it('getProfile calls GET /api/profile', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.getProfile();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile',
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('updateProfile calls PUT /api/profile with body', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.updateProfile({ default_portfolio: 'SMSF', exchange: 'AU' });
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ default_portfolio: 'SMSF', exchange: 'AU' }),
        }),
      );
    });

    it('deleteProfile calls DELETE /api/profile', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.deleteProfile();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('getKeys calls GET /api/profile/keys', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.getKeys();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/keys',
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('updateKeys calls PUT /api/profile/keys with body', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.updateKeys({ eodhd_key: 'abc123' });
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/keys',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ eodhd_key: 'abc123' }),
        }),
      );
    });

    it('deleteKey calls DELETE /api/profile/keys/:id', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.deleteKey('navexa_key');
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/keys/navexa_key',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('provision calls POST /api/profile/provision', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.provision();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/provision',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('getMcpConfig calls GET /api/profile/mcp', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.getMcpConfig();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/mcp',
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('getProxyStatus calls GET /api/profile/status', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.getProxyStatus();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/profile/status',
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('getUsage calls GET /api/usage', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.getUsage();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/usage',
        expect.objectContaining({ method: 'GET' }),
      );
    });

    it('createCheckout calls POST /api/billing/checkout', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.createCheckout();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/billing/checkout',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('createBillingPortal calls POST /api/billing/portal', async () => {
      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await client.createBillingPortal();
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/billing/portal',
        expect.objectContaining({ method: 'POST' }),
      );
    });
  });

  describe('error handling', () => {
    it('throws with error details on non-401 error response', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 422,
        json: () =>
          Promise.resolve({
            error: {
              code: 'invalid_key',
              message: 'EODHD API returned 401',
            },
          }),
      });

      const { createApiClient } = await import('../api');
      const client = createApiClient('https://api.test', () => 'tok', vi.fn());
      await expect(client.getProfile()).rejects.toThrow();
    });
  });
});
