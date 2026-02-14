import { useState } from 'preact/hooks';
import type { User, KeyStatus } from '../types';
import { KeyInput } from '../components/key-input';

interface SettingsProps {
  user: User;
  keys: {
    eodhd_key?: KeyStatus;
    navexa_key?: KeyStatus;
    gemini_key?: KeyStatus;
  };
  onUpdateKeys: (keys: Record<string, string>) => void;
  onUpdateProfile: (data: Record<string, unknown>) => void;
  onDeleteAccount: () => void;
}

export function Settings({ user, keys, onUpdateKeys, onUpdateProfile, onDeleteAccount }: SettingsProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [portfolio, setPortfolio] = useState(user.default_portfolio ?? '');
  const [exchange, setExchange] = useState(user.exchange ?? '');

  return (
    <div class="space-y-8">
      <div class="border-b-2 border-black pb-4">
        <h1 class="text-2xl font-bold tracking-widest">SETTINGS</h1>
      </div>

      <section class="border-2 border-black p-6 space-y-3">
        <h2 class="text-sm font-bold tracking-wide mb-4">PROFILE</h2>
        <div class="flex items-center gap-4 mb-4">
          {user.avatar_url && (
            <img
              src={user.avatar_url}
              alt=""
              class="w-12 h-12 border-2 border-black"
            />
          )}
          <div>
            <p class="font-bold">{user.display_name}</p>
            <p class="text-sm">{user.email}</p>
          </div>
        </div>
      </section>

      <section class="space-y-2">
        <h2 class="text-sm font-bold tracking-wide mb-4">API KEYS</h2>
        <KeyInput
          label="EODHD"
          keyStatus={keys.eodhd_key}
          onSave={(val) => onUpdateKeys({ eodhd_key: val })}
          onDelete={() => {}}
        />
        <KeyInput
          label="NAVEXA"
          keyStatus={keys.navexa_key}
          onSave={(val) => onUpdateKeys({ navexa_key: val })}
          onDelete={() => {}}
        />
        <KeyInput
          label="GEMINI"
          keyStatus={keys.gemini_key}
          onSave={(val) => onUpdateKeys({ gemini_key: val })}
          onDelete={() => {}}
        />
      </section>

      <section class="border-2 border-black p-6 space-y-4">
        <h2 class="text-sm font-bold tracking-wide">PREFERENCES</h2>
        <div>
          <label for="portfolio" class="block text-sm font-bold mb-1">
            DEFAULT PORTFOLIO
          </label>
          <select
            id="portfolio"
            value={portfolio}
            onChange={(e) => {
              const val = (e.target as HTMLSelectElement).value;
              setPortfolio(val);
              onUpdateProfile({ default_portfolio: val });
            }}
            class="border-2 border-black px-3 py-2 font-mono text-sm bg-white text-black w-full"
          >
            <option value="">Select...</option>
            {(user.portfolios ?? []).map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label for="exchange" class="block text-sm font-bold mb-1">
            EXCHANGE
          </label>
          <select
            id="exchange"
            value={exchange}
            onChange={(e) => {
              const val = (e.target as HTMLSelectElement).value;
              setExchange(val);
              onUpdateProfile({ exchange: val });
            }}
            class="border-2 border-black px-3 py-2 font-mono text-sm bg-white text-black w-full"
          >
            <option value="AU">AU</option>
            <option value="US">US</option>
            <option value="UK">UK</option>
          </select>
        </div>
      </section>

      <section class="border-2 border-black p-6">
        <h2 class="text-sm font-bold tracking-wide mb-4">DANGER ZONE</h2>
        {!confirmDelete ? (
          <button
            class="bg-black text-white border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-white hover:text-black"
            onClick={() => setConfirmDelete(true)}
          >
            DELETE ACCOUNT
          </button>
        ) : (
          <div class="space-y-3">
            <p class="text-sm">Are you sure? This action cannot be undone.</p>
            <div class="flex gap-2">
              <button
                class="bg-black text-white border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-white hover:text-black"
                onClick={() => {
                  onDeleteAccount();
                  setConfirmDelete(false);
                }}
              >
                CONFIRM DELETE
              </button>
              <button
                class="bg-white text-black border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-black hover:text-white"
                onClick={() => setConfirmDelete(false)}
              >
                CANCEL
              </button>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
