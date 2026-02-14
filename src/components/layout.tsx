import { useState, useEffect, useRef } from 'preact/hooks';
import type { ComponentChildren } from 'preact';

interface LayoutProps {
  currentPath: string;
  onLogout: () => void;
  onNavigate: (path: string) => void;
  children: ComponentChildren;
}

const NAV_LINKS = [
  { path: '/dashboard', label: 'DASHBOARD' },
  { path: '/settings', label: 'SETTINGS' },
  { path: '/connect', label: 'CONNECT' },
  { path: '/billing', label: 'BILLING' },
];

export function Layout({ currentPath, onLogout, onNavigate, children }: LayoutProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setMenuOpen(false);
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [menuOpen]);

  return (
    <div class="min-h-screen bg-white text-black font-mono flex flex-col">
      <a
        href="#main-content"
        class="sr-only focus:not-sr-only focus:absolute focus:top-0 focus:left-0 focus:z-50 focus:bg-black focus:text-white focus:px-4 focus:py-2 focus:text-sm"
      >
        Skip to content
      </a>

      <nav class="bg-black text-white border-b-2 border-black">
        <div class="max-w-5xl mx-auto px-4 py-3 flex items-center justify-between">
          <a
            href="/"
            class="text-xl font-bold tracking-widest no-underline text-white"
            onClick={(e) => {
              e.preventDefault();
              onNavigate('/');
            }}
          >
            VIRE
          </a>

          <button
            aria-label="Toggle menu"
            aria-expanded={menuOpen}
            data-testid="menu-toggle"
            class="md:hidden bg-black text-white border border-white px-2 py-1 cursor-pointer"
            onClick={() => setMenuOpen(!menuOpen)}
          >
            {menuOpen ? '[X]' : '[=]'}
          </button>

          <div
            ref={menuRef}
            role="menu"
            class={`${menuOpen ? 'flex' : 'hidden'} md:flex flex-col md:flex-row items-start md:items-center gap-4 absolute md:static top-14 left-0 right-0 bg-black md:bg-transparent p-4 md:p-0 z-10`}
          >
            {NAV_LINKS.map(({ path, label }) => (
              <a
                key={path}
                href={path}
                role="menuitem"
                class={`text-white no-underline text-sm tracking-wide hover:bg-white hover:text-black px-2 py-1 ${
                  currentPath === path ? 'underline' : ''
                }`}
                onClick={(e) => {
                  e.preventDefault();
                  onNavigate(path);
                  setMenuOpen(false);
                }}
              >
                {label}
              </a>
            ))}
            <button
              role="menuitem"
              class="text-white text-sm tracking-wide hover:bg-white hover:text-black px-2 py-1 bg-transparent border-none cursor-pointer font-mono"
              onClick={onLogout}
            >
              LOGOUT
            </button>
          </div>
        </div>
      </nav>

      <main id="main-content" class="flex-1 max-w-5xl mx-auto w-full px-4 py-8">
        {children}
      </main>

      <footer class="border-t-2 border-black py-4 text-center text-xs opacity-60">
        VIRE v0.1.0
      </footer>
    </div>
  );
}
