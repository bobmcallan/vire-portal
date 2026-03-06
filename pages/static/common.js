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
        closedHoldings: null,
        closedLoading: false,
        portfolioTotalValue: 0,
        portfolioGain: 0,
        portfolioGainPct: 0,
        portfolioCost: 0,
        equityValue: 0,
        capitalInvested: 0,
        hasCapitalData: false,
        grossCashBalance: 0,
        availableCash: 0,
        grossContributions: 0,
        totalDividends: 0,
        ledgerDividendReturn: 0,
        lastSynced: '',
        changeDayPct: null,
        changeWeekPct: null,
        changeMonthPct: null,
        hasChanges: false,
        changeCashDayPct: null,
        changeCashWeekPct: null,
        changeCashMonthPct: null,
        hasCashChanges: false,
        changeReturnDayDollar: null,
        changeReturnWeekDollar: null,
        changeReturnMonthDollar: null,
        hasReturnDollarChanges: false,
        changeReturnDayPct: null,
        changeReturnWeekPct: null,
        changeReturnMonthPct: null,
        hasReturnPctChanges: false,
        breadth: null,
        hasBreadth: false,
        watchlist: [],
        glossary: {},
        refreshing: false,
        growthData: [],
        hasGrowthData: false,
        chartInstance: null,
        showChartBreakdown: false,
        loading: true,
        portfolioLoading: false,
        error: '',
        get isDefault() { return this.selected === this.defaultPortfolio; },
        get filteredHoldings() {
            let h = this.holdings.slice();
            if (this.showClosed && this.closedHoldings) {
                const openTickers = new Set(h.map(x => x.ticker));
                for (const ch of this.closedHoldings) {
                    if (!openTickers.has(ch.ticker)) h.push(ch);
                }
            }
            h.sort((a, b) => (a.ticker || '').localeCompare(b.ticker || ''));
            return h;
        },
        get totalValue() {
            return this.portfolioTotalValue;
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
                this.$watch('showClosed', (val) => {
                    if (val) this.fetchClosedHoldings();
                });
                this.$watch('showChartBreakdown', (val) => {
                    if (this.chartInstance) {
                        this.chartInstance.data.datasets[1].hidden = !val;
                        this.chartInstance.data.datasets[2].hidden = !val;
                        this.chartInstance.update();
                    }
                });
                // Fetch glossary for tooltips (non-blocking)
                vireStore.fetch('/api/glossary')
                    .then(async res => {
                        if (res.ok) {
                            const data = await res.json();
                            const map = {};
                            for (const cat of (data.categories || [])) {
                                for (const t of (cat.terms || [])) {
                                    map[t.term] = t.definition;
                                }
                            }
                            this.glossary = map;
                        }
                    }).catch(() => {});
            } catch (e) {
                debugError('portfolioDashboard', 'init failed', e);
                this.error = 'Failed to connect to server';
            } finally {
                this.loading = false;
            }
        },

        async loadPortfolio() {
            if (!this.selected) return;
            this.portfolioLoading = true;
            this.closedHoldings = null;
            try {
                const holdingsRes = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected));

                if (holdingsRes.ok) {
                    const holdingsData = await holdingsRes.json();
                    this.holdings = vireStore.dedup(holdingsData.holdings || [], 'ticker');
                    this.totalDividends = Number(holdingsData.income_dividends_forecast) || 0;
                    this.ledgerDividendReturn = Number(holdingsData.income_dividends_received) || 0;
                    // Parse last_synced (UTC → local time)
                    this.lastSynced = holdingsData.last_synced || '';
                    // Parse changes
                    const changes = holdingsData.changes;
                    if (changes) {
                        this.changeDayPct = changes.yesterday?.portfolio_value?.has_previous ? changes.yesterday.portfolio_value.pct_change : null;
                        this.changeWeekPct = changes.week?.portfolio_value?.has_previous ? changes.week.portfolio_value.pct_change : null;
                        this.changeMonthPct = changes.month?.portfolio_value?.has_previous ? changes.month.portfolio_value.pct_change : null;
                        this.hasChanges = this.changeDayPct !== null || this.changeWeekPct !== null || this.changeMonthPct !== null;
                        // Cash changes
                        this.changeCashDayPct = changes.yesterday?.capital_gross?.has_previous ? changes.yesterday.capital_gross.pct_change : null;
                        this.changeCashWeekPct = changes.week?.capital_gross?.has_previous ? changes.week.capital_gross.pct_change : null;
                        this.changeCashMonthPct = changes.month?.capital_gross?.has_previous ? changes.month.capital_gross.pct_change : null;
                        this.hasCashChanges = this.changeCashDayPct !== null || this.changeCashWeekPct !== null || this.changeCashMonthPct !== null;
                        // Equity value $ changes (raw_change = dollar movement)
                        this.changeReturnDayDollar = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.raw_change : null;
                        this.changeReturnWeekDollar = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.raw_change : null;
                        this.changeReturnMonthDollar = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.raw_change : null;
                        this.hasReturnDollarChanges = this.changeReturnDayDollar !== null || this.changeReturnWeekDollar !== null || this.changeReturnMonthDollar !== null;
                        // Equity value % changes (pct_change = percentage movement)
                        this.changeReturnDayPct = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.pct_change : null;
                        this.changeReturnWeekPct = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.pct_change : null;
                        this.changeReturnMonthPct = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.pct_change : null;
                        this.hasReturnPctChanges = this.changeReturnDayPct !== null || this.changeReturnWeekPct !== null || this.changeReturnMonthPct !== null;
                    } else {
                        this.changeDayPct = null;
                        this.changeWeekPct = null;
                        this.changeMonthPct = null;
                        this.hasChanges = false;
                        this.changeCashDayPct = null;
                        this.changeCashWeekPct = null;
                        this.changeCashMonthPct = null;
                        this.hasCashChanges = false;
                        this.changeReturnDayDollar = null;
                        this.changeReturnWeekDollar = null;
                        this.changeReturnMonthDollar = null;
                        this.hasReturnDollarChanges = false;
                        this.changeReturnDayPct = null;
                        this.changeReturnWeekPct = null;
                        this.changeReturnMonthPct = null;
                        this.hasReturnPctChanges = false;
                    }
                    this.portfolioTotalValue = Number(holdingsData.portfolio_value) || 0;
                    this.portfolioGain = Number(holdingsData.equity_holdings_return) || 0;
                    this.portfolioGainPct = Number(holdingsData.equity_holdings_return_pct) || 0;
                    this.portfolioCost = Number(holdingsData.equity_holdings_cost) || 0;
                    this.equityValue = Number(holdingsData.equity_holdings_value) || 0;
                    this.grossCashBalance = Number(holdingsData.capital_gross) || 0;
                    this.availableCash = Number(holdingsData.capital_available) || 0;
                    // Parse capital performance
                    const cp = holdingsData.capital_performance;
                    if (cp && cp.transaction_count > 0) {
                        this.capitalInvested = Number(cp.capital_contributions_net) || 0;
                        this.grossContributions = Number(cp.capital_contributions_gross) || 0;
                        this.hasCapitalData = true;
                    } else {
                        this.capitalInvested = 0;
                        this.grossContributions = 0;
                        this.hasCapitalData = false;
                    }
                    // Parse breadth (server or computed fallback)
                    this.breadth = holdingsData.breadth || this.computeBreadth();
                    this.hasBreadth = this.breadth !== null;
                } else {
                    this.holdings = [];
                    this.portfolioTotalValue = 0;
                    this.portfolioGain = 0;
                    this.portfolioGainPct = 0;
                    this.portfolioCost = 0;
                    this.equityValue = 0;
                    this.grossCashBalance = 0;
                    this.availableCash = 0;
                    this.grossContributions = 0;
                    this.totalDividends = 0;
                    this.ledgerDividendReturn = 0;
                    this.lastSynced = '';
                    this.changeDayPct = null;
                    this.changeWeekPct = null;
                    this.changeMonthPct = null;
                    this.hasChanges = false;
                    this.changeCashDayPct = null;
                    this.changeCashWeekPct = null;
                    this.changeCashMonthPct = null;
                    this.hasCashChanges = false;
                    this.changeReturnDayDollar = null;
                    this.changeReturnWeekDollar = null;
                    this.changeReturnMonthDollar = null;
                    this.hasReturnDollarChanges = false;
                    this.changeReturnDayPct = null;
                    this.changeReturnWeekPct = null;
                    this.changeReturnMonthPct = null;
                    this.hasReturnPctChanges = false;
                    this.capitalInvested = 0;
                    this.hasCapitalData = false;
                    this.breadth = null;
                    this.hasBreadth = false;
                }
                // Fetch growth history and watchlist (non-blocking, non-fatal)
                this.fetchGrowthData();
                this.fetchWatchlist();
            } catch (e) {
                debugError('portfolioDashboard', 'loadPortfolio failed', e);
            } finally {
                this.portfolioLoading = false;
            }
        },

        async fetchClosedHoldings() {
            if (!this.selected || this.closedHoldings !== null) return;
            this.closedLoading = true;
            try {
                const res = await fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '?include_closed=true');
                if (res.ok) {
                    const data = await res.json();
                    const all = vireStore.dedup(data.holdings || [], 'ticker');
                    this.closedHoldings = all.filter(x => x.holding_value_market === 0);
                } else {
                    this.closedHoldings = [];
                }
            } catch (e) {
                debugLog('portfolioDashboard', 'fetchClosedHoldings failed', e);
                this.closedHoldings = [];
            } finally {
                this.closedLoading = false;
            }
        },

        async fetchGrowthData(force) {
            try {
                const url = '/api/portfolios/' + encodeURIComponent(this.selected) + '/timeline' + (force ? '?force_refresh=true' : '');
                const res = await vireStore.fetch(url);
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

        async fetchWatchlist() {
            try {
                const res = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '/watchlist');
                if (res.ok) {
                    const data = await res.json();
                    this.watchlist = data.items || [];
                } else {
                    this.watchlist = [];
                }
            } catch (e) {
                debugLog('portfolioDashboard', 'watchlist fetch failed', e);
                this.watchlist = [];
            }
        },

        filterAnomalies(points) {
            if (!points || points.length === 0) return [];
            const filtered = [];
            for (let i = 0; i < points.length; i++) {
                const p = Object.assign({}, points[i]);
                if (i > 0 && filtered.length > 0) {
                    const prev = filtered[filtered.length - 1];
                    if (prev.portfolio_value > 0) {
                        const change = Math.abs(p.portfolio_value - prev.portfolio_value) / prev.portfolio_value;
                        if (change > 0.5) {
                            p.portfolio_value = prev.portfolio_value;
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
                const d = new Date(p.date);
                return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
            });
            const totalValues = this.growthData.map(p => p.portfolio_value || p.value || 0);
            const equityValues = this.growthData.map(p => p.equity_holdings_value || 0);
            const capitalLine = this.growthData.map(p => p.capital_contributions_net || this.capitalInvested || 0);

            // Compute rebalance markers (holding_count changes by 3+)
            const rebalanceAnnotations = {};
            for (let i = 1; i < this.growthData.length; i++) {
                const curr = this.growthData[i];
                const prev = this.growthData[i - 1];
                if (curr.holding_count != null && prev.holding_count != null) {
                    const delta = Math.abs(curr.holding_count - prev.holding_count);
                    if (delta >= 3) {
                        rebalanceAnnotations['rebal' + i] = {
                            type: 'line',
                            xMin: labels[i],
                            xMax: labels[i],
                            borderColor: '#ccc',
                            borderWidth: 1,
                            borderDash: [4, 4],
                            label: {
                                display: true,
                                content: 'Rebalance',
                                position: 'start',
                                font: { size: 9, family: "'IBM Plex Mono', monospace" },
                                color: '#888',
                                backgroundColor: 'transparent',
                            },
                        };
                    }
                }
            }

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
                            label: 'Equity Value',
                            data: equityValues,
                            borderColor: '#888',
                            borderWidth: 1,
                            borderDash: [6, 3],
                            pointRadius: 0,
                            pointHoverRadius: 4,
                            fill: false,
                            tension: 0,
                            hidden: !this.showChartBreakdown,
                        },
                        {
                            label: 'Net Deposited',
                            data: capitalLine,
                            borderColor: '#000',
                            borderWidth: 1,
                            borderDash: [2, 2],
                            pointRadius: 0,
                            pointHoverRadius: 4,
                            fill: false,
                            tension: 0,
                            hidden: !this.showChartBreakdown,
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
                        annotation: Object.keys(rebalanceAnnotations).length > 0 ? {
                            annotations: rebalanceAnnotations,
                        } : undefined,
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
            this.closedHoldings = null;
            try {
                vireStore.invalidate('/api/portfolios');
                const res = await fetch('/api/portfolios/' + encodeURIComponent(this.selected) + '?force_refresh=true');
                if (res.ok) {
                    const data = await res.json();
                    this.holdings = vireStore.dedup(data.holdings || [], 'ticker');
                    this.totalDividends = Number(data.income_dividends_forecast) || 0;
                    this.ledgerDividendReturn = Number(data.income_dividends_received) || 0;
                    this.portfolioTotalValue = Number(data.portfolio_value) || 0;
                    this.portfolioGain = Number(data.equity_holdings_return) || 0;
                    this.portfolioGainPct = Number(data.equity_holdings_return_pct) || 0;
                    this.portfolioCost = Number(data.equity_holdings_cost) || 0;
                    this.equityValue = Number(data.equity_holdings_value) || 0;
                    this.grossCashBalance = Number(data.capital_gross) || 0;
                    this.availableCash = Number(data.capital_available) || 0;
                    // Parse last_synced (UTC → local time)
                    this.lastSynced = data.last_synced || '';
                    // Parse changes
                    const changes = data.changes;
                    if (changes) {
                        this.changeDayPct = changes.yesterday?.portfolio_value?.has_previous ? changes.yesterday.portfolio_value.pct_change : null;
                        this.changeWeekPct = changes.week?.portfolio_value?.has_previous ? changes.week.portfolio_value.pct_change : null;
                        this.changeMonthPct = changes.month?.portfolio_value?.has_previous ? changes.month.portfolio_value.pct_change : null;
                        this.hasChanges = this.changeDayPct !== null || this.changeWeekPct !== null || this.changeMonthPct !== null;
                        // Cash changes
                        this.changeCashDayPct = changes.yesterday?.capital_gross?.has_previous ? changes.yesterday.capital_gross.pct_change : null;
                        this.changeCashWeekPct = changes.week?.capital_gross?.has_previous ? changes.week.capital_gross.pct_change : null;
                        this.changeCashMonthPct = changes.month?.capital_gross?.has_previous ? changes.month.capital_gross.pct_change : null;
                        this.hasCashChanges = this.changeCashDayPct !== null || this.changeCashWeekPct !== null || this.changeCashMonthPct !== null;
                        // Equity value $ changes (raw_change = dollar movement)
                        this.changeReturnDayDollar = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.raw_change : null;
                        this.changeReturnWeekDollar = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.raw_change : null;
                        this.changeReturnMonthDollar = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.raw_change : null;
                        this.hasReturnDollarChanges = this.changeReturnDayDollar !== null || this.changeReturnWeekDollar !== null || this.changeReturnMonthDollar !== null;
                        // Equity value % changes (pct_change = percentage movement)
                        this.changeReturnDayPct = changes.yesterday?.equity_holdings_value?.has_previous ? changes.yesterday.equity_holdings_value.pct_change : null;
                        this.changeReturnWeekPct = changes.week?.equity_holdings_value?.has_previous ? changes.week.equity_holdings_value.pct_change : null;
                        this.changeReturnMonthPct = changes.month?.equity_holdings_value?.has_previous ? changes.month.equity_holdings_value.pct_change : null;
                        this.hasReturnPctChanges = this.changeReturnDayPct !== null || this.changeReturnWeekPct !== null || this.changeReturnMonthPct !== null;
                    } else {
                        this.changeDayPct = null;
                        this.changeWeekPct = null;
                        this.changeMonthPct = null;
                        this.hasChanges = false;
                        this.changeCashDayPct = null;
                        this.changeCashWeekPct = null;
                        this.changeCashMonthPct = null;
                        this.hasCashChanges = false;
                        this.changeReturnDayDollar = null;
                        this.changeReturnWeekDollar = null;
                        this.changeReturnMonthDollar = null;
                        this.hasReturnDollarChanges = false;
                        this.changeReturnDayPct = null;
                        this.changeReturnWeekPct = null;
                        this.changeReturnMonthPct = null;
                        this.hasReturnPctChanges = false;
                    }
                    // Re-parse capital performance
                    const cp = data.capital_performance;
                    if (cp && cp.transaction_count > 0) {
                        this.capitalInvested = Number(cp.capital_contributions_net) || 0;
                        this.grossContributions = Number(cp.capital_contributions_gross) || 0;
                        this.hasCapitalData = true;
                    } else {
                        this.capitalInvested = 0;
                        this.grossContributions = 0;
                        this.hasCapitalData = false;
                    }
                    // Re-parse breadth (server or computed fallback)
                    this.breadth = data.breadth || this.computeBreadth();
                    this.hasBreadth = this.breadth !== null;
                }
                // Re-fetch growth data and watchlist with force refresh
                this.fetchGrowthData(true);
                this.fetchWatchlist();
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
        fmtSynced(utcStr) {
            if (!utcStr) return '';
            try {
                const d = new Date(utcStr);
                if (isNaN(d.getTime())) return '';
                return d.toLocaleString('en-AU', {
                    day: 'numeric', month: 'short', year: 'numeric',
                    hour: '2-digit', minute: '2-digit', hour12: false
                });
            } catch { return ''; }
        },
        fmtSyncedTime(utcStr) {
            if (!utcStr) return '';
            try {
                const d = new Date(utcStr);
                if (isNaN(d.getTime())) return '';
                return d.toLocaleTimeString('en-AU', { hour: '2-digit', minute: '2-digit', hour12: false });
            } catch { return ''; }
        },
        changePct(val) {
            if (val == null) return '-';
            return (val >= 0 ? '+' : '') + Number(val).toFixed(1) + '%';
        },
        changeDollar(val) {
            if (val == null) return '-';
            const sign = val >= 0 ? '+' : '';
            const abs = Math.abs(val);
            if (abs >= 1000000) return sign + (val / 1000000).toFixed(1) + 'M';
            if (abs >= 1000) return sign + (val / 1000).toFixed(1) + 'K';
            return sign + Number(val).toFixed(0);
        },
        changeClass(val) {
            if (val == null || val === 0) return 'change-neutral';
            return val > 0 ? 'change-up' : 'change-down';
        },
        trendArrow(score) {
            if (score == null) return '\u2192';
            if (score > 0.1) return '\u2191';
            if (score < -0.1) return '\u2193';
            return '\u2192';
        },
        trendArrowClass(score) {
            if (score == null) return 'change-neutral';
            if (score > 0.1) return 'change-up';
            if (score < -0.1) return 'change-down';
            return 'change-neutral';
        },
        holdingTodayChange(h) {
            if (h.current_price == null || h.yesterday_close_price == null || h.units == null) return null;
            return (h.current_price - h.yesterday_close_price) * h.units;
        },
        fmtTodayChange(val) {
            if (val == null) return '';
            const sign = val >= 0 ? '+' : '-';
            const abs = Math.abs(val);
            if (abs >= 1000000) return sign + '$' + (abs / 1000000).toFixed(1) + 'M';
            if (abs >= 1000) return sign + '$' + (abs / 1000).toFixed(1) + 'K';
            return sign + '$' + abs.toFixed(0);
        },
        computeBreadth() {
            const active = this.holdings.filter(h => h.holding_value_market > 0);
            if (active.length === 0) return null;
            let rising = 0, flat = 0, falling = 0;
            let risingWeight = 0, flatWeight = 0, fallingWeight = 0;
            let totalWeight = 0;
            let weightedScore = 0;
            let todayChange = 0;
            let todayValid = false;
            for (const h of active) {
                const w = h.holding_value_market || 0;
                const score = h.trend_score || 0;
                totalWeight += w;
                weightedScore += score * w;
                if (score > 0.1) { rising++; risingWeight += w; }
                else if (score < -0.1) { falling++; fallingWeight += w; }
                else { flat++; flatWeight += w; }
                const tc = this.holdingTodayChange(h);
                if (tc !== null) { todayChange += tc; todayValid = true; }
            }
            const avg = totalWeight > 0 ? weightedScore / totalWeight : 0;
            let trend_label;
            if (avg > 0.3) trend_label = 'Uptrend';
            else if (avg > 0.1) trend_label = 'Mixed-Up';
            else if (avg > -0.1) trend_label = 'Mixed';
            else if (avg > -0.3) trend_label = 'Mixed-Down';
            else trend_label = 'Downtrend';
            return {
                rising_count: rising,
                flat_count: flat,
                falling_count: falling,
                rising_weight_pct: totalWeight > 0 ? (risingWeight / totalWeight) * 100 : 0,
                flat_weight_pct: totalWeight > 0 ? (flatWeight / totalWeight) * 100 : 0,
                falling_weight_pct: totalWeight > 0 ? (fallingWeight / totalWeight) * 100 : 0,
                trend_label: trend_label,
                trend_score: avg,
                today_change: todayValid ? todayChange : null,
            };
        },
        glossaryDef(term) {
            return this.glossary[term] || '';
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
        accounts: [],
        totalCash: 0,
        transactionCount: 0,
        byCategory: {},
        loading: true,
        error: '',
        currentPage: 1,
        pageSize: 100,
        get isDefault() { return this.selected === this.defaultPortfolio; },
        get hasTransactions() { return this.transactions.length > 0; },
        get hasAccounts() { return this.accounts.length > 0; },
        get nonZeroCategories() {
            return Object.entries(this.byCategory).filter(([, v]) => v !== 0);
        },
        get hasCategoryBreakdown() { return this.nonZeroCategories.length > 0; },
        get totalPages() { return Math.max(1, Math.ceil(this.transactions.length / this.pageSize)); },
        get pagedTransactions() {
            const start = (this.currentPage - 1) * this.pageSize;
            return this.transactions.slice(start, start + this.pageSize);
        },
        gainClass(val) {
            if (val == null || val === 0) return '';
            return val > 0 ? 'gain-positive' : 'gain-negative';
        },
        txnClass(amount) {
            return Number(amount) >= 0 ? 'gain-positive' : 'gain-negative';
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
                    const txns = data.transactions || [];
                    txns.sort((a, b) => new Date(b.date) - new Date(a.date));
                    this.transactions = txns;
                    this.accounts = data.accounts || [];
                    const summary = data.summary || {};
                    this.totalCash = summary.capital_gross || 0;
                    this.transactionCount = summary.transaction_count || 0;
                    this.byCategory = summary.net_cash_by_category || {};
                } else {
                    this.transactions = [];
                    this.accounts = [];
                    this.totalCash = 0;
                    this.transactionCount = 0;
                    this.byCategory = {};
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
