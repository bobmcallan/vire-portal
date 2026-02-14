import { render } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import { setState } from './state';
import { setJwt, getJwt, isAuthenticated, refreshToken } from './auth';
import { createApiClient } from './api';
import type { Config, User, UsageData, KeysResponse, McpConfig, ProxyStatus } from './types';
import { Landing } from './pages/landing';
import { Callback } from './pages/callback';
import { Dashboard } from './pages/dashboard';
import { Settings } from './pages/settings';
import { Connect } from './pages/connect';
import { Billing } from './pages/billing';
import { Layout } from './components/layout';
import './styles/main.css';

function isValidRedirectUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:' &&
      (parsed.hostname.endsWith('.stripe.com') || parsed.hostname === 'checkout.stripe.com' || parsed.hostname === 'billing.stripe.com');
  } catch {
    return false;
  }
}

function sanitizeAvatarUrl(url: string): string {
  if (!url) return '';
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== 'https:') return '';
    return url;
  } catch {
    return '';
  }
}

function App() {
  const [config, setConfig] = useState<Config | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [path, setPath] = useState(window.location.pathname);
  const [usage, setUsage] = useState<UsageData | null>(null);
  const [keys, setKeys] = useState<KeysResponse>({});
  const [mcpConfig, setMcpConfig] = useState<McpConfig | null>(null);
  const [proxyStatus, setProxyStatus] = useState<ProxyStatus>({
    proxy_status: 'not_provisioned',
  });
  const lastFetchedPath = useRef<string | null>(null);

  useEffect(() => {
    const onPopState = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const navigate = (newPath: string) => {
    window.history.pushState(null, '', newPath);
    setPath(newPath);
  };

  useEffect(() => {
    fetch('/config.json')
      .then((res) => res.json())
      .then((cfg: Config) => {
        setConfig(cfg);
        setState({ config: cfg });
        return refreshToken(cfg.apiUrl)
          .then((data) => {
            setJwt(data.token);
            setState({ jwt: data.token });
          })
          .catch(() => {
            // No valid session
          });
      })
      .catch(() => {
        // Use env vars as fallback in dev
        const fallback: Config = {
          apiUrl: import.meta.env.VITE_API_URL ?? 'http://localhost:8080',
          domain: import.meta.env.VITE_DOMAIN ?? 'localhost',
        };
        setConfig(fallback);
        setState({ config: fallback });
      })
      .finally(() => setLoading(false));
  }, []);

  const apiClient = config
    ? createApiClient(
        config.apiUrl,
        () => getJwt(),
        () => {
          setJwt(null);
          setUser(null);
          setState({ jwt: null, user: null });
          navigate('/');
        },
        (newToken) => {
          setJwt(newToken);
          setState({ jwt: newToken });
        },
      )
    : null;

  // Fetch profile data only when authenticated and path changes to a protected route
  // Guard against redundant fetches with lastFetchedPath ref
  useEffect(() => {
    if (!apiClient || !isAuthenticated()) return;
    const protectedPaths = ['/dashboard', '/settings', '/connect', '/billing'];
    if (!protectedPaths.includes(path)) return;
    if (lastFetchedPath.current === path && user) return;
    lastFetchedPath.current = path;

    apiClient.getProfile().then((profile) => {
      const u = profile as unknown as User;
      u.avatar_url = sanitizeAvatarUrl(u.avatar_url);
      setUser(u);
      setState({ user: u });
    }).catch(() => {});
    apiClient.getKeys().then((k) => setKeys(k as unknown as KeysResponse)).catch(() => {});
    apiClient.getUsage().then((u) => setUsage(u as unknown as UsageData)).catch(() => {});
    apiClient.getMcpConfig().then((m) => setMcpConfig(m as unknown as McpConfig)).catch(() => {});
    apiClient.getProxyStatus().then((s) => setProxyStatus(s as unknown as ProxyStatus)).catch(() => {});
  }, [config, path]);

  if (loading) {
    return (
      <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
        <p class="text-xl tracking-widest">LOADING...</p>
      </div>
    );
  }

  if (!config) return null;

  // Public routes
  if (path === '/') {
    return <Landing apiUrl={config.apiUrl} />;
  }

  if (path === '/auth/callback') {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    const callbackState = params.get('state');

    // Handle missing query params
    if (!code || !callbackState) {
      return (
        <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
          <div class="text-center space-y-4">
            <p class="text-xl font-bold">[ERR] INVALID CALLBACK</p>
            <p class="text-sm">Missing authorization parameters.</p>
            <a
              href="/"
              class="inline-block border-2 border-black bg-black text-white py-2 px-6 text-sm no-underline hover:bg-white hover:text-black"
            >
              BACK TO HOME
            </a>
          </div>
        </div>
      );
    }

    return (
      <Callback
        code={code}
        state={callbackState}
        apiUrl={config.apiUrl}
        onSuccess={(token, u) => {
          u.avatar_url = sanitizeAvatarUrl(u.avatar_url);
          setJwt(token);
          setUser(u);
          setState({ jwt: token, user: u });
          navigate('/dashboard');
        }}
      />
    );
  }

  // Auth guard for protected routes -- use useEffect-based redirect to avoid
  // infinite render loop from calling navigate() during render
  if (!isAuthenticated()) {
    // Schedule redirect instead of calling navigate() synchronously
    setTimeout(() => navigate('/'), 0);
    return (
      <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
        <p class="text-xl tracking-widest">REDIRECTING...</p>
      </div>
    );
  }

  const handleLogout = async () => {
    if (apiClient) {
      try {
        await fetch(`${config.apiUrl}/api/auth/logout`, {
          method: 'POST',
          credentials: 'include',
        });
      } catch { /* ignore logout errors */ }
    }
    setJwt(null);
    setUser(null);
    setState({ jwt: null, user: null });
    navigate('/');
  };

  const handleUpdateKeys = async (k: Record<string, string>) => {
    if (!apiClient) return;
    try {
      const result = await apiClient.updateKeys(k);
      setKeys(result as unknown as KeysResponse);
    } catch { /* handled by API client */ }
  };

  const handleUpdateProfile = async (d: Record<string, unknown>) => {
    if (!apiClient) return;
    try {
      const result = await apiClient.updateProfile(d);
      const u = result as unknown as User;
      u.avatar_url = sanitizeAvatarUrl(u.avatar_url);
      setUser(u);
      setState({ user: u });
    } catch { /* handled by API client */ }
  };

  const handleDeleteAccount = async () => {
    if (!apiClient) return;
    try {
      await apiClient.deleteProfile();
      setJwt(null);
      setUser(null);
      setState({ jwt: null, user: null });
      navigate('/');
    } catch { /* handled by API client */ }
  };

  const content = (() => {
    switch (path) {
      case '/dashboard':
        if (!usage || !user) return <p>Loading...</p>;
        return <Dashboard usage={usage} user={user} />;

      case '/settings':
        if (!user) return <p>Loading...</p>;
        return (
          <Settings
            user={user}
            keys={keys}
            onUpdateKeys={handleUpdateKeys}
            onUpdateProfile={handleUpdateProfile}
            onDeleteAccount={handleDeleteAccount}
          />
        );

      case '/connect':
        return <Connect mcpConfig={mcpConfig} proxyStatus={proxyStatus} />;

      case '/billing':
        return (
          <Billing
            plan={user?.plan ?? 'free'}
            sessionId={new URLSearchParams(window.location.search).get('session_id')}
            onCheckout={async () => {
              const res = await apiClient?.createCheckout();
              if (res && 'checkout_url' in res) {
                const url = res.checkout_url as string;
                if (isValidRedirectUrl(url)) {
                  window.location.href = url;
                }
              }
            }}
            onManage={async () => {
              const res = await apiClient?.createBillingPortal();
              if (res && 'portal_url' in res) {
                const url = res.portal_url as string;
                if (isValidRedirectUrl(url)) {
                  window.location.href = url;
                }
              }
            }}
          />
        );

      default:
        return (
          <div class="text-center py-12">
            <p class="text-2xl font-bold">[404]</p>
            <p class="text-sm mt-2">Page not found</p>
          </div>
        );
    }
  })();

  return (
    <Layout currentPath={path} onLogout={handleLogout} onNavigate={navigate}>
      {content}
    </Layout>
  );
}

const root = document.getElementById('app');
if (root) {
  render(<App />, root);
}
