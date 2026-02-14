import { describe, it, expect, vi } from 'vitest';

/**
 * Tests for src/state.ts - App state management
 *
 * The state module provides a simple pub/sub store holding:
 * - user: User profile or null
 * - jwt: JWT token string or null
 * - config: Config (apiUrl, domain) or null
 *
 * Exports: getState, setState, subscribe, resetState, useAppState
 */

describe('state', () => {
  describe('getState / setState', () => {
    it('returns initial state with all null values', async () => {
      const { getState } = await import('../state');
      const state = getState();
      expect(state).toEqual({
        user: null,
        jwt: null,
        config: null,
      });
    });

    it('updates state with partial values', async () => {
      const { getState, setState } = await import('../state');
      setState({ jwt: 'test-token' });
      expect(getState().jwt).toBe('test-token');
    });

    it('merges partial state without overwriting other fields', async () => {
      const { getState, setState } = await import('../state');
      setState({ jwt: 'token-1' });
      setState({ config: { apiUrl: 'https://api.test', domain: 'test.com' } });
      const state = getState();
      expect(state.jwt).toBe('token-1');
      expect(state.config?.apiUrl).toBe('https://api.test');
    });

    it('sets user profile', async () => {
      const { getState, setState } = await import('../state');
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
      setState({ user });
      expect(getState().user).toEqual(user);
    });
  });

  describe('subscribe', () => {
    it('notifies listeners on state change', async () => {
      const { setState, subscribe } = await import('../state');
      const listener = vi.fn();
      subscribe(listener);
      setState({ jwt: 'new-token' });
      expect(listener).toHaveBeenCalled();
    });

    it('returns unsubscribe function', async () => {
      const { setState, subscribe } = await import('../state');
      const listener = vi.fn();
      const unsubscribe = subscribe(listener);
      unsubscribe();
      setState({ jwt: 'another-token' });
      expect(listener).not.toHaveBeenCalled();
    });

    it('supports multiple listeners', async () => {
      const { setState, subscribe } = await import('../state');
      const listener1 = vi.fn();
      const listener2 = vi.fn();
      subscribe(listener1);
      subscribe(listener2);
      setState({ jwt: 'multi-token' });
      expect(listener1).toHaveBeenCalled();
      expect(listener2).toHaveBeenCalled();
    });
  });

  describe('resetState', () => {
    it('clears all state back to null values', async () => {
      const { getState, setState, resetState } = await import('../state');
      setState({
        jwt: 'token',
        config: { apiUrl: 'https://api.test', domain: 'test.com' },
      });
      resetState();
      const state = getState();
      expect(state.user).toBeNull();
      expect(state.jwt).toBeNull();
      expect(state.config).toBeNull();
    });
  });
});
