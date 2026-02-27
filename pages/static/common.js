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
        portfolioGain: 0,
        portfolioGainPct: 0,
        portfolioCost: 0,
        capitalInvested: 0,
        capitalGain: 0,
        simpleReturnPct: 0,
        annualizedReturnPct: 0,
        hasCapitalData: false,
        refreshing: false,
        trend: '',
        rsiSignal: '',
        dataPoints: 0,
        hasIndicators: false,
        growthData: [],
        hasGrowthData: false,
        chartInstance: null,
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
        get totalCost() {
            return this.portfolioCost;
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
                const holdingsRes = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected));

                if (holdingsRes.ok) {
                    const holdingsData = await holdingsRes.json();
                    this.holdings = vireStore.dedup(holdingsData.holdings || [], 'ticker');
                    this.portfolioGain = Number(holdingsData.total_net_return) || 0;
                    this.portfolioGainPct = Number(holdingsData.total_net_return_pct) || 0;
                    this.portfolioCost = Number(holdingsData.total_cost) || 0;
                    // Parse capital performance
                    const cp = holdingsData.capital_performance;
                    if (cp && cp.transaction_count > 0) {
                        this.capitalInvested = Number(cp.net_capital_deployed) || 0;
                        // Derive capital gain from actual holdings total, not server's current_portfolio_value
                        this.capitalGain = this.totalValue - this.capitalInvested;
                        this.simpleReturnPct = this.capitalInvested !== 0
                            ? (this.capitalGain / this.capitalInvested) * 100 : 0;
                        this.annualizedReturnPct = Number(cp.annualized_return_pct) || 0;
                        this.hasCapitalData = true;
                    } else {
                        this.capitalInvested = 0; this.capitalGain = 0;
                        this.simpleReturnPct = 0; this.annualizedReturnPct = 0;
                        this.hasCapitalData = false;
                    }
                } else {
                    this.holdings = [];
                    this.portfolioGain = 0;
                    this.portfolioGainPct = 0;
                    this.portfolioCost = 0;
                    this.capitalInvested = 0; this.capitalGain = 0;
                    this.simpleReturnPct = 0; this.annualizedReturnPct = 0;
                    this.hasCapitalData = false;
                }
                // Fetch indicators (non-blocking, non-fatal)
                vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/indicators')
                    .then(async res => {
                        if (res.ok) {
                            const ind = await res.json();
                            this.trend = ind.trend || '';
                            this.rsiSignal = ind.rsi_signal || '';
                            this.dataPoints = ind.data_points || 0;
                            this.hasIndicators = true;
                        }
                    }).catch(() => { this.hasIndicators = false; });
                // Fetch growth history (non-blocking, non-fatal)
                this.fetchGrowthData();
            } catch (e) {
                debugError('portfolioDashboard', 'loadPortfolio failed', e);
            }
        },

        async fetchGrowthData() {
            try {
                const res = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/history');
                if (res.ok) {
                    const data = await res.json();
                    const points = data.data_points || [];
                    this.growthData = this.filterAnomalies(points);
                    this.hasGrowthData = this.growthData.length > 0;
                    if (this.hasGrowthData) {
                        this.$nextTick(() => this.renderChart());
                    }
                } else {
                    this.growthData = [];
                    this.hasGrowthData = false;
                }
            } catch (e) {
                debugLog('portfolioDashboard', 'growth data fetch failed', e);
                this.growthData = [];
                this.hasGrowthData = false;
            }
        },

        filterAnomalies(points) {
            if (!points || points.length === 0) return [];
            const filtered = [];
            for (let i = 0; i < points.length; i++) {
                const p = Object.assign({}, points[i]);
                if (i > 0 && filtered.length > 0) {
                    const prev = filtered[filtered.length - 1];
                    if (prev.TotalValue > 0) {
                        const change = Math.abs(p.TotalValue - prev.TotalValue) / prev.TotalValue;
                        if (change > 0.5) {
                            p.TotalValue = prev.TotalValue;
                        }
                    }
                }
                filtered.push(p);
            }
            return filtered;
        },

        renderChart() {
            if (this.chartInstance) {
                this.chartInstance.destroy();
                this.chartInstance = null;
            }
            const canvas = document.getElementById('growthChart');
            if (!canvas || typeof Chart === 'undefined') return;

            const labels = this.growthData.map(p => {
                const d = new Date(p.Date);
                return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
            });
            const totalValues = this.growthData.map(p => p.TotalValue);
            const totalCosts = this.growthData.map(p => p.TotalCost);
            const capitalLine = this.growthData.map(() => this.capitalInvested);

            this.chartInstance = new Chart(canvas, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Portfolio Value',
                            data: totalValues,
                            borderColor: '#000',
                            borderWidth: 2,
                            borderDash: [],
                            pointRadius: 0,
                            pointHoverRadius: 4,
                            fill: false,
                            tension: 0,
                        },
                        {
                            label: 'Cost Basis',
                            data: totalCosts,
                            borderColor: '#888',
                            borderWidth: 1,
                            borderDash: [6, 3],
                            pointRadius: 0,
                            pointHoverRadius: 4,
                            fill: false,
                            tension: 0,
                        },
                        {
                            label: 'Capital Deployed',
                            data: capitalLine,
                            borderColor: '#000',
                            borderWidth: 1,
                            borderDash: [2, 2],
                            pointRadius: 0,
                            pointHoverRadius: 0,
                            fill: false,
                            tension: 0,
                        },
                    ],
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: {
                        mode: 'index',
                        intersect: false,
                    },
                    plugins: {
                        legend: {
                            display: false,
                        },
                        tooltip: {
                            backgroundColor: '#fff',
                            titleColor: '#000',
                            bodyColor: '#000',
                            borderColor: '#000',
                            borderWidth: 1,
                            titleFont: { family: "'IBM Plex Mono', monospace", size: 11 },
                            bodyFont: { family: "'IBM Plex Mono', monospace", size: 11 },
                            callbacks: {
                                label: function(ctx) {
                                    const val = Number(ctx.raw).toLocaleString('en-AU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
                                    return ctx.dataset.label + ': $' + val;
                                },
                            },
                        },
                    },
                    scales: {
                        x: {
                            grid: { display: false },
                            ticks: {
                                font: { family: "'IBM Plex Mono', monospace", size: 10 },
                                color: '#888',
                                maxTicksLimit: 10,
                            },
                            border: { color: '#000' },
                        },
                        y: {
                            grid: { color: '#eee' },
                            ticks: {
                                font: { family: "'IBM Plex Mono', monospace", size: 10 },
                                color: '#888',
                                callback: function(val) {
                                    return '$' + Number(val).toLocaleString('en-AU', { maximumFractionDigits: 0 });
                                },
                            },
                            border: { display: false },
                        },
                    },
                },
            });
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

        async refreshPortfolio() {
            if (this.refreshing || !this.selected) return;
            this.refreshing = true;
            try {
                vireStore.invalidate('/api/portfolios');
                const res = await fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '?force_refresh=true');
                if (res.ok) {
                    const data = await res.json();
                    this.holdings = vireStore.dedup(data.holdings || [], 'ticker');
                    this.portfolioGain = Number(data.total_net_return) || 0;
                    this.portfolioGainPct = Number(data.total_net_return_pct) || 0;
                    this.portfolioCost = Number(data.total_cost) || 0;
                    // Re-parse capital performance
                    const cp = data.capital_performance;
                    if (cp && cp.transaction_count > 0) {
                        this.capitalInvested = Number(cp.net_capital_deployed) || 0;
                        // Derive capital gain from actual holdings total, not server's current_portfolio_value
                        this.capitalGain = this.totalValue - this.capitalInvested;
                        this.simpleReturnPct = this.capitalInvested !== 0
                            ? (this.capitalGain / this.capitalInvested) * 100 : 0;
                        this.annualizedReturnPct = Number(cp.annualized_return_pct) || 0;
                        this.hasCapitalData = true;
                    } else {
                        this.capitalInvested = 0; this.capitalGain = 0;
                        this.simpleReturnPct = 0; this.annualizedReturnPct = 0;
                        this.hasCapitalData = false;
                    }
                }
                // Re-fetch indicators
                vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/indicators')
                    .then(async res => {
                        if (res.ok) {
                            const ind = await res.json();
                            this.trend = ind.trend || '';
                            this.rsiSignal = ind.rsi_signal || '';
                            this.dataPoints = ind.data_points || 0;
                            this.hasIndicators = true;
                        }
                    }).catch(() => { this.hasIndicators = false; });
                // Re-fetch growth data
                this.fetchGrowthData();
                window.dispatchEvent(new CustomEvent('toast', { detail: { msg: 'Portfolio refreshed' } }));
            } catch (e) {
                debugError('portfolioDashboard', 'refreshPortfolio failed', e);
            } finally {
                this.refreshing = false;
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

// Cash Transactions component
function cashTransactions() {
    return {
        portfolios: [],
        selected: '',
        defaultPortfolio: '',
        transactions: [],
        totalDeposits: 0,
        totalWithdrawals: 0,
        netCashFlow: 0,
        loading: true,
        error: '',
        currentPage: 1,
        pageSize: 100,
        get isDefault() { return this.selected === this.defaultPortfolio; },
        get hasTransactions() { return this.transactions.length > 0; },
        get totalPages() { return Math.max(1, Math.ceil(this.transactions.length / this.pageSize)); },
        get pagedTransactions() {
            const start = (this.currentPage - 1) * this.pageSize;
            return this.transactions.slice(start, start + this.pageSize);
        },
        gainClass(val) {
            if (val == null || val === 0) return '';
            return val > 0 ? 'gain-positive' : 'gain-negative';
        },
        txnClass(type) {
            const credits = ['deposit', 'contribution', 'transfer_in', 'dividend'];
            return credits.includes(type) ? 'gain-positive' : 'gain-negative';
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
                if (this.selected) await this.loadTransactions();
            } catch (e) {
                debugError('cashTransactions', 'init failed', e);
                this.error = 'Failed to connect to server';
            } finally {
                this.loading = false;
            }
        },

        async loadTransactions() {
            if (!this.selected) return;
            this.currentPage = 1;
            try {
                const res = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/cash-transactions');
                if (res.ok) {
                    const data = await res.json();
                    this.transactions = data.transactions || [];
                    const summary = data.summary || {};
                    this.totalDeposits = Number(summary.total_deposits) || 0;
                    this.totalWithdrawals = Number(summary.total_withdrawals) || 0;
                    this.netCashFlow = Number(summary.net_cash_flow) || 0;
                } else {
                    this.transactions = [];
                    this.totalDeposits = 0;
                    this.totalWithdrawals = 0;
                    this.netCashFlow = 0;
                }
            } catch (e) {
                debugError('cashTransactions', 'loadTransactions failed', e);
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
                debugError('cashTransactions', 'toggleDefault failed', e);
            }
        },

        prevPage() {
            if (this.currentPage > 1) this.currentPage--;
        },
        nextPage() {
            if (this.currentPage < this.totalPages) this.currentPage++;
        },

        fmt(val) {
            return val != null ? Number(val).toLocaleString('en-AU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '-';
        },
        formatDate(dateStr) {
            if (!dateStr) return '-';
            const d = new Date(dateStr);
            if (isNaN(d.getTime())) return dateStr;
            return d.toLocaleDateString('en-AU', { year: 'numeric', month: 'short', day: 'numeric' });
        },
    };
}

// Portfolio Strategy component
function portfolioStrategy() {
    return {
        portfolios: [],
        selected: '',
        defaultPortfolio: '',
        strategy: '',
        plan: '',
        loading: true,
        error: '',
        get isDefault() { return this.selected === this.defaultPortfolio; },

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
                debugError('portfolioStrategy', 'init failed', e);
                this.error = 'Failed to connect to server';
            } finally {
                this.loading = false;
            }
        },

        async loadPortfolio() {
            if (!this.selected) return;
            try {
                const [strategyRes, planRes] = await Promise.all([
                    vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/strategy'),
                    vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/plan'),
                ]);

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
                debugError('portfolioStrategy', 'loadPortfolio failed', e);
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
                debugError('portfolioStrategy', 'toggleDefault failed', e);
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
                debugError('portfolioStrategy', 'saveStrategy failed', e);
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
                debugError('portfolioStrategy', 'savePlan failed', e);
            }
        },
    };
}
