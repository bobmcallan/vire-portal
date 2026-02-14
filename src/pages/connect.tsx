import type { McpConfig, ProxyStatus } from '../types';
import { CopyBlock } from '../components/copy-block';

interface ConnectProps {
  mcpConfig: McpConfig | null;
  proxyStatus: ProxyStatus;
}

export function Connect({ mcpConfig, proxyStatus }: ConnectProps) {
  const statusLabel = proxyStatus.proxy_status.toUpperCase().replace('_', ' ');

  if (proxyStatus.proxy_status === 'not_provisioned' || !mcpConfig) {
    return (
      <div class="space-y-8">
        <div class="border-b-2 border-black pb-4">
          <h1 class="text-2xl font-bold tracking-widest">CONNECT</h1>
        </div>
        <div class="border-2 border-black p-6 text-center space-y-4">
          <p class="text-lg font-bold">[{statusLabel}]</p>
          <p class="text-sm">
            Provision your MCP proxy first from the Settings page.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div class="space-y-8">
      <div class="flex items-center justify-between border-b-2 border-black pb-4">
        <h1 class="text-2xl font-bold tracking-widest">CONNECT</h1>
        <span class="text-sm">[{statusLabel}]</span>
      </div>

      <div class="border-2 border-black p-4">
        <div class="text-sm">
          <span class="font-bold">PROXY URL:</span>{' '}
          <span class="break-all">{mcpConfig.proxy_url}</span>
        </div>
      </div>

      <CopyBlock
        title="Claude Code"
        content={JSON.stringify(mcpConfig.claude_code_config, null, 2)}
      />

      <CopyBlock
        title="Claude Desktop"
        content={JSON.stringify(mcpConfig.claude_desktop_config, null, 2)}
      />
    </div>
  );
}
