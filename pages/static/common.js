// VIRE Portal - Client utilities and Alpine.js components
// Global debug flag — toggle in browser console: window.VIRE_DEBUG = true
window.VIRE_DEBUG = typeof window.VIRE_CLIENT_DEBUG !== 'undefined' ? window.VIRE_CLIENT_DEBUG : false;

// Debug logger — conditional, timestamped
window.debugLog = function (component, message, ...args) {
    if (window.VIRE_DEBUG) {
        const timestamp = new Date().toISOString().split('T')[1].split('.')[0];
        console.log(`[${timestamp}] [${component}]`, message, ...args);
    }
};

// Error logger — always on, includes stack trace
window.debugError = function (component, message, error) {
    const timestamp = new Date().toISOString().split('T')[1].split('.')[0];
    console.error(`[${timestamp}] [${component}]`, message, error);
    if (error && error.stack) {
        console.error(`[${timestamp}] [${component}] Stack:`, error.stack);
    }
};

// CSRF: inject _csrf hidden field into all POST forms from the _csrf cookie.
// The server sets _csrf as a non-HttpOnly cookie on GET responses.
document.addEventListener('DOMContentLoaded', () => {
    const csrfCookie = document.cookie.split('; ').find(c => c.startsWith('_csrf='));
    if (!csrfCookie) return;
    const csrfToken = csrfCookie.split('=')[1];
    if (!csrfToken) return;

    document.querySelectorAll('form[method="POST"]').forEach(form => {
        if (form.querySelector('input[name="_csrf"]')) return; // already has one
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = '_csrf';
        input.value = csrfToken;
        form.appendChild(input);
    });
});

document.addEventListener('alpine:init', () => {

    // Dropdown
    Alpine.data('dropdown', () => ({
        open: false,
        toggle() { this.open = !this.open; },
        close()  { this.open = false; },
    }));

    // Nav Menu (hamburger: desktop dropdown + mobile slide-out)
    Alpine.data('navMenu', () => ({
        dropdownOpen: false,
        mobileOpen: false,
        isMobile() { return window.innerWidth <= 768; },
        toggle() {
            if (this.isMobile()) { this.mobileOpen = true; this.dropdownOpen = false; }
            else { this.dropdownOpen = !this.dropdownOpen; this.mobileOpen = false; }
        },
        closeDropdown() { this.dropdownOpen = false; },
        closeMobile() { this.mobileOpen = false; },
    }));

    // Tabs
    Alpine.data('tabs', (initial) => ({
        current: initial || '',
        select(t)   { this.current = t; },
        isActive(t) { return this.current === t ? 'active' : ''; },
    }));

    // Collapsible Panel
    Alpine.data('collapse', (startOpen) => ({
        open: startOpen || false,
        toggle() { this.open = !this.open; },
        arrow()  { return this.open ? 'transform:rotate(180deg);transition:transform 0.15s' : 'transition:transform 0.15s'; },
    }));

    // Toast Notifications
    Alpine.data('toasts', () => ({
        list: [],
        add(detail) {
            const t = {
                id: Date.now() + Math.random(),
                msg: detail.msg || '',
                dark: detail.dark || false,
            };
            this.list.push(t);
            setTimeout(() => {
                this.list = this.list.filter(x => x.id !== t.id);
            }, detail.duration || 3000);
        },
    }));

    // Confirm Action
    Alpine.data('confirm', (message) => ({
        ask(action) {
            if (window.confirm(message)) action();
        },
    }));

    // Status Indicators
    Alpine.data('statusIndicators', () => ({
        portal: 'startup',
        server: 'startup',
        init() {
            this.check();
            setInterval(() => this.check(), 5000);
        },
        async check() {
            try {
                const pr = await fetch('/api/health', { signal: AbortSignal.timeout(3000) });
                this.portal = pr.ok ? 'up' : 'down';
            } catch { this.portal = 'down'; }
            try {
                const sr = await fetch('/api/server-health', { signal: AbortSignal.timeout(3000) });
                this.server = sr.ok ? 'up' : 'down';
            } catch { this.server = 'down'; }
        },
    }));

});

// Data store: caches API responses, deduplicates arrays, prevents concurrent fetches.
const vireStore = {
    _cache: {},       // URL -> {data, expiry}
    _inflight: {},    // URL -> Promise (single-flight)
    _ttl: 30000,      // 30s default TTL

    async fetch(url, options) {
        // Check cache first
        const cached = this._cache[url];
        if (cached && Date.now() < cached.expiry) {
            return cached.data.clone();
        }

        // Check inflight — return same promise if already fetching
        if (this._inflight[url]) {
            const resp = await this._inflight[url];
            return resp.clone();
        }

        // Fetch, store in cache, return
        const promise = fetch(url, options).then(resp => {
            if (resp.ok) {
                this._cache[url] = { data: resp, expiry: Date.now() + this._ttl };
            }
            delete this._inflight[url];
            return resp;
        }).catch(err => {
            delete this._inflight[url];
            throw err;
        });

        this._inflight[url] = promise;
        return (await promise).clone();
    },

    invalidate(urlPrefix) {
        for (const key of Object.keys(this._cache)) {
            if (key.startsWith(urlPrefix)) {
                delete this._cache[key];
            }
        }
    },

    dedup(array, key) {
        const seen = new Set();
        return array.filter(item => {
            const k = item[key];
            if (seen.has(k)) return false;
            seen.add(k);
            return true;
        });
    }
};

// Portfolio Dashboard component
function portfolioDashboard() {
    return {
        portfolios: [],
        selected: '',
        defaultPortfolio: '',
        holdings: [],
        showClosed: false,
        strategy: '',
        plan: '',
        portfolioGain: 0,
        portfolioGainPct: 0,
        loading: true,
        error: '',
        get isDefault() { return this.selected === this.defaultPortfolio; },
        get filteredHoldings() {
            let h = this.holdings.slice();
            if (!this.showClosed) {
                h = h.filter(x => x.market_value !== 0);
            }
            h.sort((a, b) => (a.ticker || '').localeCompare(b.ticker || ''));
            return h;
        },
        get totalValue() {
            return this.filteredHoldings.reduce((sum, h) => sum + (Number(h.market_value) || 0), 0);
        },
        get totalGain() {
            return this.portfolioGain;
        },
        get totalGainPct() {
            return this.portfolioGainPct;
        },
        gainClass(val) {
            if (val == null || val === 0) return '';
            return val > 0 ? 'gain-positive' : 'gain-negative';
        },

        async init() {
            try {
                const res = await vireStore.fetch('/api/portfolios');
                if (!res.ok) {
                    this.error = 'Failed to load portfolios';
                    this.loading = false;
                    return;
                }
                const data = await res.json();
                this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
                this.defaultPortfolio = data.default || '';
                if (this.defaultPortfolio) {
                    this.selected = this.defaultPortfolio;
                } else if (this.portfolios.length > 0) {
                    this.selected = this.portfolios[0].name;
                }
                if (this.selected) await this.loadPortfolio();
            } catch (e) {
                debugError('portfolioDashboard', 'init failed', e);
                this.error = 'Failed to connect to server';
            } finally {
                this.loading = false;
            }
        },

        async loadPortfolio() {
            if (!this.selected) return;
            try {
                const [holdingsRes, strategyRes, planRes] = await Promise.all([
                    vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected)),
                    vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/strategy'),
                    vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/plan'),
                ]);

                if (holdingsRes.ok) {
                    const holdingsData = await holdingsRes.json();
                    this.holdings = vireStore.dedup(holdingsData.holdings || [], 'ticker');
                    this.portfolioGain = Number(holdingsData.total_gain) || 0;
                    this.portfolioGainPct = Number(holdingsData.total_gain_pct) || 0;
                } else {
                    this.holdings = [];
                    this.portfolioGain = 0;
                    this.portfolioGainPct = 0;
                }

                if (strategyRes.ok) {
                    const strategyData = await strategyRes.json();
                    this.strategy = strategyData.notes || JSON.stringify(strategyData.strategy || strategyData, null, 2);
                } else {
                    this.strategy = '';
                }

                if (planRes.ok) {
                    const planData = await planRes.json();
                    this.plan = planData.notes || JSON.stringify(planData.plan || planData, null, 2);
                } else {
                    this.plan = '';
                }
            } catch (e) {
                debugError('portfolioDashboard', 'loadPortfolio failed', e);
            }
        },

        async toggleDefault() {
            try {
                if (this.isDefault) {
                    await fetch('/api/portfolios/default', {
                        method: 'PUT',
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({}),
                    });
                    this.defaultPortfolio = '';
                } else {
                    await fetch('/api/portfolios/default', {
                        method: 'PUT',
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({ name: this.selected }),
                    });
                    this.defaultPortfolio = this.selected;
                }
                vireStore.invalidate('/api/portfolios');
                window.dispatchEvent(new CustomEvent('toast', { detail: { msg: 'Default updated' } }));
            } catch (e) {
                debugError('portfolioDashboard', 'toggleDefault failed', e);
            }
        },

        async saveStrategy() {
            try {
                await fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/strategy', {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ strategy: this.strategy }),
                });
                vireStore.invalidate('/api/portfolios');
                window.dispatchEvent(new CustomEvent('toast', { detail: { msg: 'Strategy saved' } }));
            } catch (e) {
                debugError('portfolioDashboard', 'saveStrategy failed', e);
            }
        },

        async savePlan() {
            try {
                await fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/plan', {
                    method: 'PUT',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ notes: this.plan }),
                });
                vireStore.invalidate('/api/portfolios');
                window.dispatchEvent(new CustomEvent('toast', { detail: { msg: 'Plan saved' } }));
            } catch (e) {
                debugError('portfolioDashboard', 'savePlan failed', e);
            }
        },

        fmt(val) {
            return val != null ? Number(val).toLocaleString('en-AU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '-';
        },
        pct(val) {
            return val != null ? Number(val).toFixed(2) + '%' : '-';
        },
    };
}
