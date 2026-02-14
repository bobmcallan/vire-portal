export interface Config {
  apiUrl: string;
  domain: string;
}

export interface User {
  user_id: string;
  email: string;
  display_name: string;
  avatar_url: string;
  auth_provider: string;
  created_at: string;
  status: string;
  keys_configured: boolean;
  default_portfolio?: string;
  portfolios?: string[];
  exchange?: string;
  plan: string;
  proxy_url?: string;
  proxy_status?: string;
  provisioned_at?: string;
}

export interface AppState {
  user: User | null;
  jwt: string | null;
  config: Config | null;
}

export interface AuthCallbackResponse {
  token: string;
  user: User;
}

export interface RefreshResponse {
  token: string;
}

export interface KeyStatus {
  status: 'valid' | 'invalid';
  last4?: string;
  validated_at?: string;
  portfolios_found?: number;
  error?: string;
}

export interface KeysResponse {
  eodhd_key?: KeyStatus;
  navexa_key?: KeyStatus;
  gemini_key?: KeyStatus;
}

export interface UsageData {
  period: string;
  total_requests: number;
  quota_limit: number;
  quota_remaining: number;
  status: string;
  daily_counts: Array<{ date: string; count: number }>;
  top_endpoints: Array<{ endpoint: string; count: number }>;
}

export interface McpConfig {
  proxy_url: string;
  claude_code_config: Record<string, unknown>;
  claude_desktop_config: Record<string, unknown>;
}

export interface ProxyStatus {
  proxy_status: 'running' | 'stopped' | 'not_provisioned' | 'throttled';
  last_activity?: string;
  proxy_url?: string;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
