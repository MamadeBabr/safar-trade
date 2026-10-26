/**
 * SAFAR TRADE - Frontend Application Logic
 * Vanilla JavaScript implementation for Market Watch, Charting, Auth, Subscriptions, and WebSocket
 */

// =============================================================================
// 1. GLOBAL APPLICATION STATE
// =============================================================================
const state = {
  // Auth & User
  token: localStorage.getItem('safar_token') || '',
  refreshToken: localStorage.getItem('safar_refresh_token') || '',
  user: null,
  subscription: null,

  // Market & Navigation
  activeTab: 'market',
  symbols: [
    'BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT',
    'DOGEUSDT', 'ADAUSDT', 'AVAXUSDT', 'LINKUSDT', 'SUIUSDT'
  ],
  tickers: {},             // Map: symbol -> { o, h, l, c, v, t, is_closed }
  selectedSymbol: 'BTCUSDT',
  selectedTimeframe: '15m',
  selectedCountback: 100,

  // Charting (Lightweight Charts)
  chart: null,
  candleSeries: null,
  chartCandles: [],        // Current array of OHLC bars

  // WebSocket & Polling
  ws: null,
  wsReconnectTimer: null,
  wsSubscribedTopics: new Set(),
  tickerPollTimer: null,

  // Alerts
  alerts: [],
  unreadAlerts: 0,
};

// =============================================================================
// 2. HTTP API CLIENT & TOKEN REFRESH INTERCEPTOR
// =============================================================================
async function apiCall(endpoint, options = {}) {
  const url = endpoint.startsWith('http') ? endpoint : endpoint;
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {})
  };

  if (state.token) {
    headers['Authorization'] = `Bearer ${state.token}`;
  }

  let response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    console.error(`Network error requesting ${url}:`, err);
    throw new Error('Network connection failed. Please check your internet or server.');
  }

  // Handle Token Expiry (401 Unauthorized)
  if (response.status === 401 && state.refreshToken) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      headers['Authorization'] = `Bearer ${state.token}`;
      response = await fetch(url, { ...options, headers });
    } else {
      logout();
      showToast('Session expired. Please sign in again.', 'warning');
      throw new Error('Session expired');
    }
  }

  let data = null;
  const contentType = response.headers.get('content-type');
  if (contentType && contentType.includes('application/json')) {
    data = await response.json();
  }

  if (!response.ok) {
    const errorMsg = data?.detail || data?.message || data?.title || `Request failed (${response.status})`;
    const error = new Error(errorMsg);
    error.status = response.status;
    error.data = data;
    throw error;
  }

  return data;
}

async function refreshAccessToken() {
  if (!state.refreshToken) return false;
  try {
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: state.refreshToken }),
    });

    if (!res.ok) return false;

    const tokens = await res.json();
    setTokens(tokens.access_token, tokens.refresh_token);
    return true;
  } catch (err) {
    console.error('Failed to refresh access token:', err);
    return false;
  }
}

function setTokens(access, refresh) {
  state.token = access || '';
  state.refreshToken = refresh || '';
  if (access) {
    localStorage.setItem('safar_token', access);
  } else {
    localStorage.removeItem('safar_token');
  }
  if (refresh) {
    localStorage.setItem('safar_refresh_token', refresh);
  } else {
    localStorage.removeItem('safar_refresh_token');
  }
}

function hasActiveSubscription() {
  if (!state.subscription) return false;
  const status = state.subscription.status?.toLowerCase();
  return status === 'active' || status === 'trialing';
}

// =============================================================================
// 3. AUTHENTICATION & USER MANAGEMENT
// =============================================================================
async function initAuth() {
  if (!state.token) {
    renderUserNav();
    return;
  }

  try {
    const res = await apiCall('/api/v1/auth/me');
    state.user = res.user;
    state.subscription = res.subscription || null;
  } catch (err) {
    console.warn('Could not restore user session:', err.message);
    setTokens('', '');
    state.user = null;
    state.subscription = null;
  }

  renderUserNav();
  updatePaywallBanner();
  initWebSocket();
}

async function handleLogin(email, password) {
  const res = await apiCall('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password })
  });

  setTokens(res.access_token, res.refresh_token);
  state.user = res.user;
  state.subscription = res.subscription || null;

  renderUserNav();
  updatePaywallBanner();
  closeAuthModal();
  showToast(`Welcome back, ${res.user.full_name || res.user.email}!`, 'success');

  // Connect WebSocket & refresh UI
  initWebSocket();
  if (state.activeTab === 'chart') loadCandleHistory();
  if (state.activeTab === 'subscriptions') loadSubscriptionsTab();
  if (state.activeTab === 'events') loadEconomicEvents();
}

async function handleRegister(email, password, fullName) {
  const res = await apiCall('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, full_name: fullName })
  });

  setTokens(res.access_token, res.refresh_token);
  state.user = res.user;
  state.subscription = res.subscription || null;

  renderUserNav();
  updatePaywallBanner();
  closeAuthModal();
  showToast('Account created successfully! Welcome to Safar Trade.', 'success');

  initWebSocket();
  if (state.activeTab === 'subscriptions') loadSubscriptionsTab();
}

async function logout() {
  if (state.token) {
    try {
      await apiCall('/api/v1/auth/logout', {
        method: 'POST',
        body: JSON.stringify({ refresh_token: state.refreshToken })
      });
    } catch (e) {
      // Ignore logout request errors
    }
  }

  setTokens('', '');
  state.user = null;
  state.subscription = null;

  closeWebSocket();
  renderUserNav();
  updatePaywallBanner();
  showToast('You have been signed out.', 'info');

  if (state.activeTab === 'subscriptions') loadSubscriptionsTab();
}

function renderUserNav() {
  const container = document.getElementById('user-profile-section');
  if (!container) return;

  if (state.user) {
    const isPro = hasActiveSubscription();
    const subLabel = isPro ? 'PRO ACTIVE' : 'NO SUBSCRIPTION';
    const subClass = isPro ? 'sub-pro' : 'sub-free';

    container.innerHTML = `
      <div class="user-badge-group">
        <div class="user-avatar">${(state.user.full_name || state.user.email)[0].toUpperCase()}</div>
        <div class="user-info-text">
          <span class="user-name">${state.user.full_name || state.user.email}</span>
          <span class="user-sub-badge ${subClass}">${subLabel}</span>
        </div>
        <button class="btn btn-xs btn-outline" onclick="logout()" title="Logout">Sign Out</button>
      </div>
    `;
  } else {
    container.innerHTML = `
      <button class="btn btn-sm btn-primary" onclick="openAuthModal('login')">
        Sign In / Register
      </button>
    `;
  }
}

function updatePaywallBanner() {
  const banner = document.getElementById('paywall-banner');
  if (!banner) return;

  if (!hasActiveSubscription()) {
    banner.style.display = 'block';
  } else {
    banner.style.display = 'none';
  }
}

// =============================================================================
// 4. NAVIGATION & TABS
// =============================================================================
function switchTab(tabId) {
  state.activeTab = tabId;

  // Update nav buttons
  document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
  const activeBtn = document.getElementById(`tab-btn-${tabId}`);
  if (activeBtn) activeBtn.classList.add('active');

  // Update panels
  document.querySelectorAll('.view-panel').forEach(panel => panel.classList.remove('active'));
  const targetPanel = document.getElementById(`view-${tabId}`);
  if (targetPanel) targetPanel.classList.add('active');

  // Reset alert badge if opening alerts tab
  if (tabId === 'alerts') {
    state.unreadAlerts = 0;
    updateAlertsBadge();
  }

  // Trigger tab-specific loaders
  if (tabId === 'market') {
    fetchLiveTickers();
  } else if (tabId === 'chart') {
    initChartIfNeeded();
    loadCandleHistory();
  } else if (tabId === 'subscriptions') {
    loadSubscriptionsTab();
  } else if (tabId === 'events') {
    loadEconomicEvents();
  }
}

// =============================================================================
// 5. TAB 1: REAL-TIME MARKET WATCH
// =============================================================================
async function loadSupportedSymbols() {
  try {
    const res = await apiCall('/api/v1/data/symbols');
    if (res && res.symbols && res.symbols.length > 0) {
      state.symbols = res.symbols;
      populateSymbolSelectors();
    }
  } catch (err) {
    console.warn('Failed to load supported symbols, using defaults:', err.message);
  }
}

function populateSymbolSelectors() {
  const select = document.getElementById('chart-symbol-select');
  if (!select) return;

  select.innerHTML = state.symbols.map(s => {
    const selected = s === state.selectedSymbol ? 'selected' : '';
    return `<option value="${s}" ${selected}>${s}</option>`;
  }).join('');

  const countElem = document.getElementById('active-symbols-count');
  if (countElem) {
    countElem.textContent = `Tracking ${state.symbols.length} Symbols`;
  }
}

async function fetchLiveTickers() {
  try {
    const res = await apiCall(`/api/v1/data/tickers?symbols=${state.symbols.slice(0, 12).join(',')}`);
    if (res && res.tickers) {
      Object.entries(res.tickers).forEach(([sym, candle]) => {
        if (candle) {
          updateTickerData(sym, candle, false);
        }
      });
      renderMarketGrid();
    }
  } catch (err) {
    console.warn('Could not fetch tickers:', err.message);
  }
}

function updateTickerData(symbol, candle, flash = true) {
  const prev = state.tickers[symbol];
  state.tickers[symbol] = candle;

  // Flash card if it exists in DOM
  if (flash) {
    const card = document.getElementById(`ticker-card-${symbol}`);
    if (card) {
      const priceElem = card.querySelector('.ticker-price');
      if (priceElem) {
        priceElem.textContent = formatPrice(candle.c || candle.close);
      }

      const prevClose = prev ? parseFloat(prev.c || prev.close) : 0;
      const currClose = parseFloat(candle.c || candle.close);

      card.classList.remove('flash-up', 'flash-down');
      void card.offsetWidth; // Force CSS reflow
      if (currClose >= prevClose) {
        card.classList.add('flash-up');
      } else {
        card.classList.add('flash-down');
      }
    }
  }
}

function renderMarketGrid() {
  const grid = document.getElementById('market-grid');
  if (!grid) return;

  const symbolsToRender = state.symbols.slice(0, 12);

  grid.innerHTML = symbolsToRender.map(sym => {
    const ticker = state.tickers[sym];
    const open = ticker ? parseFloat(ticker.o || ticker.open || 0) : 0;
    const close = ticker ? parseFloat(ticker.c || ticker.close || 0) : 0;
    const high = ticker ? parseFloat(ticker.h || ticker.high || 0) : 0;
    const low = ticker ? parseFloat(ticker.l || ticker.low || 0) : 0;
    const volume = ticker ? parseFloat(ticker.v || ticker.volume || 0) : 0;

    let changePercent = 0;
    if (open > 0 && close > 0) {
      changePercent = ((close - open) / open) * 100;
    }

    const isPositive = changePercent >= 0;
    const changeClass = isPositive ? 'change-positive' : 'change-negative';
    const changeSign = isPositive ? '+' : '';
    const formattedPrice = close > 0 ? formatPrice(close) : '--';

    return `
      <div id="ticker-card-${sym}" class="ticker-card" onclick="openDeepDiveChart('${sym}')">
        <div class="ticker-card-header">
          <div class="ticker-symbol-group">
            <span class="ticker-symbol">${sym}</span>
            <span class="ticker-exchange">BINANCE</span>
          </div>
          <span class="ticker-badge ${changeClass}">
            ${changeSign}${changePercent.toFixed(2)}%
          </span>
        </div>

        <div class="ticker-price-row">
          <span class="ticker-price">${formattedPrice}</span>
          <span class="ticker-currency">USDT</span>
        </div>

        <div class="ticker-meta-grid">
          <div class="meta-item">
            <span class="meta-label">24h High</span>
            <span class="meta-val">${high > 0 ? formatPrice(high) : '--'}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">24h Low</span>
            <span class="meta-val">${low > 0 ? formatPrice(low) : '--'}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">Volume</span>
            <span class="meta-val">${volume > 0 ? volume.toFixed(2) : '--'}</span>
          </div>
        </div>

        <button class="btn btn-xs btn-outline btn-block mt-3" onclick="event.stopPropagation(); openDeepDiveChart('${sym}')">
          In-Depth Chart & Candles →
        </button>
      </div>
    `;
  }).join('');
}

function openDeepDiveChart(symbol) {
  state.selectedSymbol = symbol;
  const select = document.getElementById('chart-symbol-select');
  if (select) select.value = symbol;
  switchTab('chart');
}

// =============================================================================
// 6. TAB 2: IN-DEPTH CANDLESTICK CHART & HISTORICAL DATA
// =============================================================================
function initChartIfNeeded() {
  const container = document.getElementById('candlestick-chart-container');
  if (!container || state.chart) return;

  if (typeof LightweightCharts === 'undefined') {
    console.error('LightweightCharts library not loaded');
    return;
  }

  state.chart = LightweightCharts.createChart(container, {
    width: container.clientWidth || 800,
    height: 480,
    layout: {
      background: { color: '#0f141f' },
      textColor: '#94a3b8',
      fontSize: 12,
      fontFamily: "'JetBrains Mono', monospace",
    },
    grid: {
      vertLines: { color: 'rgba(255, 255, 255, 0.05)' },
      horzLines: { color: 'rgba(255, 255, 255, 0.05)' },
    },
    crosshair: {
      mode: LightweightCharts.CrosshairMode.Normal,
      vertLine: {
        width: 1,
        color: '#6366f1',
        style: LightweightCharts.LineStyle.Dashed,
      },
      horzLine: {
        width: 1,
        color: '#6366f1',
        style: LightweightCharts.LineStyle.Dashed,
      },
    },
    rightPriceScale: {
      borderColor: 'rgba(255, 255, 255, 0.1)',
      autoScale: true,
    },
    timeScale: {
      borderColor: 'rgba(255, 255, 255, 0.1)',
      timeVisible: true,
      secondsVisible: false,
    },
  });

  state.candleSeries = state.chart.addCandlestickSeries({
    upColor: '#10b981',
    downColor: '#f43f5e',
    borderVisible: false,
    wickUpColor: '#10b981',
    wickDownColor: '#f43f5e',
  });

  // Responsive chart resize listener
  window.addEventListener('resize', () => {
    if (state.chart && container) {
      state.chart.applyOptions({ width: container.clientWidth });
    }
  });
}

async function loadCandleHistory() {
  initChartIfNeeded();

  const loader = document.getElementById('chart-loader');
  if (loader) loader.style.display = 'flex';

  const symbol = state.selectedSymbol;
  const tf = state.selectedTimeframe;
  const countback = state.selectedCountback;

  try {
    const res = await apiCall(`/api/v1/data/candles?symbol=${symbol}&timeframe=${tf}&countback=${countback}`);
    if (loader) loader.style.display = 'none';

    const candles = res.candles || [];
    if (res.live_candle) {
      candles.push(res.live_candle);
    }

    state.chartCandles = candles;

    // Transform for TradingView Lightweight Charts
    // Timestamp must be in seconds (Unix timestamp) and sorted ascending
    const seriesData = candles.map(c => {
      const timeMs = c.open_time || c.openTime || c.t;
      return {
        time: Math.floor(timeMs / 1000),
        open: parseFloat(c.open || c.o),
        high: parseFloat(c.high || c.h),
        low: parseFloat(c.low || c.l),
        close: parseFloat(c.close || c.c),
      };
    }).sort((a, b) => a.time - b.time);

    // Filter duplicate timestamps if any
    const uniqueSeriesData = [];
    const seenTimes = new Set();
    for (const bar of seriesData) {
      if (!seenTimes.has(bar.time)) {
        seenTimes.add(bar.time);
        uniqueSeriesData.push(bar);
      }
    }

    if (state.candleSeries && uniqueSeriesData.length > 0) {
      state.candleSeries.setData(uniqueSeriesData);
      state.chart.timeScale().fitContent();

      // Update current price display in toolbar
      const latestBar = uniqueSeriesData[uniqueSeriesData.length - 1];
      updatePriceDisplay(latestBar.close, latestBar.time * 1000);
    }

    renderCandlesTable(candles);

    // Ensure we are subscribed to live WebSocket updates for this symbol & tf
    subscribeToSymbolTopic(symbol, tf);

  } catch (err) {
    if (loader) loader.style.display = 'none';
    console.error('Failed to load candle history:', err);

    if (err.status === 403 || err.status === 401) {
      showToast('Candlestick historical data is restricted to active subscribers.', 'warning');
      updatePaywallBanner();
    } else {
      showToast(err.message || 'Failed to fetch candles', 'error');
    }
  }
}

function renderCandlesTable(candles) {
  const tbody = document.getElementById('candles-table-body');
  const countElem = document.getElementById('candles-record-count');
  if (!tbody) return;

  if (countElem) {
    countElem.textContent = `${candles.length} bars`;
  }

  if (!candles || candles.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" class="text-center text-muted">No candle records available.</td></tr>`;
    return;
  }

  // Show latest 50 records in reverse order
  const displayCandles = [...candles].reverse().slice(0, 50);

  tbody.innerHTML = displayCandles.map(c => {
    const t = c.open_time || c.openTime || c.t;
    const dateStr = new Date(t).toISOString().replace('T', ' ').substring(0, 19);
    const o = parseFloat(c.open || c.o);
    const h = parseFloat(c.high || c.h);
    const l = parseFloat(c.low || c.l);
    const close = parseFloat(c.close || c.c);
    const v = parseFloat(c.volume || c.v || 0);

    const isGreen = close >= o;
    const colorClass = isGreen ? 'text-green' : 'text-red';

    return `
      <tr>
        <td class="font-mono">${dateStr}</td>
        <td class="font-mono">${formatPrice(o)}</td>
        <td class="font-mono">${formatPrice(h)}</td>
        <td class="font-mono">${formatPrice(l)}</td>
        <td class="font-mono ${colorClass}">${formatPrice(close)}</td>
        <td class="font-mono">${v.toFixed(3)}</td>
      </tr>
    `;
  }).join('');
}

function updatePriceDisplay(price, timestamp) {
  const priceElem = document.getElementById('chart-price-val');
  const timeElem = document.getElementById('chart-price-time');

  if (priceElem) {
    priceElem.textContent = formatPrice(price);
  }
  if (timeElem && timestamp) {
    const date = new Date(timestamp);
    timeElem.textContent = `Last update: ${date.toLocaleTimeString()}`;
  }
}

function onSymbolChange(newSymbol) {
  state.selectedSymbol = newSymbol;
  loadCandleHistory();
}

function setTimeframe(tf) {
  state.selectedTimeframe = tf;
  document.querySelectorAll('.tf-btn').forEach(b => {
    if (b.dataset.tf === tf) b.classList.add('active');
    else b.classList.remove('active');
  });
  loadCandleHistory();
}

function setCountback(val) {
  state.selectedCountback = parseInt(val, 10) || 100;
  loadCandleHistory();
}

// =============================================================================
// 7. TAB 3: PACKAGES & SUBSCRIPTIONS
// =============================================================================
async function loadSubscriptionsTab() {
  await Promise.all([
    renderCurrentSubscription(),
    renderPackagesList()
  ]);
}

async function renderCurrentSubscription() {
  const container = document.getElementById('current-subscription-card');
  if (!container) return;

  if (!state.user) {
    container.innerHTML = `
      <div class="empty-state">
        <p>Please <strong>sign in</strong> to view and manage your active subscription.</p>
        <button class="btn btn-sm btn-primary mt-3" onclick="openAuthModal('login')">Sign In</button>
      </div>
    `;
    return;
  }

  try {
    const sub = await apiCall('/api/v1/subscriptions/current');
    state.subscription = sub;
    updatePaywallBanner();
    renderUserNav();

    const expiresDate = sub.expires_at ? new Date(sub.expires_at).toLocaleDateString() : 'N/A';
    const startsDate = sub.starts_at ? new Date(sub.starts_at).toLocaleDateString() : 'N/A';
    const pkgName = sub.package ? sub.package.name : 'Subscription';

    container.innerHTML = `
      <div class="active-sub-banner">
        <div class="sub-header-row">
          <div>
            <span class="badge badge-success">ACTIVE SUBSCRIPTION</span>
            <h2 class="sub-plan-title">${pkgName}</h2>
          </div>
          <button class="btn btn-sm btn-danger-outline" onclick="cancelSubscription()">Cancel Plan</button>
        </div>
        <div class="sub-details-grid">
          <div>
            <span class="sub-detail-label">Status:</span>
            <span class="sub-detail-val font-bold text-green">${sub.status.toUpperCase()}</span>
          </div>
          <div>
            <span class="sub-detail-label">Activated:</span>
            <span class="sub-detail-val">${startsDate}</span>
          </div>
          <div>
            <span class="sub-detail-label">Next Renewal / Expiry:</span>
            <span class="sub-detail-val">${expiresDate}</span>
          </div>
          <div>
            <span class="sub-detail-label">Auto-Renew:</span>
            <span class="sub-detail-val">${sub.auto_renew ? 'Enabled' : 'Disabled'}</span>
          </div>
        </div>
      </div>
    `;
  } catch (err) {
    state.subscription = null;
    updatePaywallBanner();
    renderUserNav();

    container.innerHTML = `
      <div class="no-sub-banner">
        <div class="no-sub-content">
          <h3>No Active Subscription</h3>
          <p class="text-muted">You are currently on the Free tier. Choose a package below to unlock real-time WebSocket feeds, historical candle datasets, and macroeconomic alerts.</p>
        </div>
      </div>
    `;
  }
}

async function renderPackagesList() {
  const container = document.getElementById('packages-grid');
  if (!container) return;

  try {
    const packages = await apiCall('/api/v1/packages');
    if (!packages || packages.length === 0) {
      container.innerHTML = `<div class="empty-state">No subscription packages found.</div>`;
      return;
    }

    container.innerHTML = packages.map(pkg => {
      const isCurrent = state.subscription && state.subscription.package_id === pkg.id && state.subscription.status === 'active';
      const features = Array.isArray(pkg.features) ? pkg.features : [];

      return `
        <div class="package-card ${isCurrent ? 'package-card-active' : ''}">
          ${isCurrent ? '<div class="popular-tag">CURRENT PLAN</div>' : ''}
          <h3 class="package-name">${pkg.name}</h3>
          <p class="package-desc">${pkg.description || 'Full access to professional market analytics.'}</p>
          
          <div class="package-pricing">
            <span class="price-currency">$</span>
            <span class="price-number">${parseFloat(pkg.price).toFixed(0)}</span>
            <span class="price-period">/ ${pkg.billing_period || 'month'}</span>
          </div>

          <ul class="package-features">
            ${features.map(f => `<li><span class="check-icon">✓</span> ${f}</li>`).join('')}
          </ul>

          <div class="package-action">
            ${isCurrent ? `
              <button class="btn btn-block btn-secondary" disabled>Currently Subscribed</button>
            ` : `
              <button class="btn btn-block btn-primary" onclick="subscribeToPackage(${pkg.id})">
                ${state.subscription ? 'Switch to this Plan' : 'Subscribe Now'}
              </button>
            `}
          </div>
        </div>
      `;
    }).join('');
  } catch (err) {
    container.innerHTML = `<div class="empty-state text-red">Failed to load packages: ${err.message}</div>`;
  }
}

async function subscribeToPackage(packageId) {
  if (!state.user) {
    openAuthModal('login');
    showToast('Please sign in or create an account first.', 'info');
    return;
  }

  try {
    const res = await apiCall('/api/v1/subscriptions', {
      method: 'POST',
      body: JSON.stringify({
        package_id: packageId,
        payment_method: 'card',
        auto_renew: true
      })
    });

    state.subscription = res;
    showToast('Subscription activated successfully! Enjoy full access.', 'success');
    updatePaywallBanner();
    renderUserNav();
    loadSubscriptionsTab();

    // Connect WebSocket if not connected
    initWebSocket();
  } catch (err) {
    showToast(err.message || 'Failed to activate subscription', 'error');
  }
}

async function cancelSubscription() {
  if (!confirm('Are you sure you want to cancel your subscription? You will lose access to live streams and candlestick data.')) {
    return;
  }

  try {
    await apiCall('/api/v1/subscriptions/cancel', { method: 'POST' });
    showToast('Subscription canceled successfully.', 'info');
    state.subscription = null;
    updatePaywallBanner();
    renderUserNav();
    loadSubscriptionsTab();
    closeWebSocket();
  } catch (err) {
    showToast(err.message || 'Failed to cancel subscription', 'error');
  }
}

// =============================================================================
// 8. TAB 4: FOREX FACTORY ECONOMIC CALENDAR
// =============================================================================
async function loadEconomicEvents() {
  const tbody = document.getElementById('events-table-body');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">Loading economic calendar events...</td></tr>`;

  try {
    const res = await apiCall('/api/v1/data/economic-events?limit=50');
    const events = res.events || [];

    if (events.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">No economic events scheduled.</td></tr>`;
      return;
    }

    tbody.innerHTML = events.map(evt => {
      const dateStr = new Date(evt.date).toLocaleString(undefined, {
        month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
      });

      const impact = (evt.impact || 'low').toLowerCase();
      let impactBadge = 'badge-neutral';
      if (impact === 'high') impactBadge = 'badge-danger';
      else if (impact === 'medium') impactBadge = 'badge-warning';

      return `
        <tr>
          <td class="font-mono">${dateStr}</td>
          <td><span class="currency-tag">${evt.currency}</span></td>
          <td class="font-medium">${evt.name}</td>
          <td><span class="badge ${impactBadge}">${evt.impact}</span></td>
          <td class="font-mono text-bold">${evt.actual || '--'}</td>
          <td class="font-mono text-muted">${evt.forecast || '--'}</td>
          <td class="font-mono text-dim">${evt.previous || '--'}</td>
        </tr>
      `;
    }).join('');
  } catch (err) {
    if (err.status === 403 || err.status === 401) {
      tbody.innerHTML = `
        <tr>
          <td colspan="7" class="text-center text-muted">
            <span class="text-red">Subscription Required:</span> Economic calendar data is restricted to active subscribers.
            <button class="btn btn-xs btn-primary ml-2" onclick="switchTab('subscriptions')">Upgrade Plan</button>
          </td>
        </tr>
      `;
    } else {
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-red">Failed to load events: ${err.message}</td></tr>`;
    }
  }
}

// =============================================================================
// 9. TAB 5: LIVE ALERTS FEED
// =============================================================================
function handleIncomingAlert(alert) {
  state.alerts.unshift(alert);

  if (state.activeTab !== 'alerts') {
    state.unreadAlerts++;
    updateAlertsBadge();
  }

  renderAlertsList();

  // Show toast notification
  const toastType = alert.level === 'critical' ? 'error' : (alert.level === 'warning' ? 'warning' : 'info');
  showToast(`[${alert.level?.toUpperCase()}] ${alert.title}: ${alert.message}`, toastType);
}

function updateAlertsBadge() {
  const badge = document.getElementById('alerts-badge');
  if (!badge) return;

  if (state.unreadAlerts > 0) {
    badge.textContent = state.unreadAlerts;
    badge.style.display = 'inline-block';
  } else {
    badge.style.display = 'none';
  }
}

function renderAlertsList() {
  const container = document.getElementById('alerts-list');
  if (!container) return;

  if (state.alerts.length === 0) {
    container.innerHTML = `<div class="empty-state">No alerts received yet. Live notifications will appear here.</div>`;
    return;
  }

  container.innerHTML = state.alerts.map(a => {
    const timeStr = a.timestamp ? new Date(a.timestamp).toLocaleTimeString() : new Date().toLocaleTimeString();
    const level = (a.level || 'info').toLowerCase();
    let badgeClass = 'badge-neutral';
    if (level === 'critical') badgeClass = 'badge-danger';
    else if (level === 'warning') badgeClass = 'badge-warning';

    return `
      <div class="alert-card alert-${level}">
        <div class="alert-card-header">
          <div class="alert-title-wrap">
            <span class="badge ${badgeClass}">${level.toUpperCase()}</span>
            <strong class="alert-title">${a.title}</strong>
          </div>
          <span class="alert-time font-mono">${timeStr}</span>
        </div>
        <p class="alert-msg">${a.message}</p>
        ${a.category ? `<span class="alert-cat">#${a.category}</span>` : ''}
      </div>
    `;
  }).join('');
}

function clearAlerts() {
  state.alerts = [];
  state.unreadAlerts = 0;
  updateAlertsBadge();
  renderAlertsList();
}

// =============================================================================
// 10. WEBSOCKET REAL-TIME CLIENT
// =============================================================================
function initWebSocket() {
  if (!state.token || !hasActiveSubscription()) {
    closeWebSocket();
    setWSStatus('disconnected', 'Disconnected (Sub Required)');
    return;
  }

  if (state.ws && (state.ws.readyState === WebSocket.OPEN || state.ws.readyState === WebSocket.CONNECTING)) {
    return;
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?token=${encodeURIComponent(state.token)}`;

  setWSStatus('connecting', 'Connecting...');

  try {
    state.ws = new WebSocket(wsUrl);

    state.ws.onopen = () => {
      setWSStatus('connected', 'Live Connected');
      console.log('WebSocket connected successfully');

      // Subscribe to default topics: top symbols and alerts
      const topics = [
        'alerts',
        `candles:${state.selectedSymbol}:${state.selectedTimeframe}`,
        ...state.symbols.slice(0, 10).map(s => `candles:${s}:1m`)
      ];

      sendWSCommand('subscribe', topics);
    };

    state.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleWSMessage(msg);
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err);
      }
    };

    state.ws.onerror = (err) => {
      console.warn('WebSocket error encountered:', err);
      setWSStatus('disconnected', 'Connection Error');
    };

    state.ws.onclose = (event) => {
      setWSStatus('disconnected', 'Disconnected');
      state.ws = null;

      // Auto-reconnect after 3 seconds if user still has active subscription
      if (state.token && hasActiveSubscription()) {
        clearTimeout(state.wsReconnectTimer);
        state.wsReconnectTimer = setTimeout(() => {
          console.log('Attempting WebSocket reconnect...');
          initWebSocket();
        }, 3000);
      }
    };
  } catch (err) {
    console.error('Failed to construct WebSocket:', err);
    setWSStatus('disconnected', 'Error');
  }
}

function closeWebSocket() {
  clearTimeout(state.wsReconnectTimer);
  if (state.ws) {
    state.ws.close();
    state.ws = null;
  }
  setWSStatus('disconnected', 'Disconnected');
}

function sendWSCommand(action, topics) {
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  const payload = { action, topics };
  state.ws.send(JSON.stringify(payload));
}

function subscribeToSymbolTopic(symbol, tf) {
  if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return;
  const topic = `candles:${symbol}:${tf}`;
  sendWSCommand('subscribe', [topic]);
}

function handleWSMessage(msg) {
  if (msg.type === 'candle') {
    const candle = msg.data;
    if (!candle) return;

    const sym = candle.symbol;
    const tf = candle.timeframe || '1m';

    // Update ticker card in Market Watch
    updateTickerData(sym, candle, true);

    // If candle belongs to the currently active chart symbol & timeframe, update chart bar in real-time!
    if (sym === state.selectedSymbol && tf === state.selectedTimeframe && state.candleSeries) {
      const timeMs = candle.t || candle.open_time || candle.openTime;
      const bar = {
        time: Math.floor(timeMs / 1000),
        open: parseFloat(candle.o || candle.open),
        high: parseFloat(candle.h || candle.high),
        low: parseFloat(candle.l || candle.low),
        close: parseFloat(candle.c || candle.close),
      };

      state.candleSeries.update(bar);
      updatePriceDisplay(bar.close, timeMs);
    }
  } else if (msg.type === 'alert') {
    handleIncomingAlert(msg.data || msg);
  } else if (msg.type === 'status') {
    console.log('WS Server Status:', msg.message, msg.data);
  }
}

function setWSStatus(status, text) {
  const container = document.getElementById('ws-status');
  const textElem = document.getElementById('ws-status-text');
  if (!container || !textElem) return;

  container.className = `status-indicator ${status}`;
  textElem.textContent = text;
}

// =============================================================================
// 11. AUTH MODAL & QUICK LOGIN
// =============================================================================
let currentAuthMode = 'login';

function openAuthModal(mode = 'login') {
  switchAuthMode(mode);
  const modal = document.getElementById('auth-modal');
  if (modal) modal.style.display = 'flex';
}

function closeAuthModal() {
  const modal = document.getElementById('auth-modal');
  if (modal) modal.style.display = 'none';
  const errorElem = document.getElementById('auth-error-msg');
  if (errorElem) errorElem.style.display = 'none';
}

function switchAuthMode(mode) {
  currentAuthMode = mode;
  const loginTab = document.getElementById('modal-tab-login');
  const regTab = document.getElementById('modal-tab-register');
  const nameGroup = document.getElementById('name-field-group');
  const submitBtn = document.getElementById('auth-submit-btn');

  if (mode === 'login') {
    loginTab.classList.add('active');
    regTab.classList.remove('active');
    nameGroup.style.display = 'none';
    submitBtn.textContent = 'Sign In';
  } else {
    loginTab.classList.remove('active');
    regTab.classList.add('active');
    nameGroup.style.display = 'block';
    submitBtn.textContent = 'Create Account';
  }
}

async function quickLogin(email, password) {
  try {
    await handleLogin(email, password);
  } catch (err) {
    showAuthError(err.message || 'Login failed');
  }
}

async function handleAuthSubmit(event) {
  event.preventDefault();
  const email = document.getElementById('auth-email').value.trim();
  const password = document.getElementById('auth-password').value;
  const fullName = document.getElementById('auth-name').value.trim();

  try {
    if (currentAuthMode === 'login') {
      await handleLogin(email, password);
    } else {
      await handleRegister(email, password, fullName);
    }
  } catch (err) {
    showAuthError(err.message || 'Authentication failed');
  }
}

function showAuthError(msg) {
  const errorElem = document.getElementById('auth-error-msg');
  if (!errorElem) return;
  errorElem.textContent = msg;
  errorElem.style.display = 'block';
}

// =============================================================================
// 12. TOAST NOTIFICATIONS & HELPERS
// =============================================================================
function showToast(message, type = 'info', duration = 4000) {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.innerHTML = `
    <span class="toast-icon">${type === 'success' ? '✓' : (type === 'error' ? '✕' : 'ℹ')}</span>
    <span class="toast-text">${message}</span>
  `;

  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(-10px)';
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

function formatPrice(val) {
  const num = parseFloat(val);
  if (isNaN(num)) return '--';
  if (num >= 1000) {
    return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  } else if (num >= 1) {
    return num.toFixed(4);
  } else {
    return num.toFixed(6);
  }
}

// =============================================================================
// 13. BOOTSTRAP APPLICATION
// =============================================================================
document.addEventListener('DOMContentLoaded', async () => {
  // 1. Initialize Authentication & Session
  await initAuth();

  // 2. Load Supported Symbols
  await loadSupportedSymbols();

  // 3. Load Initial Live Tickers on Market Watch
  await fetchLiveTickers();

  // 4. Polling fallback for live tickers every 4 seconds (great when WS is not connected)
  state.tickerPollTimer = setInterval(() => {
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
      fetchLiveTickers();
    }
  }, 4000);
});
