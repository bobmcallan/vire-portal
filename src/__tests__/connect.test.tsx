import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/preact';

/**
 * Tests for src/pages/connect.tsx
 *
 * The connect page displays:
 * - MCP proxy status
 * - Claude Code config block with copy button
 * - Claude Desktop config block with copy button
 */

const mockMcpConfig = {
  proxy_url: 'https://vire-mcp-a1b2c3-xyz.run.app',
  claude_code_config: {
    mcpServers: {
      vire: {
        type: 'http',
        url: 'https://vire-mcp-a1b2c3-xyz.run.app/mcp',
      },
    },
  },
  claude_desktop_config: {
    mcpServers: {
      vire: {
        url: 'https://vire-mcp-a1b2c3-xyz.run.app/mcp',
      },
    },
  },
};

const mockStatus = {
  proxy_status: 'running' as const,
  last_activity: '2026-02-09T12:30:00Z',
  proxy_url: 'https://vire-mcp-a1b2c3-xyz.run.app',
};

describe('Connect page', () => {
  it('displays proxy status', async () => {
    const { Connect } = await import('../pages/connect');
    const { container } = render(
      <Connect mcpConfig={mockMcpConfig} proxyStatus={mockStatus} />,
    );
    expect(container.textContent).toMatch(/running/i);
  });

  it('displays Claude Code config block', async () => {
    const { Connect } = await import('../pages/connect');
    const { container } = render(
      <Connect mcpConfig={mockMcpConfig} proxyStatus={mockStatus} />,
    );
    expect(container.textContent).toMatch(/claude code/i);
    expect(container.textContent).toMatch(/mcpServers/);
  });

  it('displays Claude Desktop config block', async () => {
    const { Connect } = await import('../pages/connect');
    const { container } = render(
      <Connect mcpConfig={mockMcpConfig} proxyStatus={mockStatus} />,
    );
    expect(container.textContent).toMatch(/claude desktop/i);
  });

  it('has copy buttons for both config blocks', async () => {
    const { Connect } = await import('../pages/connect');
    const { getAllByText } = render(
      <Connect mcpConfig={mockMcpConfig} proxyStatus={mockStatus} />,
    );
    const copyButtons = getAllByText(/copy/i);
    expect(copyButtons.length).toBeGreaterThanOrEqual(2);
  });

  it('displays the proxy URL', async () => {
    const { Connect } = await import('../pages/connect');
    const { container } = render(
      <Connect mcpConfig={mockMcpConfig} proxyStatus={mockStatus} />,
    );
    expect(container.textContent).toMatch(/vire-mcp-a1b2c3/);
  });

  it('shows not provisioned state when proxy is not provisioned', async () => {
    const { Connect } = await import('../pages/connect');
    const notProvisioned = {
      proxy_status: 'not_provisioned' as const,
    };
    const { container } = render(
      <Connect mcpConfig={null} proxyStatus={notProvisioned} />,
    );
    expect(container.textContent).toMatch(/not.?provisioned|provision/i);
  });
});
