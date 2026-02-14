import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/preact';

/**
 * Tests for src/components/copy-block.tsx
 *
 * The copy-block component displays:
 * - Pre-formatted code block with black border
 * - Copy button
 * - "COPIED" feedback with aria-live for screen readers
 */

describe('CopyBlock component', () => {
  beforeEach(() => {
    // Mock clipboard API
    const mockClipboard = {
      writeText: vi.fn().mockResolvedValue(undefined),
    };
    Object.defineProperty(navigator, 'clipboard', {
      value: mockClipboard,
      writable: true,
      configurable: true,
    });
  });

  it('displays the provided code content', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { container } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    expect(container.textContent).toMatch(/"key": "value"/);
  });

  it('displays the title', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { getByText } = render(
      <CopyBlock title="Claude Code" content='{"config": true}' />,
    );
    expect(getByText('Claude Code')).toBeTruthy();
  });

  it('renders a copy button', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { getByText } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    expect(getByText(/copy/i)).toBeTruthy();
  });

  it('copies content to clipboard when copy button is clicked', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { getByText } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    fireEvent.click(getByText(/copy/i));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      '{"key": "value"}',
    );
  });

  it('shows "COPIED" feedback after copying', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { getByText, container } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    fireEvent.click(getByText(/^copy$/i));
    await waitFor(() => {
      expect(container.textContent).toMatch(/copied/i);
    });
  });

  it('has aria-live region for copy confirmation', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { container } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    const liveRegion = container.querySelector('[aria-live]');
    expect(liveRegion).toBeTruthy();
  });

  it('renders code in a pre element', async () => {
    const { CopyBlock } = await import('../components/copy-block');
    const { container } = render(
      <CopyBlock title="Config" content='{"key": "value"}' />,
    );
    const pre = container.querySelector('pre');
    expect(pre).toBeTruthy();
  });
});
