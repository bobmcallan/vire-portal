import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';

/**
 * Tests for src/pages/dashboard.tsx
 *
 * The dashboard displays:
 * - Total requests count for the period
 * - Quota bar showing usage percentage
 * - Daily trend chart (CSS bars)
 * - Top endpoints table
 * - Instance status (text-based: [RUNNING], [STOPPED], etc.)
 * - Plan badge (FREE / PRO)
 */

const mockUsage = {
  period: '2026-02',
  total_requests: 3421,
  quota_limit: 10000,
  quota_remaining: 6579,
  status: 'active',
  daily_counts: [
    { date: '2026-02-01', count: 120 },
    { date: '2026-02-02', count: 185 },
    { date: '2026-02-03', count: 210 },
  ],
  top_endpoints: [
    { endpoint: 'portfolio_compliance', count: 842 },
    { endpoint: 'get_summary', count: 521 },
  ],
};

const mockUser = {
  user_id: 'u1',
  email: 'test@test.com',
  display_name: 'Test User',
  avatar_url: '',
  auth_provider: 'google',
  created_at: '2026-01-01T00:00:00Z',
  status: 'active',
  keys_configured: true,
  plan: 'pro',
  proxy_status: 'running',
};

describe('Dashboard page', () => {
  it('displays total requests count', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { getByText } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    expect(getByText(/3,?421/)).toBeTruthy();
  });

  it('displays quota information', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { container } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    // Should show quota limit or remaining
    expect(container.textContent).toMatch(/10,?000|6,?579/);
  });

  it('renders a quota bar element', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { container } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    // Should have a visual bar element (div with width style or similar)
    const bars = container.querySelectorAll('[role="progressbar"], [data-testid="quota-bar"]');
    expect(bars.length).toBeGreaterThan(0);
  });

  it('displays top endpoints', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { getByText } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    expect(getByText(/portfolio_compliance/)).toBeTruthy();
    expect(getByText(/get_summary/)).toBeTruthy();
  });

  it('displays instance status as text label', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { container } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    // Status should be text-based, not color-based
    expect(container.textContent).toMatch(/running/i);
  });

  it('displays plan badge', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { container } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    expect(container.textContent).toMatch(/pro/i);
  });

  it('shows stopped status when proxy is stopped', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const stoppedUser = { ...mockUser, proxy_status: 'stopped' };
    const { container } = render(
      <Dashboard usage={mockUsage} user={stoppedUser} />,
    );
    expect(container.textContent).toMatch(/stopped/i);
  });

  it('renders daily trend bars', async () => {
    const { Dashboard } = await import('../pages/dashboard');
    const { container } = render(
      <Dashboard usage={mockUsage} user={mockUser} />,
    );
    // Should render some bar elements for daily counts
    const bars = container.querySelectorAll('[data-testid="daily-bar"]');
    expect(bars.length).toBe(3);
  });
});
