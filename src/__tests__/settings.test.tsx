import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';

/**
 * Tests for src/pages/settings.tsx
 *
 * The settings page displays:
 * - Profile info (read-only: email, name, avatar)
 * - API keys section with 3 key-input components (EODHD, Navexa, Gemini)
 * - Preferences (default portfolio, exchange)
 * - Delete account button with confirmation
 */

const mockUser = {
  user_id: 'u1',
  email: 'alice@example.com',
  display_name: 'Alice',
  avatar_url: 'https://example.com/avatar.jpg',
  auth_provider: 'google',
  created_at: '2026-01-01T00:00:00Z',
  status: 'active',
  keys_configured: true,
  default_portfolio: 'SMSF',
  portfolios: ['SMSF', 'Personal'],
  exchange: 'AU',
  plan: 'pro',
};

const mockKeys = {
  eodhd_key: { status: 'valid' as const, last4: 'c123', validated_at: '2026-02-09T10:05:00Z' },
  navexa_key: { status: 'valid' as const, last4: 'f456', validated_at: '2026-02-09T10:05:00Z', portfolios_found: 2 },
  gemini_key: undefined,
};

describe('Settings page', () => {
  it('displays user email', async () => {
    const { Settings } = await import('../pages/settings');
    const { getByText } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(getByText('alice@example.com')).toBeTruthy();
  });

  it('displays user display name', async () => {
    const { Settings } = await import('../pages/settings');
    const { getByText } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(getByText('Alice')).toBeTruthy();
  });

  it('renders EODHD key section', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(container.textContent).toMatch(/eodhd/i);
  });

  it('renders Navexa key section', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(container.textContent).toMatch(/navexa/i);
  });

  it('renders Gemini key section', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(container.textContent).toMatch(/gemini/i);
  });

  it('shows masked key value for configured keys', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    // Should show last 4 characters of configured keys
    expect(container.textContent).toMatch(/c123/);
    expect(container.textContent).toMatch(/f456/);
  });

  it('shows "NOT SET" for unconfigured keys', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    // Gemini key is undefined, should show not configured state
    expect(container.textContent).toMatch(/not set/i);
  });

  it('renders portfolio preference selector', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(container.textContent).toMatch(/portfolio/i);
  });

  it('renders exchange preference selector', async () => {
    const { Settings } = await import('../pages/settings');
    const { container } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(container.textContent).toMatch(/exchange/i);
  });

  it('renders delete account button', async () => {
    const { Settings } = await import('../pages/settings');
    const { getByText } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={vi.fn()} />,
    );
    expect(getByText(/delete account/i)).toBeTruthy();
  });

  it('shows confirmation before deleting account', async () => {
    const onDeleteAccount = vi.fn();
    const { Settings } = await import('../pages/settings');
    const { getByText } = render(
      <Settings user={mockUser} keys={mockKeys} onUpdateKeys={vi.fn()} onUpdateProfile={vi.fn()} onDeleteAccount={onDeleteAccount} />,
    );

    fireEvent.click(getByText(/delete account/i));
    // Should show a confirmation prompt before actually deleting
    expect(getByText(/are you sure/i)).toBeTruthy();
    // Should not have called delete yet
    expect(onDeleteAccount).not.toHaveBeenCalled();
  });
});
