import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';

/**
 * Tests for src/components/layout.tsx
 *
 * The layout component renders:
 * - Top nav bar with "VIRE" title
 * - Navigation links (Dashboard, Settings, Connect, Billing)
 * - Active link styling (underlined or inverted)
 * - Mobile hamburger menu toggle
 * - Footer with version info
 * - Content area (children)
 */

describe('Layout component', () => {
  it('renders VIRE title in nav', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText('VIRE')).toBeTruthy();
  });

  it('renders Dashboard nav link', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText(/dashboard/i)).toBeTruthy();
  });

  it('renders Settings nav link', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText(/settings/i)).toBeTruthy();
  });

  it('renders Connect nav link', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText(/connect/i)).toBeTruthy();
  });

  it('renders Billing nav link', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText(/billing/i)).toBeTruthy();
  });

  it('renders children in content area', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>test-content</div>
      </Layout>,
    );
    expect(getByText('test-content')).toBeTruthy();
  });

  it('renders logout button', async () => {
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    expect(getByText(/logout|sign out/i)).toBeTruthy();
  });

  it('calls onLogout when logout is clicked', async () => {
    const onLogout = vi.fn();
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={onLogout} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    fireEvent.click(getByText(/logout|sign out/i));
    expect(onLogout).toHaveBeenCalled();
  });

  it('calls onNavigate when a nav link is clicked', async () => {
    const onNavigate = vi.fn();
    const { Layout } = await import('../components/layout');
    const { getByText } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={onNavigate}>
        <div>content</div>
      </Layout>,
    );
    fireEvent.click(getByText(/settings/i));
    expect(onNavigate).toHaveBeenCalledWith('/settings');
  });

  it('has a mobile menu toggle button', async () => {
    const { Layout } = await import('../components/layout');
    const { container } = render(
      <Layout currentPath="/dashboard" onLogout={vi.fn()} onNavigate={vi.fn()}>
        <div>content</div>
      </Layout>,
    );
    const menuButton = container.querySelector('[aria-label="Toggle menu"], [data-testid="menu-toggle"]');
    expect(menuButton).toBeTruthy();
  });
});
