import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';

/**
 * Tests for src/pages/billing.tsx
 *
 * The billing page displays:
 * - Current plan (Free / Pro)
 * - Upgrade button (for free users) -> Stripe checkout
 * - Manage subscription button (for pro users) -> Stripe billing portal
 * - Post-checkout success message when session_id is in URL
 */

describe('Billing page', () => {
  it('displays current plan for free user', async () => {
    const { Billing } = await import('../pages/billing');
    const { container } = render(
      <Billing plan="free" onCheckout={vi.fn()} onManage={vi.fn()} sessionId={null} />,
    );
    expect(container.textContent).toMatch(/free/i);
  });

  it('displays current plan for pro user', async () => {
    const { Billing } = await import('../pages/billing');
    const { container } = render(
      <Billing plan="pro" onCheckout={vi.fn()} onManage={vi.fn()} sessionId={null} />,
    );
    expect(container.textContent).toMatch(/pro/i);
  });

  it('shows upgrade button for free plan', async () => {
    const { Billing } = await import('../pages/billing');
    const { getByText } = render(
      <Billing plan="free" onCheckout={vi.fn()} onManage={vi.fn()} sessionId={null} />,
    );
    expect(getByText(/upgrade/i)).toBeTruthy();
  });

  it('shows manage subscription button for pro plan', async () => {
    const { Billing } = await import('../pages/billing');
    const { getByText } = render(
      <Billing plan="pro" onCheckout={vi.fn()} onManage={vi.fn()} sessionId={null} />,
    );
    expect(getByText(/manage/i)).toBeTruthy();
  });

  it('calls onCheckout when upgrade button is clicked', async () => {
    const onCheckout = vi.fn();
    const { Billing } = await import('../pages/billing');
    const { getByText } = render(
      <Billing plan="free" onCheckout={onCheckout} onManage={vi.fn()} sessionId={null} />,
    );
    fireEvent.click(getByText(/upgrade/i));
    expect(onCheckout).toHaveBeenCalled();
  });

  it('calls onManage when manage button is clicked', async () => {
    const onManage = vi.fn();
    const { Billing } = await import('../pages/billing');
    const { getByText } = render(
      <Billing plan="pro" onCheckout={vi.fn()} onManage={onManage} sessionId={null} />,
    );
    fireEvent.click(getByText(/manage/i));
    expect(onManage).toHaveBeenCalled();
  });

  it('shows success message when session_id is present (post-checkout)', async () => {
    const { Billing } = await import('../pages/billing');
    const { container } = render(
      <Billing
        plan="free"
        onCheckout={vi.fn()}
        onManage={vi.fn()}
        sessionId="cs_test_123"
      />,
    );
    expect(container.textContent).toMatch(/success|thank|processing|upgraded/i);
  });
});
