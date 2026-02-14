import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/preact';

/**
 * Tests for src/pages/callback.tsx
 *
 * The callback page:
 * - Extracts code and state from URL query params
 * - Reads provider from sessionStorage (set by landing page)
 * - POSTs to gateway /api/auth/callback with { provider, code, state }
 * - On success: stores JWT in state, redirects to /dashboard
 * - On error: shows error message with retry link
 * - Shows loading state while processing
 */

describe('Callback page', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('shows loading state initially', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockReturnValue(new Promise(() => {})), // never resolves
    );
    sessionStorage.setItem('oauth_provider', 'google');

    const { Callback } = await import('../pages/callback');
    const { getByText } = render(
      <Callback code="auth-code" state="state-token" apiUrl="https://api.test" />,
    );
    expect(getByText(/authenticating/i)).toBeTruthy();
  });

  it('shows error when apiUrl is not provided', async () => {
    const { Callback } = await import('../pages/callback');
    const { container } = render(
      <Callback code="auth-code" state="state-token" />,
    );
    await waitFor(() => {
      expect(container.textContent).toMatch(/not configured/i);
    });
  });

  it('reads provider from sessionStorage and includes in POST', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          token: 'jwt',
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
    sessionStorage.setItem('oauth_provider', 'google');

    const { Callback } = await import('../pages/callback');
    render(
      <Callback
        code="auth-code"
        state="state-token"
        apiUrl="https://api.test"
        onSuccess={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.test/api/auth/callback',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            provider: 'google',
            code: 'auth-code',
            state: 'state-token',
          }),
        }),
      );
    });
  });

  it('falls back to "unknown" provider when sessionStorage is empty', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          token: 'jwt',
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

    const { Callback } = await import('../pages/callback');
    render(
      <Callback
        code="auth-code"
        state="state-token"
        apiUrl="https://api.test"
        onSuccess={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          body: JSON.stringify({
            provider: 'unknown',
            code: 'auth-code',
            state: 'state-token',
          }),
        }),
      );
    });
  });

  it('calls onSuccess with token and user on successful auth', async () => {
    const user = {
      user_id: 'u1',
      email: 'test@test.com',
      display_name: 'Test',
      avatar_url: '',
      auth_provider: 'google',
      created_at: '2026-01-01T00:00:00Z',
      status: 'active',
      keys_configured: false,
      plan: 'free',
    };
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ token: 'jwt-token', user }),
      }),
    );

    const onSuccess = vi.fn();
    const { Callback } = await import('../pages/callback');
    render(
      <Callback
        code="auth-code"
        state="state-token"
        apiUrl="https://api.test"
        onSuccess={onSuccess}
      />,
    );

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledWith('jwt-token', user);
    });
  });

  it('shows error message on failed auth', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            error: { code: 'invalid_code', message: 'Auth code expired' },
          }),
      }),
    );

    const { Callback } = await import('../pages/callback');
    const { container } = render(
      <Callback
        code="bad-code"
        state="state-token"
        apiUrl="https://api.test"
        onSuccess={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(container.textContent).toMatch(/failed|error|expired/i);
    });
  });

  it('clears provider from sessionStorage after reading', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            token: 'jwt',
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
      }),
    );
    sessionStorage.setItem('oauth_provider', 'google');

    const { Callback } = await import('../pages/callback');
    render(
      <Callback
        code="code"
        state="state"
        apiUrl="https://api.test"
        onSuccess={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(sessionStorage.getItem('oauth_provider')).toBeNull();
    });
  });
});
