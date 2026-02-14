import { useEffect, useState } from 'preact/hooks';
import type { User } from '../types';
import { getStoredProvider } from '../auth';

interface CallbackProps {
  code: string;
  state: string;
  apiUrl?: string;
  onSuccess?: (token: string, user: User) => void;
}

export function Callback({ code, state, apiUrl, onSuccess }: CallbackProps) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const provider = getStoredProvider();

    if (!apiUrl) {
      setLoading(false);
      setError('Application not configured. Please try again.');
      return;
    }

    fetch(`${apiUrl}/api/auth/callback`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ provider, code, state }),
    })
      .then(async (res) => {
        if (!res.ok) {
          const err = await res.json();
          throw new Error(err.error?.message ?? 'Authentication failed');
        }
        return res.json();
      })
      .then((data) => {
        setLoading(false);
        onSuccess?.(data.token, data.user);
      })
      .catch((err) => {
        setLoading(false);
        setError(err.message);
      });
  }, [code, state, apiUrl]);

  if (loading) {
    return (
      <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
        <div class="text-center">
          <p class="text-2xl font-bold tracking-widest">AUTHENTICATING...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
        <div class="text-center space-y-4">
          <p class="text-xl font-bold">[ERR] AUTHENTICATION FAILED</p>
          <p class="text-sm">{error}</p>
          <a
            href="/"
            class="inline-block border-2 border-black bg-black text-white py-2 px-6 text-sm no-underline hover:bg-white hover:text-black"
          >
            TRY AGAIN
          </a>
        </div>
      </div>
    );
  }

  return (
    <div class="min-h-screen bg-white text-black font-mono flex items-center justify-center">
      <p class="text-lg">Redirecting...</p>
    </div>
  );
}
