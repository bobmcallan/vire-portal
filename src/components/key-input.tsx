import { useState } from 'preact/hooks';
import type { KeyStatus } from '../types';

interface KeyInputProps {
  label: string;
  keyStatus: KeyStatus | undefined;
  onSave: (value: string) => void;
  onDelete: () => void;
}

export function KeyInput({ label, keyStatus, onSave, onDelete }: KeyInputProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');
  const inputId = `key-${label.toLowerCase().replace(/\s+/g, '-')}`;

  const isConfigured = keyStatus?.status === 'valid';
  const isInvalid = keyStatus?.status === 'invalid';

  const handleSave = () => {
    if (value.trim()) {
      onSave(value.trim());
      setValue('');
      setEditing(false);
    }
  };

  return (
    <div class={`border-2 ${!keyStatus ? 'border-dashed' : 'border-solid'} border-black p-4 mb-4`}>
      <div class="flex items-center justify-between mb-2">
        <label for={inputId} class="font-bold text-sm tracking-wide">
          {label}
        </label>
        {isConfigured && (
          <span class="text-sm">[OK] ••••{keyStatus.last4}</span>
        )}
        {isInvalid && (
          <span class="text-sm">[ERR]</span>
        )}
        {!keyStatus && (
          <span class="text-sm italic opacity-60">NOT SET</span>
        )}
      </div>

      {isInvalid && keyStatus.error && (
        <p class="text-sm mb-2">{keyStatus.error}</p>
      )}

      {(!keyStatus || editing) && (
        <div class="flex gap-2">
          <input
            id={inputId}
            type="password"
            value={value}
            autocomplete="off"
            onInput={(e) => setValue((e.target as HTMLInputElement).value)}
            class="flex-1 border-2 border-black px-3 py-2 font-mono text-sm bg-white text-black focus:outline focus:outline-3 focus:outline-black"
            placeholder={`Enter ${label} API key`}
          />
          <button
            class="bg-black text-white border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-white hover:text-black"
            onClick={handleSave}
          >
            {keyStatus ? 'SAVE' : 'ADD'}
          </button>
        </div>
      )}

      {isConfigured && !editing && (
        <div class="flex gap-2 mt-2">
          <button
            class="bg-black text-white border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-white hover:text-black"
            onClick={() => setEditing(true)}
          >
            UPDATE
          </button>
          <button
            class="bg-white text-black border-2 border-black px-4 py-2 text-sm font-mono cursor-pointer hover:bg-black hover:text-white"
            onClick={onDelete}
          >
            DELETE
          </button>
        </div>
      )}
    </div>
  );
}
