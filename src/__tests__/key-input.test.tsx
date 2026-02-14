import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';

/**
 * Tests for src/components/key-input.tsx
 *
 * The key-input component displays API key status in B&W:
 * - Not configured: dashed border, "NOT SET" text
 * - Configured: solid border, masked "••••last4", [UPDATE] button
 * - Connected: thick border, [OK] prefix, context info
 * - Invalid: [ERR] prefix, error message
 *
 * No color is used -- states are distinguished by text labels,
 * border styles, and opacity.
 */

describe('KeyInput component', () => {
  describe('not configured state', () => {
    it('shows "NOT SET" text when key is not configured', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(container.textContent).toMatch(/not set/i);
    });

    it('shows an add/save button when not configured', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { getByText } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(getByText(/add|save/i)).toBeTruthy();
    });

    it('renders the label', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { getByText } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(getByText('EODHD')).toBeTruthy();
    });
  });

  describe('configured state', () => {
    it('shows masked value with last 4 characters', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={{ status: 'valid', last4: 'c123', validated_at: '2026-02-09T10:05:00Z' }}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(container.textContent).toMatch(/c123/);
    });

    it('shows update button when key is configured', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { getByText } = render(
        <KeyInput
          label="EODHD"
          keyStatus={{ status: 'valid', last4: 'c123', validated_at: '2026-02-09T10:05:00Z' }}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(getByText(/update/i)).toBeTruthy();
    });
  });

  describe('invalid state', () => {
    it('shows error prefix text', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={{ status: 'invalid', error: 'API returned 401' }}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(container.textContent).toMatch(/err/i);
    });

    it('shows error message', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={{ status: 'invalid', error: 'API returned 401' }}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      expect(container.textContent).toMatch(/API returned 401/);
    });
  });

  describe('interactions', () => {
    it('has an input field for entering a key', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      const input = container.querySelector('input');
      expect(input).toBeTruthy();
    });

    it('calls onSave with the entered key value', async () => {
      const onSave = vi.fn();
      const { KeyInput } = await import('../components/key-input');
      const { container, getByText } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={onSave}
          onDelete={vi.fn()}
        />,
      );
      const input = container.querySelector('input')!;
      fireEvent.input(input, { target: { value: 'my-api-key' } });
      fireEvent.click(getByText(/add|save/i));
      expect(onSave).toHaveBeenCalledWith('my-api-key');
    });

    it('has proper label association via htmlFor', async () => {
      const { KeyInput } = await import('../components/key-input');
      const { container } = render(
        <KeyInput
          label="EODHD"
          keyStatus={undefined}
          onSave={vi.fn()}
          onDelete={vi.fn()}
        />,
      );
      const label = container.querySelector('label');
      const input = container.querySelector('input');
      expect(label).toBeTruthy();
      expect(input).toBeTruthy();
      if (label && input) {
        expect(label.getAttribute('for')).toBe(input.getAttribute('id'));
      }
    });
  });
});
