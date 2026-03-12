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

    // Status Indicators (WebSocket)
    Alpine.data('statusIndicators', () => ({
        portal: 'startup',
        server: 'startup',
        _ws: null,
        _timer: null,
        _backoff: 1000,
        _maxBackoff: 30000,
        init() {
            this._connect();
        },
        destroy() {
            if (this._timer) clearInterval(this._timer);
            if (this._ws) this._ws.close();
        },
        _connect() {
            const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const url = `${proto}//${location.host}/ws/status`;
            try {
                this._ws = new WebSocket(url);
            } catch {
                this.portal = 'down';
                this.server = 'down';
                this._scheduleReconnect();
                return;
            }
            this._ws.onopen = () => {
                this._backoff = 1000;
                this._ping();
                if (this._timer) clearInterval(this._timer);
                this._timer = setInterval(() => this._ping(), 5000);
            };
            this._ws.onmessage = (e) => {
                try {
                    const d = JSON.parse(e.data);
                    this.portal = d.portal || 'down';
                    this.server = d.server || 'down';
                } catch { /* ignore malformed */ }
            };
            this._ws.onclose = () => {
                this.portal = 'down';
                this.server = 'down';
                if (this._timer) { clearInterval(this._timer); this._timer = null; }
                this._scheduleReconnect();
            };
            this._ws.onerror = () => {}; // onclose fires after onerror
        },
        _ping() {
            if (this._ws && this._ws.readyState === WebSocket.OPEN) {
                this._ws.send('ping');
            }
        },
        _scheduleReconnect() {
            const jitter = Math.random() * 500;
            setTimeout(() => this._connect(), this._backoff + jitter);
            this._backoff = Math.min(this._backoff * 2, this._maxBackoff);
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
        portfolioCurrency: '',
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
        currencyGainLoss: null,
        currencyGainLossPct: null,
        hasCurrencyData: false,
        breadth: null,
        hasBreadth: false,
        stockTrend: null,
        hasStockTrend: false,
        watchlist: [],
        glossary: {},
        complianceReport: null,
        complianceExpanded: false,
        complianceRunning: false,
        bugFindingsExpanded: false,
        userRole: '',
        refreshing: false,
        growthData: [],
        hasGrowthData: false,
        chartInstance: null,
        showChartBreakdown: false,
        showMA20: false,
        showMA50: false,
        showMA200: false,
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
        get currencyShort() {
            const c = this.portfolioCurrency;
            return c ? '(' + c + ')' : '';
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

        get complianceState() {
            if (!this.complianceReport) return 'never';
            if (this.complianceReport.dirty) return 'dirty';
            const s = this.complianceReport.summary;
            if (s && (s.breach_count > 0 || s.warning_count > 0)) return 'issues';
            return 'clean';
        },
        get complianceStatusText() {
            const r = this.complianceReport;
            if (!r) return 'No review yet';
            const time = this._fmtComplianceTime(r.generated_at);
            if (r.dirty) return 'Prices moved since last review';
            const s = r.summary;
            if (!s || (s.breach_count === 0 && s.warning_count === 0)) return 'Reviewed ' + time + ' \u2014 no issues';
            const parts = [];
            if (s.breach_count > 0) parts.push(s.breach_count + ' breach' + (s.breach_count > 1 ? 'es' : ''));
            if (s.warning_count > 0) parts.push(s.warning_count + ' warning' + (s.warning_count > 1 ? 's' : ''));
            return 'Reviewed ' + time + ' \u2014 ' + parts.join(', ');
        },
        get complianceHeaderClass() {
            return 'compliance-state-' + this.complianceState;
        },
        get complianceShowRun() {
            return this.complianceState === 'never' || this.complianceState === 'dirty';
        },
        get complianceHasFindings() {
            return this.complianceFindings.length > 0;
        },
        get complianceFindings() {
            if (!this.complianceReport || !this.complianceReport.findings) return [];
            return this.complianceReport.findings.filter(f => f.severity !== 'BUG');
        },
        get complianceBugFindings() {
            if (!this.complianceReport || !this.complianceReport.findings) return [];
            return this.complianceReport.findings.filter(f => f.severity === 'BUG');
        },
        get complianceHasBugs() {
            return this.complianceBugFindings.length > 0;
        },
        get isAdmin() {
            return this.userRole === 'admin';
        },
        get complianceDisclaimer() {
            return this.complianceReport?.disclaimer || '';
        },

        _getPortfolioFromURL() {
            const path = window.location.pathname;
            if (path.startsWith('/dashboard/')) {
                return decodeURIComponent(path.substring('/dashboard/'.length));
            }
            if (path.startsWith('/m/')) {
                return decodeURIComponent(path.substring('/m/'.length));
            }
            return '';
        },

        _updateURL() {
            if (this.selected) {
                const base = window.location.pathname.startsWith('/m') ? '/m/' : '/dashboard/';
                const newPath = base + encodeURIComponent(this.selected);
                const params = new URLSearchParams(window.location.search);
                if (this.showClosed) params.set('closed', '1');
                else params.delete('closed');
                const qs = params.toString();
                const full = newPath + (qs ? '?' + qs : '');
                if (window.location.pathname + window.location.search !== full) {
                    history.replaceState(null, '', full);
                }
            }
        },

        _applyPortfolioData(holdingsData) {
            this.holdings = vireStore.dedup(holdingsData.holdings || [], 'ticker');
            this.totalDividends = Number(holdingsData.income_dividends_forecast) || 0;
            this.ledgerDividendReturn = Number(holdingsData.income_dividends_received) || 0;
            this.lastSynced = holdingsData.last_synced || '';
            // 1D/1W/1M changes — flat fields from server
            this.changeDayPct = holdingsData.change_day_pct ?? null;
            this.hasChanges = this.changeDayPct != null;
            this.changeReturnDayDollar = holdingsData.change_day_dollar ?? null;
            this.changeReturnWeekDollar = holdingsData.change_week_dollar ?? null;
            this.changeReturnMonthDollar = holdingsData.change_month_dollar ?? null;
            this.hasReturnDollarChanges = this.changeReturnDayDollar != null || this.changeReturnWeekDollar != null || this.changeReturnMonthDollar != null;
            this.changeReturnDayPct = holdingsData.change_day_pct ?? null;
            this.changeReturnWeekPct = holdingsData.change_week_pct ?? null;
            this.changeReturnMonthPct = holdingsData.change_month_pct ?? null;
            this.hasReturnPctChanges = this.changeReturnDayPct != null || this.changeReturnWeekPct != null || this.changeReturnMonthPct != null;
            // Cash changes — simple field reads
            const changes = holdingsData.changes;
            if (changes) {
                this.changeCashDayPct = changes.yesterday?.capital_gross?.has_previous ? changes.yesterday.capital_gross.pct_change : null;
                this.changeCashWeekPct = changes.week?.capital_gross?.has_previous ? changes.week.capital_gross.pct_change : null;
                this.changeCashMonthPct = changes.month?.capital_gross?.has_previous ? changes.month.capital_gross.pct_change : null;
                this.hasCashChanges = this.changeCashDayPct !== null || this.changeCashWeekPct !== null || this.changeCashMonthPct !== null;
            } else {
                this.changeCashDayPct = null; this.changeCashWeekPct = null; this.changeCashMonthPct = null; this.hasCashChanges = false;
            }
            this.portfolioTotalValue = Number(holdingsData.portfolio_value) || 0;
            this.portfolioCurrency = holdingsData.currency || 'AUD';
            this.portfolioGain = Number(holdingsData.equity_holdings_return) || 0;
            this.portfolioGainPct = Number(holdingsData.equity_holdings_return_pct) || 0;
            this.portfolioCost = Number(holdingsData.equity_holdings_cost) || 0;
            this.equityValue = Number(holdingsData.equity_holdings_value) || 0;
            this.grossCashBalance = Number(holdingsData.capital_gross) || 0;
            this.availableCash = Number(holdingsData.capital_available) || 0;
            const cp = holdingsData.capital_performance;
            if (cp && cp.transaction_count > 0) {
                this.capitalInvested = Number(cp.capital_contributions_net) || 0;
                this.grossContributions = Number(cp.capital_contributions_gross) || 0;
                this.hasCapitalData = true;
            } else {
                this.capitalInvested = 0; this.grossContributions = 0; this.hasCapitalData = false;
            }
            this.currencyGainLoss = holdingsData.currency_gain_loss != null ? Number(holdingsData.currency_gain_loss) : null;
            this.currencyGainLossPct = holdingsData.currency_gain_loss_pct != null ? Number(holdingsData.currency_gain_loss_pct) : null;
            this.hasCurrencyData = this.currencyGainLoss != null;
            // Use server-computed breadth (live prices, exchange-aware)
            const serverBreadth = holdingsData.breadth;
            if (serverBreadth) {
                const segs = serverBreadth.segments || [];
                // Derive counts from segments (single source of truth)
                const risingCount = segs.filter(s => s.status === 'rising').length;
                const flatCount = segs.filter(s => s.status === 'flat').length;
                const fallingCount = segs.filter(s => s.status === 'falling').length;
                // Normalize weight_pct so segments fill 100% of the bar
                const totalWeight = segs.reduce((sum, s) => sum + (s.weight_pct || 0), 0);
                const normalizedSegs = totalWeight > 0
                    ? segs.map(s => ({ ...s, bar_pct: ((s.weight_pct || 0) / totalWeight) * 100 }))
                    : segs.map(s => ({ ...s, bar_pct: 0 }));
                this.breadth = {
                    rising_count: risingCount,
                    flat_count: flatCount,
                    falling_count: fallingCount,
                    trend_label: serverBreadth.price_trend_label || '',
                    trend_score: serverBreadth.price_trend_score || 0,
                    today_change: serverBreadth.today_change,
                    today_change_pct: serverBreadth.today_change_pct,
                    yesterday_change: serverBreadth.yesterday_change,
                    segments: normalizedSegs,
                };
                this.hasBreadth = true;
            } else {
                this.breadth = null;
                this.hasBreadth = false;
            }
            // Build stock trend bar from per-holding price_trend_score
            const openHoldings = (holdingsData.holdings || []).filter(h => h.holding_value_market > 0);
            if (openHoldings.length > 0) {
                const trendSegs = openHoldings.map(h => {
                    const score = h.price_trend_score;
                    let status = 'consolidating';
                    if (score > 0.1) status = 'uptrend';
                    else if (score < -0.1) status = 'downtrend';
                    return { ticker: h.ticker, name: h.name, status, weight_pct: h.holding_weight_pct || 0, label: h.price_trend_label || '', score: score || 0 };
                });
                const trendTotal = trendSegs.reduce((sum, s) => sum + s.weight_pct, 0);
                const trendOrder = { downtrend: 0, consolidating: 1, uptrend: 2 };
                const normTrend = (trendTotal > 0
                    ? trendSegs.map(s => ({ ...s, bar_pct: (s.weight_pct / trendTotal) * 100 }))
                    : trendSegs.map(s => ({ ...s, bar_pct: 0 }))
                ).sort((a, b) => (trendOrder[a.status] ?? 1) - (trendOrder[b.status] ?? 1));
                this.stockTrend = {
                    uptrend_count: trendSegs.filter(s => s.status === 'uptrend').length,
                    consolidating_count: trendSegs.filter(s => s.status === 'consolidating').length,
                    downtrend_count: trendSegs.filter(s => s.status === 'downtrend').length,
                    segments: normTrend,
                };
                this.hasStockTrend = true;
            } else {
                this.stockTrend = null;
                this.hasStockTrend = false;
            }
        },

        _applyTimelineData(timelineData) {
            const points = timelineData.data_points || [];
            this.growthData = this.filterAnomalies(points);
            this.hasGrowthData = this.growthData.length > 0;
        },

        async init() {
            try {
                const initStart = performance.now();
                const ssrData = window.__VIRE_DATA__;
                if (ssrData && ssrData.userRole) {
                    this.userRole = ssrData.userRole;
                }
                if (ssrData && ssrData.portfolios) {
                    // --- SSR path: hydrate from server-embedded data ---
                    const data = ssrData.portfolios;
                    this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
                    this.defaultPortfolio = data.default || '';
                    // Use server-selected portfolio (from URL path or default)
                    if (ssrData.selectedPortfolio) {
                        this.selected = ssrData.selectedPortfolio;
                    } else if (this.defaultPortfolio) {
                        this.selected = this.defaultPortfolio;
                    } else if (this.portfolios.length > 0) {
                        this.selected = this.portfolios[0].name;
                    }

                    if (ssrData.portfolio) {
                        this._applyPortfolioData(ssrData.portfolio);
                        console.log('[dashboard] SSR portfolio: holdings=' + this.holdings.length + ' breadth=' + this.hasBreadth + ' value=' + this.portfolioTotalValue);
                    } else {
                        console.warn('[dashboard] SSR portfolio data is null — server fetch may have failed');
                    }
                    if (ssrData.timeline) {
                        this._applyTimelineData(ssrData.timeline);
                    }
                    if (ssrData.watchlist) {
                        this.watchlist = ssrData.watchlist.items || [];
                    }
                    if (ssrData.compliance) {
                        this._applyComplianceData(ssrData.compliance);
                    }
                    if (ssrData.glossary) {
                        const map = {};
                        for (const cat of (ssrData.glossary.categories || [])) {
                            for (const t of (cat.terms || [])) {
                                map[t.term] = t.definition;
                            }
                        }
                        this.glossary = map;
                    }

                    window.__VIRE_DATA__ = null;
                    this.loading = false;
                    // Restore showClosed from URL param
                    if (new URLSearchParams(window.location.search).get('closed') === '1') {
                        this.showClosed = true;
                        this.fetchClosedHoldings();
                    }
                    this._updateURL();
                    console.log(`[dashboard] SSR hydration: ${(performance.now() - initStart).toFixed(0)}ms`);

                    // Auto-detect and store browser timezone
                    try {
                        const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
                        if (tz && document.cookie.includes('vire_session')) {
                            const storedTz = sessionStorage.getItem('vire_tz_sent');
                            if (storedTz !== tz) {
                                fetch('/profile', {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                                    body: '_csrf=' + encodeURIComponent(this._getCsrf()) + '&timezone=' + encodeURIComponent(tz)
                                }).then(() => sessionStorage.setItem('vire_tz_sent', tz)).catch(() => {});
                            }
                        }
                    } catch(e) {}

                    // Set up watchers (same as client-side path)
                    this.$watch('showClosed', (val) => {
                        if (val) this.fetchClosedHoldings();
                        this._updateURL();
                    });
                    this.$watch('showChartBreakdown', () => this.renderChart());
                    this.$watch('showMA20', () => this.renderChart());
                    this.$watch('showMA50', () => this.renderChart());
                    this.$watch('showMA200', () => this.renderChart());

                    if (this.hasGrowthData) {
                        this.$nextTick(() => this.renderChart());
                    }
                    return;
                }

                // --- Client-side fallback path ---
                console.log('[dashboard] SSR data not available, using client-side fetch');
                const fetchStart = performance.now();
                const res = await vireStore.fetch('/api/portfolios');
                console.log(`[dashboard] fetch /api/portfolios: ${(performance.now() - fetchStart).toFixed(0)}ms`);
                if (!res.ok) {
                    this.error = 'Failed to load portfolios';
                    this.loading = false;
                    return;
                }
                const data = await res.json();
                this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
                this.defaultPortfolio = data.default || '';
                // Check URL path for portfolio selection
                const urlPortfolio = this._getPortfolioFromURL();
                if (urlPortfolio && this.portfolios.some(p => p.name === urlPortfolio)) {
                    this.selected = urlPortfolio;
                } else if (this.defaultPortfolio) {
                    this.selected = this.defaultPortfolio;
                } else if (this.portfolios.length > 0) {
                    this.selected = this.portfolios[0].name;
                }
                if (this.selected) await this.loadPortfolio();
                // Restore showClosed from URL param
                if (new URLSearchParams(window.location.search).get('closed') === '1') {
                    this.showClosed = true;
                    this.fetchClosedHoldings();
                }
                this.$watch('showClosed', (val) => {
                    if (val) this.fetchClosedHoldings();
                    this._updateURL();
                });
                this.$watch('showChartBreakdown', () => this.renderChart());
                this.$watch('showMA20', () => this.renderChart());
                this.$watch('showMA50', () => this.renderChart());
                this.$watch('showMA200', () => this.renderChart());
                console.log(`[dashboard] client-side init (excl. glossary/growth/watchlist): ${(performance.now() - initStart).toFixed(0)}ms`);
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
            this._updateURL();
            this.portfolioLoading = true;
            this.closedHoldings = null;
            try {
                const lpStart = performance.now();
                const holdingsRes = await vireStore.fetch('/api/portfolios/' + encodeURIComponent(this.selected));

                if (holdingsRes.ok) {
                    const holdingsData = await holdingsRes.json();
                    this._applyPortfolioData(holdingsData);
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
                    this.stockTrend = null;
                    this.hasStockTrend = false;
                }
                console.log(`[dashboard] fetch /api/portfolios/${this.selected}: ${(performance.now() - lpStart).toFixed(0)}ms`);
                // Fetch growth history, watchlist, and compliance (non-blocking, non-fatal)
                this.fetchGrowthData();
                this.fetchWatchlist();
                this.fetchCompliance();
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

        _getCsrf() {
            const c = document.cookie.split('; ').find(c => c.startsWith('_csrf='));
            return c ? c.split('=')[1] : '';
        },

        _fmtComplianceTime(isoStr) {
            if (!isoStr) return '';
            const d = new Date(isoStr);
            if (isNaN(d.getTime())) return '';
            return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        },

        _applyComplianceData(data) {
            this.complianceReport = data?.report || null;
        },

        async fetchCompliance() {
            try {
                const res = await vireStore.fetch('/api/compliance/latest?portfolio_name=' + encodeURIComponent(this.selected));
                if (res.ok) {
                    const data = await res.json();
                    this._applyComplianceData(data);
                }
            } catch (e) {
                console.warn('[dashboard] compliance fetch failed:', e);
            }
        },

        async runComplianceReview() {
            this.complianceRunning = true;
            try {
                const res = await vireStore.fetch('/api/compliance/run', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ portfolio_name: this.selected, force_refresh: true })
                });
                if (res.ok) {
                    const data = await res.json();
                    this._applyComplianceData(data);
                    this.complianceExpanded = true;
                }
            } catch (e) {
                console.warn('[dashboard] compliance run failed:', e);
            } finally {
                this.complianceRunning = false;
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

        computeMA(values, period) {
            const result = [];
            for (let i = 0; i < values.length; i++) {
                if (i < period - 1) { result.push(null); continue; }
                let sum = 0;
                for (let j = i - period + 1; j <= i; j++) sum += values[j];
                result.push(sum / period);
            }
            return result;
        },

        renderChart() {
            if (this.chartInstance) {
                this.chartInstance.destroy();
                this.chartInstance = null;
            }
            const canvas = document.getElementById('growthChart');
            const scrollEl = document.getElementById('growthChartScroll');
            const sizerEl = document.getElementById('growthChartSizer');
            if (!canvas || !scrollEl || !sizerEl || typeof Chart === 'undefined') return;

            if (this.growthData.length === 0) return;

            // Filter to last 6 months from today
            const sixMonthsAgo = new Date();
            sixMonthsAgo.setMonth(sixMonthsAgo.getMonth() - 6);
            const chartData = this.growthData.filter(p => new Date(p.date) >= sixMonthsAgo);
            const totalPoints = chartData.length;
            if (totalPoints === 0) return;

            // Size canvas to fill container (no horizontal scrolling for 6-month view)
            const containerWidth = scrollEl.clientWidth;
            sizerEl.style.width = containerWidth + 'px';

            // Build labels: month boundaries only, year at year change
            const labels = chartData.map((p, i) => {
                const d = new Date(p.date);
                const prev = i > 0 ? new Date(chartData[i - 1].date) : null;
                if (prev && d.getFullYear() !== prev.getFullYear()) {
                    return d.toLocaleDateString('en-US', { month: 'short' }) + ' ' + d.getFullYear();
                }
                if (i === 0 || (prev && d.getMonth() !== prev.getMonth())) {
                    return d.toLocaleDateString('en-US', { month: 'short' });
                }
                return '';
            });

            const totalValues = chartData.map(p => p.portfolio_value || p.value || 0);
            const equityValues = chartData.map(p => p.equity_holdings_value || 0);
            const capitalLine = chartData.map(p => p.capital_contributions_net || this.capitalInvested || 0);
            const grossLine = this.grossContributions > 0
                ? labels.map(() => this.grossContributions)
                : null;

            // Background fill: green above cost basis, red below
            const costBase = capitalLine.length > 0 ? capitalLine[capitalLine.length - 1] : 0;

            // Moving averages
            const ma20 = this.computeMA(totalValues, 20);
            const ma50 = this.computeMA(totalValues, 50);
            const ma200 = this.computeMA(totalValues, 200);

            const datasets = [
                {
                    label: 'Portfolio Value',
                    data: totalValues,
                    borderColor: '#000',
                    borderWidth: 2,
                    borderDash: [],
                    pointRadius: 0,
                    pointHoverRadius: 4,
                    fill: {
                        target: { value: costBase },
                        above: 'rgba(45, 138, 78, 0.08)',
                        below: 'rgba(181, 71, 71, 0.08)',
                    },
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
            ];

            if (grossLine) {
                datasets.push({
                    label: 'Gross Contributions',
                    data: grossLine,
                    borderColor: '#888',
                    borderWidth: 1,
                    borderDash: [2, 2],
                    pointRadius: 0,
                    pointHoverRadius: 4,
                    fill: false,
                    tension: 0,
                    hidden: !this.showChartBreakdown,
                });
            }

            // Add MA datasets (hidden by default, toggled via controls)
            if (totalValues.length >= 20) {
                datasets.push({
                    label: '20 MA',
                    data: ma20,
                    borderColor: '#6fa8dc',
                    borderWidth: 1,
                    borderDash: [],
                    pointRadius: 0,
                    pointHoverRadius: 0,
                    fill: false,
                    tension: 0.2,
                    hidden: !this.showMA20,
                });
            }
            if (totalValues.length >= 50) {
                datasets.push({
                    label: '50 MA',
                    data: ma50,
                    borderColor: '#e69138',
                    borderWidth: 1,
                    borderDash: [],
                    pointRadius: 0,
                    pointHoverRadius: 0,
                    fill: false,
                    tension: 0.2,
                    hidden: !this.showMA50,
                });
            }
            if (totalValues.length >= 200) {
                datasets.push({
                    label: '200 MA',
                    data: ma200,
                    borderColor: '#cc4125',
                    borderWidth: 1,
                    borderDash: [],
                    pointRadius: 0,
                    pointHoverRadius: 0,
                    fill: false,
                    tension: 0.2,
                    hidden: !this.showMA200,
                });
            }

            this.chartInstance = new Chart(canvas, {
                type: 'line',
                data: { labels: labels, datasets: datasets },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: {
                        mode: 'index',
                        intersect: false,
                    },
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            backgroundColor: '#fff',
                            titleColor: '#000',
                            bodyColor: '#000',
                            borderColor: '#000',
                            borderWidth: 1,
                            titleFont: { family: "'IBM Plex Mono', monospace", size: 11 },
                            bodyFont: { family: "'IBM Plex Mono', monospace", size: 11 },
                            filter: function(item) { return item.raw != null; },
                            callbacks: {
                                title: function(items) {
                                    if (!items.length) return '';
                                    const idx = items[0].dataIndex;
                                    const chart = items[0].chart;
                                    const rawData = chart.config._config?.rawDates;
                                    if (rawData && rawData[idx]) {
                                        const d = new Date(rawData[idx]);
                                        return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
                                    }
                                    return items[0].label || '';
                                },
                                label: function(ctx) {
                                    if (ctx.raw == null) return null;
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
                                autoSkip: true,
                                maxTicksLimit: 8,
                                maxRotation: 0,
                                callback: function(val, idx) {
                                    const label = this.getLabelForValue(val);
                                    return label || null;
                                },
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

            // Store raw dates for tooltip
            this.chartInstance.config._config.rawDates = chartData.map(p => p.date);
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
                    this._applyPortfolioData(data);
                }
                // Re-fetch growth data, watchlist, and compliance with force refresh
                this.fetchGrowthData(true);
                this.fetchWatchlist();
                this.fetchCompliance();
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
            return h.yesterday_price_change;
        },
        fmtTodayChange(val) {
            if (val == null) return '';
            const sign = val >= 0 ? '+' : '-';
            const abs = Math.abs(val);
            if (abs >= 1000000) return sign + '$' + (abs / 1000000).toFixed(1) + 'M';
            if (abs >= 1000) return sign + '$' + (abs / 1000).toFixed(1) + 'K';
            return sign + '$' + abs.toFixed(0);
        },
        holdingBreadthClass(h) {
            return 'breadth-' + (h.breadth_status || 'flat');
        },
        holdingBreadthArrow(h) {
            const s = h.breadth_status;
            if (s === 'rising') return '\u25B2';
            if (s === 'falling') return '\u25BC';
            return '\u25C6';
        },
        holdingDailyPct(h) {
            return h.yesterday_price_change_pct;
        },
        holdingForSegment(seg) {
            return this.holdings.find(h => h.ticker === seg.ticker);
        },
        segmentTooltip(seg) {
            return seg.ticker + ' ' + seg.status + ' (' + (seg.weight_pct || 0).toFixed(1) + '%)';
        },
        trendSegmentTooltip(seg) {
            return seg.ticker + ' ' + (seg.label || seg.status) + ' (' + (seg.weight_pct || 0).toFixed(1) + '%)';
        },
        fmtPrice(val) {
            return val != null ? '$' + Number(val).toFixed(2) : '';
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
                const ssrData = window.__VIRE_DATA__;
                if (ssrData && ssrData.portfolios) {
                    const data = ssrData.portfolios;
                    this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
                    this.defaultPortfolio = data.default || '';
                    if (this.defaultPortfolio) {
                        this.selected = this.defaultPortfolio;
                    } else if (this.portfolios.length > 0) {
                        this.selected = this.portfolios[0].name;
                    }
                    if (ssrData.transactions) {
                        const td = ssrData.transactions;
                        const txns = td.transactions || [];
                        txns.sort((a, b) => new Date(b.date) - new Date(a.date));
                        this.transactions = txns;
                        this.accounts = td.accounts || [];
                        const summary = td.summary || {};
                        this.totalCash = summary.capital_gross || 0;
                        this.transactionCount = summary.transaction_count || 0;
                        this.byCategory = summary.net_cash_by_category || {};
                    }
                    window.__VIRE_DATA__ = null;
                    this.loading = false;
                    return;
                }
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

// Stock Detail component
function stockDetail() {
    return {
        ticker: '',
        holding: null,
        portfolioName: '',
        detail: null,
        trades: [],
        position: null,
        positionTimeline: [],
        walkChart: null,
        hasWalkData: false,
        hasTrades: false,

        fmt(val) {
            if (val == null) return '-';
            return '$' + Number(val).toLocaleString('en-AU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
        },
        pct(val) {
            if (val == null) return '-';
            return Number(val).toFixed(2) + '%';
        },
        gainClass(val) {
            if (val == null || val === 0) return '';
            return val > 0 ? 'gain-positive' : 'gain-negative';
        },
        trendArrow(score) {
            if (score == null) return '';
            if (score > 0) return '\u25B2';
            if (score < 0) return '\u25BC';
            return '\u25C6';
        },
        trendArrowClass(score) {
            if (score == null) return '';
            if (score > 0) return 'change-up';
            if (score < 0) return 'change-down';
            return 'change-neutral';
        },
        holdingCurrency() {
            return this.position?.original_currency || this.holding?.original_currency || 'AUD';
        },
        isForeignCurrency() {
            return this.holdingCurrency() !== 'AUD';
        },
        dividendDisplay() {
            const recv = this.position?.dividend_received || this.holding?.dividend_received || 0;
            const fcast = this.position?.dividend_forecast || this.holding?.dividend_forecast || 0;
            if (recv === 0 && fcast === 0) return null;
            return this.fmt(recv) + ' received / ' + this.fmt(fcast) + ' forecast';
        },
        fmtMarketCap(val) {
            if (val == null) return '-';
            const num = Number(val);
            if (num >= 1e12) return '$' + (num / 1e12).toFixed(2) + 'T';
            if (num >= 1e9) return '$' + (num / 1e9).toFixed(2) + 'B';
            if (num >= 1e6) return '$' + (num / 1e6).toFixed(2) + 'M';
            return '$' + num.toLocaleString('en-AU');
        },
        fmtDate(dateStr) {
            if (!dateStr) return '-';
            const d = new Date(dateStr);
            if (isNaN(d)) return dateStr;
            return d.toLocaleDateString('en-AU', { year: 'numeric', month: 'short', day: 'numeric' });
        },

        tradeTotalFees() {
            return this.trades.reduce((sum, t) => sum + (Number(t.fees) || 0), 0);
        },
        tradeSignedValue(t) {
            const v = Number(t.value) || 0;
            return t.type === 'Buy' ? v : -v;
        },
        tradeTotalValue() {
            return this.trades.reduce((sum, t) => sum + this.tradeSignedValue(t), 0);
        },
        _buildRealisedPLMap() {
            if (this._realisedPLMap) return this._realisedPLMap;
            const map = new Map();
            for (const p of (this.positionTimeline || [])) {
                if (p.trade_events) {
                    for (const te of p.trade_events) {
                        if (te.realised_pnl != null) {
                            const key = te.type.toLowerCase() + '|' + te.units + '|' + Number(te.price).toFixed(4);
                            map.set(key, te.realised_pnl);
                        }
                    }
                }
            }
            this._realisedPLMap = map;
            return map;
        },
        tradeRealisedPL(t) {
            if (t.type !== 'Sell') return null;
            const map = this._buildRealisedPLMap();
            const key = t.type.toLowerCase() + '|' + t.units + '|' + Number(t.price).toFixed(4);
            const val = map.get(key);
            if (val != null) return val;
            return null;
        },

        renderWalkChart() {
            const ctx = document.getElementById('walkChart');
            if (!ctx) return;
            if (this.walkChart) { this.walkChart.destroy(); }
            if (typeof Chart === 'undefined') return;

            const timeline = this.positionTimeline;
            const labels = timeline.map(p => this.fmtDate(p.date));

            // New chart: close_price vs breakeven_price with fill between
            if (timeline[0] && timeline[0].close_price != null) {
                const closeData = timeline.map(p => p.close_price);
                const breakevenData = timeline.map(p => p.breakeven_price);

                // Merge same-day trade events for vertical line annotations
                const tradeLines = [];
                for (let i = 0; i < timeline.length; i++) {
                    const p = timeline[i];
                    if (p.trade_events && p.trade_events.length > 0) {
                        let totalBuyUnits = 0, totalSellUnits = 0;
                        let totalRealisedPnl = 0, hasRealised = false;
                        const details = [];
                        for (const te of p.trade_events) {
                            const lbl = te.type.charAt(0).toUpperCase() + te.type.slice(1);
                            details.push(lbl + ' ' + (te.units || '') + ' @ $' + Number(te.price).toFixed(4));
                            if (te.type.toLowerCase() === 'buy') totalBuyUnits += (te.units || 0);
                            else totalSellUnits += (te.units || 0);
                            if (te.realised_pnl != null) { totalRealisedPnl += te.realised_pnl; hasRealised = true; }
                        }
                        const isSell = totalSellUnits > 0;
                        tradeLines.push({
                            index: i, label: labels[i], date: p.date,
                            color: isSell ? '#dc2626' : '#16a34a',
                            isSell, totalBuyUnits, totalSellUnits,
                            totalRealisedPnl, hasRealised, details,
                            breakeven: p.breakeven_price, closePrice: p.close_price,
                        });
                    }
                }

                // Custom plugin to draw vertical trade lines with markers
                const tradeLinePlugin = {
                    id: 'tradeLines',
                    afterDatasetsDraw(chart) {
                        const { ctx: c, chartArea, scales: { x, y } } = chart;
                        for (const tl of tradeLines) {
                            const xPos = x.getPixelForValue(tl.index);
                            c.save();
                            c.strokeStyle = tl.color;
                            c.lineWidth = 1.5;
                            c.setLineDash([3, 3]);
                            c.beginPath();
                            c.moveTo(xPos, chartArea.top);
                            c.lineTo(xPos, chartArea.bottom);
                            c.stroke();
                            c.restore();
                            // Draw small marker at close price
                            const yPos = y.getPixelForValue(tl.closePrice);
                            c.save();
                            c.fillStyle = tl.color;
                            c.beginPath();
                            if (tl.isSell) {
                                c.moveTo(xPos, yPos + 5);
                                c.lineTo(xPos - 4, yPos - 2);
                                c.lineTo(xPos + 4, yPos - 2);
                            } else {
                                c.moveTo(xPos, yPos - 5);
                                c.lineTo(xPos - 4, yPos + 2);
                                c.lineTo(xPos + 4, yPos + 2);
                            }
                            c.fill();
                            c.restore();
                        }
                    }
                };

                this.walkChart = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels,
                        datasets: [
                            {
                                label: 'Close Price',
                                data: closeData,
                                borderColor: '#334155',
                                borderWidth: 2,
                                pointRadius: 0,
                                fill: { target: 1, above: 'rgba(45, 138, 79, 0.15)', below: 'rgba(192, 96, 96, 0.15)' },
                            },
                            {
                                label: 'Breakeven',
                                data: breakevenData,
                                borderColor: '#888',
                                borderWidth: 1.5,
                                borderDash: [4, 3],
                                pointRadius: 0,
                                stepped: 'before',
                                fill: false,
                            },
                        ]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: { display: true, position: 'bottom', labels: { boxWidth: 12, font: { size: 10, family: "'IBM Plex Mono', monospace" } } },
                            filler: {},
                            tooltip: {
                                callbacks: {
                                    title: (items) => {
                                        if (!items.length) return '';
                                        const idx = items[0].dataIndex;
                                        const tl = tradeLines.find(t => t.index === idx);
                                        if (tl) return [labels[idx], ...tl.details];
                                        return labels[idx];
                                    },
                                    label: (ctx) => {
                                        const idx = ctx.dataIndex;
                                        const p = timeline[idx];
                                        const price = '$' + Number(ctx.parsed.y).toLocaleString('en-AU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
                                        const lines = [ctx.dataset.label + ': ' + price];
                                        if (p && p.close_price != null && p.breakeven_price != null && p.units) {
                                            const plDollar = (p.close_price - p.breakeven_price) * p.units;
                                            const plPct = (p.close_price - p.breakeven_price) / p.breakeven_price * 100;
                                            const sign = plDollar >= 0 ? '+' : '';
                                            lines.push('P&L: ' + sign + '$' + Number(plDollar).toLocaleString('en-AU', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) + ' (' + sign + plPct.toFixed(2) + '%)');
                                        }
                                        const tl = tradeLines.find(t => t.index === idx);
                                        if (tl && tl.hasRealised && ctx.datasetIndex === 0) {
                                            const sign = tl.totalRealisedPnl >= 0 ? '+' : '';
                                            lines.push('Realised: ' + sign + '$' + Number(tl.totalRealisedPnl).toLocaleString('en-AU', { minimumFractionDigits: 0, maximumFractionDigits: 0 }));
                                        }
                                        return lines;
                                    }
                                }
                            }
                        },
                        scales: {
                            x: { display: true, ticks: { maxTicksLimit: 8, font: { size: 9, family: "'IBM Plex Mono', monospace" } }, grid: { display: false } },
                            y: {
                                display: true,
                                ticks: { font: { size: 9, family: "'IBM Plex Mono', monospace" } },
                                grid: { color: '#eee' },
                            },
                        },
                    },
                    plugins: [tradeLinePlugin],
                });
                return;
            }

            // Fallback: old P&L chart logic
            const plData = timeline.map(p => p.net_return);
            const aboveZero = plData.map(v => v >= 0 ? v : 0);
            const belowZero = plData.map(v => v < 0 ? v : 0);

            const buyPoints = [];
            const sellPoints = [];
            for (const t of this.trades) {
                const dateStr = this.fmtDate(t.date);
                const idx = labels.indexOf(dateStr);
                if (idx >= 0) {
                    const point = { x: dateStr, y: plData[idx] };
                    if (t.type === 'Buy') buyPoints.push(point);
                    else sellPoints.push(point);
                }
            }

            this.walkChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels,
                    datasets: [
                        {
                            label: 'P&L (gain)',
                            data: aboveZero,
                            borderColor: '#2d8a4e',
                            backgroundColor: 'rgba(45, 138, 79, 0.15)',
                            borderWidth: 2,
                            pointRadius: 0,
                            fill: 'origin',
                        },
                        {
                            label: 'P&L (loss)',
                            data: belowZero,
                            borderColor: '#c06060',
                            backgroundColor: 'rgba(192, 96, 96, 0.15)',
                            borderWidth: 2,
                            pointRadius: 0,
                            fill: 'origin',
                        },
                        {
                            label: 'Buy',
                            data: buyPoints,
                            type: 'scatter',
                            pointRadius: 6,
                            pointStyle: 'triangle',
                            backgroundColor: '#16a34a',
                            borderColor: '#16a34a',
                        },
                        {
                            label: 'Sell',
                            data: sellPoints,
                            type: 'scatter',
                            pointRadius: 6,
                            pointStyle: 'triangle',
                            rotation: 180,
                            backgroundColor: '#dc2626',
                            borderColor: '#dc2626',
                        },
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: true, position: 'bottom', labels: { boxWidth: 12, font: { size: 10, family: "'IBM Plex Mono', monospace" } } },
                        tooltip: {
                            callbacks: {
                                label: (ctx) => {
                                    if (ctx.dataset.label === 'Buy' || ctx.dataset.label === 'Sell') {
                                        return ctx.dataset.label;
                                    }
                                    return 'P&L: $' + Number(ctx.parsed.y).toLocaleString('en-AU', { minimumFractionDigits: 0, maximumFractionDigits: 0 });
                                }
                            }
                        }
                    },
                    scales: {
                        x: { display: true, ticks: { maxTicksLimit: 8, font: { size: 9, family: "'IBM Plex Mono', monospace" } }, grid: { display: false } },
                        y: {
                            display: true,
                            ticks: { font: { size: 9, family: "'IBM Plex Mono', monospace" } },
                            grid: { color: (ctx) => ctx.tick.value === 0 ? '#000' : '#eee', lineWidth: (ctx) => ctx.tick.value === 0 ? 2 : 1 },
                        },
                    },
                },
            });
        },
        destroyWalkChart() {
            if (this.walkChart) { this.walkChart.destroy(); this.walkChart = null; }
        },

        init() {
            const ssrData = window.__VIRE_DATA__;
            if (ssrData) {
                this.ticker = ssrData.ticker || '';
                this.holding = ssrData.holding || null;
                this.portfolioName = ssrData.portfolioName || '';
                this.detail = ssrData.stockDetail || null;
                window.__VIRE_DATA__ = null;
            }
            if (this.detail) {
                this.position = this.detail.position || null;
                this.trades = this.detail.trades || [];
                this.positionTimeline = this.detail.position_timeline || [];
                this.hasWalkData = this.positionTimeline.length > 0;
                this.hasTrades = this.trades.length > 0;
            }
            this.$nextTick(() => {
                if (this.detail && this.hasWalkData) {
                    this.renderWalkChart();
                }
            });
        },
    };
}

// Portfolio Strategy component
function portfolioStrategy() {
    return {
        portfolios: [],
        selected: '',
        defaultPortfolio: '',
        strategyHtml: '',
        planItems: [],
        loading: true,
        error: '',
        get isDefault() { return this.selected === this.defaultPortfolio; },

        async init() {
            try {
                const ssrData = window.__VIRE_DATA__;
                if (ssrData && ssrData.portfolios) {
                    const data = ssrData.portfolios;
                    this.portfolios = vireStore.dedup(data.portfolios || [], 'name');
                    this.defaultPortfolio = data.default || '';
                    if (this.defaultPortfolio) {
                        this.selected = this.defaultPortfolio;
                    } else if (this.portfolios.length > 0) {
                        this.selected = this.portfolios[0].name;
                    }
                    if (ssrData.strategy) {
                        this.renderStrategy(ssrData.strategy);
                    }
                    if (ssrData.plan) {
                        this.renderPlan(ssrData.plan);
                    }
                    window.__VIRE_DATA__ = null;
                    this.loading = false;
                    return;
                }
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
                    this.renderStrategy(await strategyRes.json());
                } else {
                    this.strategyHtml = '<p class="text-muted">No strategy defined.</p>';
                }

                if (planRes.ok) {
                    this.renderPlan(await planRes.json());
                } else {
                    this.planItems = [];
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

        renderStrategy(data) {
            const notes = data.notes || '';
            if (notes && typeof marked !== 'undefined') {
                this.strategyHtml = marked.parse(notes);
            } else if (notes) {
                this.strategyHtml = '<pre>' + notes.replace(/</g, '&lt;') + '</pre>';
            } else {
                this.strategyHtml = '<p class="text-muted">No strategy defined.</p>';
            }
        },

        renderPlan(data) {
            this.planItems = data.items || [];
        },
    };
}

function promptEditor() {
    return {
        content: '',
        saving: false,
        message: '',
        success: false,
        promptName: '',
        init() {
            const data = window.__VIRE_DATA__;
            if (data && data.prompt) {
                this.content = data.prompt.content || '';
                this.promptName = data.prompt.name || '';
            }
            window.__VIRE_DATA__ = null;
        },
        async save() {
            this.saving = true;
            this.message = '';
            try {
                const res = await fetch('/admin/prompts/' + encodeURIComponent(this.promptName), {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content: this.content }),
                });
                if (res.ok) {
                    this.success = true;
                    this.message = 'Saved successfully';
                } else {
                    const data = await res.json().catch(() => ({}));
                    this.success = false;
                    this.message = data.error || 'Save failed';
                }
            } catch (e) {
                this.success = false;
                this.message = 'Network error';
            } finally {
                this.saving = false;
                setTimeout(() => { this.message = ''; }, 5000);
            }
        }
    };
}

// Gemini Usage component
function geminiUsage() {
    return {
        period: 'week',
        usage: null,
        loading: false,
        error: '',

        init() {
            const ssrData = window.__VIRE_DATA__;
            if (ssrData && ssrData.usage) {
                this.usage = ssrData.usage;
                window.__VIRE_DATA__ = null;
                return;
            }
            this.loadUsage();
        },

        async loadUsage() {
            this.loading = true;
            this.error = '';
            try {
                const res = await vireStore.fetch('/api/admin/gemini/usage?period=' + encodeURIComponent(this.period));
                if (!res.ok) {
                    this.error = 'Failed to load usage data (HTTP ' + res.status + ')';
                    this.loading = false;
                    return;
                }
                this.usage = await res.json();
            } catch (e) {
                this.error = 'Network error loading usage data';
            } finally {
                this.loading = false;
            }
        },

        async switchPeriod(p) {
            if (p === this.period) return;
            this.period = p;
            await this.loadUsage();
        },

        fmtTokens(n) {
            if (n == null) return '-';
            return Number(n).toLocaleString('en-AU');
        },

        fmtDuration(ms) {
            if (ms == null) return '-';
            return (Number(ms) / 1000).toFixed(2) + 's';
        },

        fmtTimestamp(ts) {
            if (!ts) return '-';
            const d = new Date(ts);
            if (isNaN(d)) return ts;
            return d.toLocaleString('en-AU', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        },
    };
}
