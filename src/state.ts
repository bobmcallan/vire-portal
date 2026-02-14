import type { AppState } from './types';

type Listener = () => void;

let state: AppState = {
  user: null,
  jwt: null,
  config: null,
};

const listeners = new Set<Listener>();

export function getState(): AppState {
  return state;
}

export function setState(partial: Partial<AppState>): void {
  state = { ...state, ...partial };
  listeners.forEach((fn) => fn());
}

export function resetState(): void {
  state = { user: null, jwt: null, config: null };
  listeners.forEach((fn) => fn());
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
