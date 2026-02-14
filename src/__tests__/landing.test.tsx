import { describe, it, expect, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/preact';

/**
 * Tests for src/pages/landing.tsx
 *
 * The landing page renders:
 * - "VIRE" title
 * - Product tagline
 * - Sign-in with Google button (anchor tag to gateway)
 * - Sign-in with GitHub button (anchor tag to gateway)
 * - Feature overview
 *
 * Sign-in buttons are <a> tags (not fetch calls) pointing to the gateway's
 * /api/auth/login/:provider endpoint. Each button's onClick stores the
 * provider name in sessionStorage for the callback page.
 */

describe('Landing page', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('renders the VIRE title', async () => {
    const { Landing } = await import('../pages/landing');
    const { getByText } = render(<Landing apiUrl="https://api.test" />);
    expect(getByText('VIRE')).toBeTruthy();
  });

  it('renders a product tagline', async () => {
    const { Landing } = await import('../pages/landing');
    const { container } = render(<Landing apiUrl="https://api.test" />);
    // Should have some descriptive text about the product
    expect(container.textContent).toMatch(/mcp|portfolio|stock|api/i);
  });

  it('renders Google sign-in as an anchor tag pointing to gateway', async () => {
    const { Landing } = await import('../pages/landing');
    const { getByText } = render(<Landing apiUrl="https://api.test" />);
    const googleLink = getByText(/sign in with google/i);
    expect(googleLink.tagName).toBe('A');
    expect(googleLink.getAttribute('href')).toBe(
      'https://api.test/api/auth/login/google',
    );
  });

  it('renders GitHub sign-in as an anchor tag pointing to gateway', async () => {
    const { Landing } = await import('../pages/landing');
    const { getByText } = render(<Landing apiUrl="https://api.test" />);
    const githubLink = getByText(/sign in with github/i);
    expect(githubLink.tagName).toBe('A');
    expect(githubLink.getAttribute('href')).toBe(
      'https://api.test/api/auth/login/github',
    );
  });

  it('stores "google" in sessionStorage when Google sign-in is clicked', async () => {
    const { Landing } = await import('../pages/landing');
    const { getByText } = render(<Landing apiUrl="https://api.test" />);
    const googleLink = getByText(/sign in with google/i);
    fireEvent.click(googleLink);
    expect(sessionStorage.getItem('oauth_provider')).toBe('google');
  });

  it('stores "github" in sessionStorage when GitHub sign-in is clicked', async () => {
    const { Landing } = await import('../pages/landing');
    const { getByText } = render(<Landing apiUrl="https://api.test" />);
    const githubLink = getByText(/sign in with github/i);
    fireEvent.click(githubLink);
    expect(sessionStorage.getItem('oauth_provider')).toBe('github');
  });
});
