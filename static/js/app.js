// ========================================================
// Rdp Sensei - Frontend App Logic
// Multi-Session RDP Management & macOS Windows App Replica
// ========================================================

const state = {
  currentTab: 'favorites', // favorites | devices | apps
  searchQuery: '',
  sortMode: 'name', // name | host | latency
  viewMode: 'grid', // grid | list
  devices: [],
  activeSessions: new Map(), // deviceId -> device
  activeSessionId: null,
  isPinging: false,
};

// Initialize Application
document.addEventListener('DOMContentLoaded', async () => {
  setupNavigation();
  setupToolbar();
  setupSessionBar();
  setupModal();
  setupPasswordPrompt();
  setupPopoutCheck();

  await loadDevices();
  triggerAutoPing();
});

// Navigation Handling
function setupNavigation() {
  const navItems = document.querySelectorAll('.sidebar-nav .nav-item');
  const viewTitle = document.getElementById('viewTitle');

  navItems.forEach(item => {
    item.addEventListener('click', () => {
      navItems.forEach(n => n.classList.remove('active'));
      item.classList.add('active');

      const tab = item.dataset.tab;
      state.currentTab = tab;

      if (tab === 'favorites') viewTitle.textContent = 'Favorites';
      else if (tab === 'devices') viewTitle.textContent = 'Devices';
      else if (tab === 'apps') viewTitle.textContent = 'Apps';

      render();
    });
  });

  // Sidebar toggle
  const sidebar = document.getElementById('sidebar');
  const toggleBtn = document.getElementById('sidebarToggleBtn');
  const showBtn = document.getElementById('sidebarShowBtn');

  if (toggleBtn) {
    toggleBtn.addEventListener('click', () => {
      sidebar.classList.add('collapsed');
      showBtn.style.display = 'flex';
    });
  }

  if (showBtn) {
    showBtn.addEventListener('click', () => {
      sidebar.classList.remove('collapsed');
      showBtn.style.display = 'none';
    });
  }
}

// Toolbar Setup
function setupToolbar() {
  const searchInput = document.getElementById('searchInput');
  const addBtn = document.getElementById('addPcBtn');
  const pingBtn = document.getElementById('pingAllBtn');
  const gridBtn = document.getElementById('viewGridBtn');
  const listBtn = document.getElementById('viewListBtn');

  searchInput.addEventListener('input', e => {
    state.searchQuery = e.target.value.toLowerCase().trim();
    render();
  });

  addBtn.addEventListener('click', () => openDeviceModal());
  pingBtn.addEventListener('click', () => triggerPingAll());

  gridBtn.addEventListener('click', () => {
    state.viewMode = 'grid';
    gridBtn.classList.add('active');
    listBtn.classList.remove('active');
    render();
  });

  listBtn.addEventListener('click', () => {
    state.viewMode = 'list';
    listBtn.classList.add('active');
    gridBtn.classList.remove('active');
    render();
  });

  // Sort Button
  document.getElementById('sortBtn').addEventListener('click', () => {
    if (state.sortMode === 'name') state.sortMode = 'host';
    else if (state.sortMode === 'host') state.sortMode = 'latency';
    else state.sortMode = 'name';
    showToast(`Sorted by: ${state.sortMode.toUpperCase()}`);
    render();
  });
}

// Fetch Devices from Go Backend
async function loadDevices() {
  try {
    const res = await fetch('/api/devices');
    if (!res.ok) throw new Error('Failed to load devices');
    state.devices = await res.json();
    render();
  } catch (err) {
    showToast('Error loading devices: ' + err.message);
  }
}

// Ping all devices concurrently via Go backend
async function triggerPingAll() {
  if (state.isPinging) return;
  state.isPinging = true;
  showToast('Pinging all remote hosts...');
  try {
    const res = await fetch('/api/ping');
    if (res.ok) {
      const results = await res.json();
      results.forEach(r => {
        const d = state.devices.find(x => x.id === r.id);
        if (d) {
          d.status = r.status;
          d.latencyMs = r.latencyMs;
        }
      });
      render();
      showToast('Ping check complete');
    }
  } catch (e) {
    console.error('Ping check error:', e);
  } finally {
    state.isPinging = false;
  }
}

function triggerAutoPing() {
  setTimeout(() => triggerPingAll(), 500);
  setInterval(() => triggerPingAll(), 30000);
}

// Filter and Sort
function getFilteredDevices() {
  let list = [...state.devices];

  // Tab filter
  if (state.currentTab === 'favorites') {
    list = list.filter(d => d.favorite);
  }

  // Search filter
  if (state.searchQuery) {
    list = list.filter(d =>
      d.name.toLowerCase().includes(state.searchQuery) ||
      d.host.toLowerCase().includes(state.searchQuery) ||
      (d.group && d.group.toLowerCase().includes(state.searchQuery)) ||
      (d.username && d.username.toLowerCase().includes(state.searchQuery))
    );
  }

  // Sort
  list.sort((a, b) => {
    if (state.sortMode === 'name') return a.name.localeCompare(b.name);
    if (state.sortMode === 'host') return a.host.localeCompare(b.host);
    if (state.sortMode === 'latency') return (a.latencyMs || 999) - (b.latencyMs || 999);
    return 0;
  });

  return list;
}

// Render Main View
function render() {
  const container = document.getElementById('contentContainer');
  const devices = getFilteredDevices();

  if (state.currentTab === 'favorites') {
    renderFavoritesView(container, devices);
  } else if (state.currentTab === 'devices') {
    renderGroupedView(container, devices);
  } else if (state.currentTab === 'apps') {
    renderAppsView(container);
  }

  updateActiveSessionsBanner();
}

// Wallpaper helper
function getWallpaperClass(name) {
  const n = (name || '').toLowerCase();
  if (n.includes('scada') || n.includes('plc') || n.includes('dinamo')) return 'wallpaper-scada';
  if (n.includes('bfs') || n.includes('mountain')) return 'wallpaper-mountain';
  if (n.includes('host') || n.includes('server') || n.includes('dc')) return 'wallpaper-terminal';
  if (n.includes('oem') || n.includes('zirve') || n.includes('burtek')) return 'wallpaper-white';
  return 'wallpaper-windows';
}

// Render Favorites
function renderFavoritesView(container, devices) {
  if (devices.length === 0) {
    container.innerHTML = `
      <div style="text-align: center; padding: 60px 20px; color: var(--text-muted);">
        <p style="font-size: 16px; margin-bottom: 8px;">No favorite PCs found</p>
        <p style="font-size: 13px;">Click the star icon ⭐ on any PC card to add it to Favorites.</p>
      </div>
    `;
    return;
  }

  let html = `<div class="cards-grid">`;
  devices.forEach(d => {
    html += createCardHtml(d);
  });
  html += `</div>`;
  container.innerHTML = html;
  bindCardEvents(container);
}

// Render Grouped Accordion View
function renderGroupedView(container, devices) {
  const groupsMap = new Map();
  const defaultOrder = ['arsen', 'bfs', 'byb', 'ev', 'hk', 'ideteks', 'oem', 'okks', 'Saved Devices', 'sener'];
  defaultOrder.forEach(g => groupsMap.set(g, []));

  devices.forEach(d => {
    const grp = d.group || 'Saved Devices';
    if (!groupsMap.has(grp)) groupsMap.set(grp, []);
    groupsMap.get(grp).push(d);
  });

  let html = `<div class="groups-container">`;

  for (const [groupName, groupDevices] of groupsMap.entries()) {
    if (state.searchQuery && groupDevices.length === 0) continue;

    const count = groupDevices.length;
    const isCollapsed = count === 0;

    html += `
      <div class="group-section ${isCollapsed ? 'collapsed' : ''}" data-group="${groupName}">
        <div class="group-header" onclick="toggleGroupCollapse(this)">
          <svg class="group-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
          <span>${groupName}</span>
          ${count > 0 ? `<span class="group-count">(${count})</span>` : ''}
        </div>
        <div class="group-content">
          ${count > 0 ? `
            <div class="cards-grid">
              ${groupDevices.map(d => createCardHtml(d)).join('')}
            </div>
          ` : `<div style="font-size: 12px; color: var(--text-muted); padding-left: 24px;">No devices in this group</div>`}
        </div>
      </div>
    `;
  }

  html += `</div>`;
  container.innerHTML = html;
  bindCardEvents(container);
}

// Render Apps View
function renderAppsView(container) {
  container.innerHTML = `
    <div style="text-align: center; padding: 60px 20px; color: var(--text-muted);">
      <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="margin-bottom: 12px; opacity: 0.6;">
        <rect x="3" y="3" width="7" height="7"></rect>
        <rect x="14" y="3" width="7" height="7"></rect>
        <rect x="14" y="14" width="7" height="7"></rect>
        <rect x="3" y="14" width="7" height="7"></rect>
      </svg>
      <h3 style="color: #fff; font-size: 16px; margin-bottom: 6px;">Remote Apps</h3>
      <p style="font-size: 13px;">Publish individual virtual apps or Windows tools from your RDP servers.</p>
    </div>
  `;
}

// Card HTML
function createCardHtml(d) {
  const wpClass = getWallpaperClass(d.name);
  const isOnline = d.status === 'online';
  const latency = d.latencyMs ? `${d.latencyMs}ms` : '3389';
  const subtitle = d.username || d.host;
  const isRunning = state.activeSessions.has(d.id);

  return `
    <div class="pc-card ${isRunning ? 'active-running' : ''}" data-id="${d.id}">
      <div class="card-thumbnail ${wpClass}">
        <div class="card-top-row">
          <span class="badge-pc">PC Connection</span>
          <div class="card-top-actions">
            ${isRunning ? `<span class="card-running-pill">🟢 ACTIVE</span>` : ''}
            <span class="ping-pill ${isOnline ? 'online' : 'offline'}">
              <span class="ping-dot"></span>
              ${isOnline ? latency : 'Off'}
            </span>
            <button class="fav-star-btn ${d.favorite ? 'active' : ''}" onclick="toggleFav(event, '${d.id}')" title="Toggle Favorite">
              ★
            </button>
          </div>
        </div>

        ${wpClass === 'wallpaper-white' ? `
          <div class="mock-window-frame">
            <div class="mock-window-header">
              <span class="mock-window-dot"></span>
              <span class="mock-window-dot"></span>
            </div>
          </div>
        ` : ''}

        ${wpClass === 'wallpaper-terminal' ? `
          <div class="mock-terminal-window"></div>
        ` : ''}

        <div class="card-bottom-info">
          <div class="pc-name" title="${d.name}">${d.name}</div>
          <div class="pc-subtitle" title="${subtitle}">${subtitle}</div>
        </div>

        <div class="card-hover-actions">
          <button class="hover-action-btn connect" onclick="startConnect(event, '${d.id}')" title="${isRunning ? 'Switch to Session' : 'Connect to PC'}">
            ${isRunning ? '👁️' : '▶'}
          </button>
          <button class="hover-action-btn" onclick="editDevice(event, '${d.id}')" title="Edit PC">
            ✏
          </button>
          <button class="hover-action-btn" onclick="deleteDevice(event, '${d.id}')" title="Delete PC">
            ✕
          </button>
        </div>
      </div>
    </div>
  `;
}

// Bind Card Click Events
function bindCardEvents(container) {
  container.querySelectorAll('.pc-card').forEach(card => {
    card.addEventListener('click', (e) => {
      if (e.target.closest('.fav-star-btn') || e.target.closest('.card-hover-actions')) {
        return;
      }
      const id = card.dataset.id;
      initiateConnection(id);
    });
  });
}

function toggleGroupCollapse(header) {
  const section = header.closest('.group-section');
  section.classList.toggle('collapsed');
}

async function toggleFav(event, id) {
  event.stopPropagation();
  const d = state.devices.find(x => x.id === id);
  if (!d) return;

  d.favorite = !d.favorite;
  try {
    await fetch(`/api/devices/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(d),
    });
    render();
    showToast(d.favorite ? `Added to Favorites: ${d.name}` : `Removed from Favorites: ${d.name}`);
  } catch (err) {
    console.error('Failed to update favorite:', err);
  }
}

function startConnect(event, id) {
  event.stopPropagation();
  initiateConnection(id);
}

// Start connection logic
function initiateConnection(id) {
  const d = state.devices.find(x => x.id === id);
  if (!d) return;

  // If already active, switch immediately!
  if (state.activeSessions.has(id)) {
    switchToSession(id);
    return;
  }

  // If credentials missing, prompt
  if (!d.password) {
    openPasswordModal(d);
  } else {
    connectToDevice(d);
  }
}

// Credentials modal
function setupPasswordPrompt() {
  const connectBtn = document.getElementById('promptConnectBtn');
  const passInput = document.getElementById('promptPassword');

  const submitPrompt = () => {
    const id = document.getElementById('promptDeviceId').value;
    const d = state.devices.find(x => x.id === id);
    if (!d) return;

    const username = document.getElementById('promptUsername').value.trim();
    const password = document.getElementById('promptPassword').value;
    const remember = document.getElementById('promptSavePassword').checked;

    if (!password) {
      alert('Lütfen şifrenizi girin.');
      return;
    }

    d.username = username || d.username;
    d.password = password;

    if (remember) {
      fetch(`/api/devices/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(d),
      }).catch(err => console.warn('Could not save password:', err));
    }

    closePasswordModal();
    connectToDevice(d);
  };

  connectBtn.addEventListener('click', submitPrompt);
  passInput.addEventListener('keydown', e => {
    if (e.key === 'Enter') submitPrompt();
  });

  document.getElementById('sessionRetryBtn').addEventListener('click', () => {
    document.getElementById('sessionErrorOverlay').style.display = 'none';
    const activeDev = state.activeSessions.get(state.activeSessionId);
    if (activeDev) {
      openPasswordModal(activeDev);
    }
  });
}

function openPasswordModal(device) {
  document.getElementById('promptDeviceId').value = device.id;
  document.getElementById('promptPcName').textContent = device.name;
  document.getElementById('promptPcHost').textContent = device.host;
  document.getElementById('promptUsername').value = device.username || '';
  const passInput = document.getElementById('promptPassword');
  passInput.value = device.password || '';

  document.getElementById('passwordModal').classList.add('active');
  setTimeout(() => passInput.focus(), 100);
}

function closePasswordModal() {
  document.getElementById('passwordModal').classList.remove('active');
}

// Launch / Add In-App RDP Session (Multi-Session!)
async function connectToDevice(d) {
  state.activeSessions.set(d.id, d);
  state.activeSessionId = d.id;

  const container = document.getElementById('sessionPanesContainer');
  let pane = document.getElementById(`pane_${d.id}`);
  if (!pane) {
    pane = document.createElement('div');
    pane.className = 'session-pane';
    pane.id = `pane_${d.id}`;
    pane.innerHTML = `
      <div class="canvas-viewport fit-screen" id="viewport_${d.id}">
        <canvas id="canvas_${d.id}" tabindex="0"></canvas>
      </div>
    `;
    container.appendChild(pane);
  }

  const canvas = document.getElementById(`canvas_${d.id}`);
  const viewport = document.getElementById(`viewport_${d.id}`);

  // Show session view
  document.getElementById('rdpSessionView').classList.add('active');
  document.getElementById('sessionErrorOverlay').style.display = 'none';

  // Record connection touch
  fetch(`/api/devices/${d.id}/touch`, { method: 'POST' }).catch(() => {});

  // Setup callbacks
  window.rdpEngine.onSessionStatusChange = (sid, status, msg) => {
    renderSessionTabs();
    if (sid === state.activeSessionId) {
      const badge = document.getElementById('sessionStatusBadge');
      if (status === 'connected') {
        badge.textContent = `Connected (${d.latencyMs ? d.latencyMs + 'ms' : 'Active'})`;
        badge.style.color = 'var(--online-green)';
        document.getElementById('sessionErrorOverlay').style.display = 'none';
        showToast(`Connected to ${d.name}`);
      } else if (status === 'disconnected') {
        badge.textContent = msg || 'Disconnected';
        badge.style.color = 'var(--offline-red)';
      } else if (status === 'error') {
        badge.textContent = 'Error';
        badge.style.color = 'var(--offline-red)';
        let friendly = msg;
        if (msg.includes('tls: access denied')) {
          friendly = 'Windows NLA kimlik doğrulaması reddedildi. Kullanıcı adı veya şifre hatalı olabilir.';
        } else if (msg.includes('connection refused') || msg.includes('timeout')) {
          friendly = `${d.host}:3389 portuna ulaşılamadı. Ağ bağlantınızı kontrol edin.`;
        }
        document.getElementById('sessionErrorTitle').textContent = 'Bağlantı Başarısız';
        document.getElementById('sessionErrorMessage').textContent = friendly;
        document.getElementById('sessionErrorOverlay').style.display = 'flex';
      }
    }
  };

  window.rdpEngine.onSessionClose = (sid) => {
    renderSessionTabs();
    updateActiveSessionsBanner();
    render();
  };

  try {
    await window.rdpEngine.connect(d, canvas, viewport);
    switchToSession(d.id);
  } catch (err) {
    console.error('Connect failed:', err);
    window.rdpEngine.handleError(d.id, err.message);
  }

  renderSessionTabs();
  updateActiveSessionsBanner();
  render();
}

// Switch between open sessions in the Tab Bar
function switchToSession(id) {
  if (!state.activeSessions.has(id)) return;
  state.activeSessionId = id;
  window.rdpEngine.setActiveSession(id);

  // Show active pane
  const container = document.getElementById('sessionPanesContainer');
  container.querySelectorAll('.session-pane').forEach(p => {
    p.classList.toggle('active', p.id === `pane_${id}`);
  });

  const sessionView = document.getElementById('rdpSessionView');
  sessionView.classList.add('active');

  const d = state.activeSessions.get(id);
  if (d) {
    document.getElementById('sessionPcName').textContent = `${d.name} (${d.host})`;
    const s = window.rdpEngine.sessions.get(id);
    const badge = document.getElementById('sessionStatusBadge');
    if (s && s.connected) {
      badge.textContent = `Connected (${d.latencyMs ? d.latencyMs + 'ms' : 'Active'})`;
      badge.style.color = 'var(--online-green)';
    } else {
      badge.textContent = s ? s.statusMsg : 'Connecting...';
      badge.style.color = '#ffbd2e';
    }
  }

  renderSessionTabs();
  updateActiveSessionsBanner();
}

// Disconnect a specific session
function disconnectSession(id) {
  window.rdpEngine.disconnect(id);
  state.activeSessions.delete(id);

  const pane = document.getElementById(`pane_${id}`);
  if (pane) pane.remove();

  if (state.activeSessions.size === 0) {
    state.activeSessionId = null;
    document.getElementById('rdpSessionView').classList.remove('active');
    document.getElementById('sessionErrorOverlay').style.display = 'none';
  } else {
    const nextId = state.activeSessions.keys().next().value;
    switchToSession(nextId);
  }

  renderSessionTabs();
  updateActiveSessionsBanner();
  render();
}

// Render dynamic tabs in top Session Tab Bar
function renderSessionTabs() {
  const tabsList = document.getElementById('sessionTabsList');
  if (!tabsList) return;
  tabsList.innerHTML = '';

  const activeId = state.activeSessionId;

  for (const [id, d] of state.activeSessions.entries()) {
    const s = window.rdpEngine.sessions.get(id);
    const tab = document.createElement('div');
    const isAct = id === activeId;
    const isConn = s && s.connected;
    const isErr = s && s.status === 'error';

    tab.className = `session-tab ${isAct ? 'active' : ''} ${isConn ? 'connected' : ''} ${isErr ? 'error' : ''}`;
    tab.innerHTML = `
      <span class="session-tab-dot"></span>
      <span class="session-tab-title" title="${d.name} (${d.host})">${d.name}</span>
      <button class="session-tab-close" title="Disconnect ${d.name}">&times;</button>
    `;

    tab.addEventListener('click', (e) => {
      if (e.target.closest('.session-tab-close')) return;
      switchToSession(id);
    });

    tab.querySelector('.session-tab-close').addEventListener('click', (e) => {
      e.stopPropagation();
      disconnectSession(id);
    });

    tabsList.appendChild(tab);
  }
}

// Update Active Sessions Dock Banner in Catalog view
function updateActiveSessionsBanner() {
  const banner = document.getElementById('activeSessionsBanner');
  const countEl = document.getElementById('activeSessionsCount');
  const chipsEl = document.getElementById('activeSessionsChips');
  if (!banner) return;

  const count = state.activeSessions.size;
  if (count === 0) {
    banner.style.display = 'none';
    return;
  }

  banner.style.display = 'flex';
  countEl.textContent = `${count} Active Connection${count > 1 ? 's' : ''}`;

  chipsEl.innerHTML = '';
  for (const [id, d] of state.activeSessions.entries()) {
    const chip = document.createElement('span');
    chip.className = 'session-chip';
    chip.textContent = d.name;
    chip.title = `Switch to ${d.name} (${d.host})`;
    chip.addEventListener('click', () => switchToSession(id));
    chipsEl.appendChild(chip);
  }
}

function closeSessionView() {
  // Disconnect active session
  if (state.activeSessionId) {
    disconnectSession(state.activeSessionId);
  }
}

// Setup Session Bar Controls
function setupSessionBar() {
  // "PCs" Catalog button in Tab Bar (leaves sessions running in background!)
  document.getElementById('sessionCatalogBtn').addEventListener('click', () => {
    document.getElementById('rdpSessionView').classList.remove('active');
    updateActiveSessionsBanner();
    render();
    showToast('Sessions remain active in background. Click banner or card to resume.');
  });

  // "+ Connect PC" button in Tab Bar
  document.getElementById('sessionAddConnectionBtn').addEventListener('click', () => {
    document.getElementById('rdpSessionView').classList.remove('active');
    updateActiveSessionsBanner();
    render();
    showToast('Select any PC from the catalog to connect concurrently.');
  });

  // Resume Sessions banner button
  document.getElementById('resumeSessionBtn').addEventListener('click', () => {
    const id = state.activeSessionId || state.activeSessions.keys().next().value;
    if (id) switchToSession(id);
  });

  // Disconnect button in floating action bar
  document.getElementById('sessionDisconnectBtn').addEventListener('click', () => {
    if (state.activeSessionId) {
      disconnectSession(state.activeSessionId);
    }
  });

  // Alt+Tab quick switch button
  document.getElementById('sessionAltTabBtn').addEventListener('click', () => {
    window.rdpEngine.sendAltTab();
    showToast('Sent Alt+Tab ⇥');
  });

  // CAD quick button
  document.getElementById('sessionCadBtn').addEventListener('click', () => {
    window.rdpEngine.sendCtrlAltDel();
    showToast('Sent Ctrl+Alt+Del');
  });

  // Windows Key
  document.getElementById('sessionWinKeyBtn').addEventListener('click', () => {
    window.rdpEngine.sendWindowsKey();
    showToast('Sent Windows Key');
  });

  // Touch Bar toggle button
  document.getElementById('sessionTouchbarBtn').addEventListener('click', () => {
    const tb = document.getElementById('touchbarStrip');
    const isHidden = tb.classList.toggle('hidden');
    const tbBtn = document.getElementById('sessionTouchbarBtn');
    tbBtn.classList.toggle('active', !isHidden);
    showToast(isHidden ? 'Touch Bar hidden' : 'Touch Bar active');
    setTimeout(() => window.rdpEngine.updateLayouts(), 260);
  });

  // Fit to screen / 1:1 pixel toggle button
  document.getElementById('sessionFitBtn').addEventListener('click', () => {
    const active = window.rdpEngine.getActiveSession();
    if (!active || !active.viewport) return;
    const isFit = active.viewport.classList.toggle('fit-screen');
    const fitBtn = document.getElementById('sessionFitBtn');
    fitBtn.querySelector('span').textContent = isFit ? '⛶ Fit Mode' : '1:1 Pixels';
    window.rdpEngine.updateLayout(active.id);
    showToast(isFit ? 'Display: Fit to Window' : 'Display: 100% Native Resolution');
  });

  // Pop-out standalone window button
  document.getElementById('sessionPopoutBtn').addEventListener('click', () => {
    const active = window.rdpEngine.getActiveSession();
    if (!active) return;
    const url = `${window.location.origin}/?popout=${active.id}`;
    window.open(url, `rdp_${active.id}`, 'width=1280,height=820,menubar=no,toolbar=no,location=no');
    showToast(`Popped out ${active.device.name} into dedicated window`);
  });

  // Clipboard sync
  document.getElementById('sessionClipboardBtn').addEventListener('click', () => {
    window.rdpEngine.syncLocalClipboard();
    showToast('Clipboard synchronized');
  });

  // Fullscreen
  document.getElementById('sessionFullscreenBtn').addEventListener('click', () => {
    if (!document.fullscreenElement) {
      document.getElementById('rdpSessionView').requestFullscreen().catch(() => {});
    } else {
      document.exitFullscreen().catch(() => {});
    }
  });
}

// Auto-connect if launched with ?popout=device_id
function setupPopoutCheck() {
  const params = new URLSearchParams(window.location.search);
  const popoutId = params.get('popout');
  if (popoutId) {
    const checkInterval = setInterval(() => {
      const d = state.devices.find(x => x.id === popoutId);
      if (d) {
        clearInterval(checkInterval);
        initiateConnection(d.id);
      }
    }, 200);
    setTimeout(() => clearInterval(checkInterval), 5000);
  }
}

// Setup Add/Edit Device Modal
function setupModal() {
  const modal = document.getElementById('deviceModal');
  const closeBtn = document.getElementById('modalCloseBtn');
  const cancelBtn = document.getElementById('modalCancelBtn');
  const saveBtn = document.getElementById('modalSaveBtn');

  const close = () => modal.classList.remove('active');
  closeBtn.addEventListener('click', close);
  cancelBtn.addEventListener('click', close);

  saveBtn.addEventListener('click', async () => {
    const id = document.getElementById('devId').value;
    const dev = {
      name: document.getElementById('devName').value.trim(),
      host: document.getElementById('devHost').value.trim(),
      port: parseInt(document.getElementById('devPort').value) || 3389,
      group: document.getElementById('devGroup').value.trim() || 'Saved Devices',
      username: document.getElementById('devUsername').value.trim(),
      password: document.getElementById('devPassword').value,
      domain: document.getElementById('devDomain').value.trim(),
      width: parseInt(document.getElementById('devWidth').value) || 1440,
      height: parseInt(document.getElementById('devHeight').value) || 900,
      swapAltMeta: document.getElementById('devSwapAltMeta').checked,
      enableAudio: document.getElementById('devEnableAudio').checked,
      favorite: document.getElementById('devFavorite').checked,
    };

    if (!dev.name || !dev.host) {
      alert('PC Adı ve IP / Host adresi zorunludur!');
      return;
    }

    try {
      const url = id ? `/api/devices/${id}` : '/api/devices';
      const method = id ? 'PUT' : 'POST';
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(dev),
      });

      if (!res.ok) throw new Error('Save failed');
      close();
      await loadDevices();
      showToast(id ? 'PC güncellendi' : 'Yeni PC eklendi');
    } catch (e) {
      alert('Hata: ' + e.message);
    }
  });
}

function openDeviceModal(device = null) {
  const modal = document.getElementById('deviceModal');
  document.getElementById('modalTitle').textContent = device ? 'Edit PC Connection' : 'Add PC Connection';
  document.getElementById('devId').value = device ? device.id : '';
  document.getElementById('devName').value = device ? device.name : '';
  document.getElementById('devHost').value = device ? device.host : '';
  document.getElementById('devPort').value = device ? (device.port || 3389) : 3389;
  document.getElementById('devGroup').value = device ? (device.group || 'Saved Devices') : 'Saved Devices';
  document.getElementById('devUsername').value = device ? (device.username || '') : '';
  document.getElementById('devPassword').value = device ? (device.password || '') : '';
  document.getElementById('devDomain').value = device ? (device.domain || '') : '';
  document.getElementById('devWidth').value = device ? (device.width || 1440) : 1440;
  document.getElementById('devHeight').value = device ? (device.height || 900) : 900;
  document.getElementById('devSwapAltMeta').checked = device ? (device.swapAltMeta ?? false) : false;
  document.getElementById('devEnableAudio').checked = device ? (device.enableAudio ?? true) : true;
  document.getElementById('devFavorite').checked = device ? !!device.favorite : false;

  modal.classList.add('active');
}

function editDevice(event, id) {
  event.stopPropagation();
  const d = state.devices.find(x => x.id === id);
  if (d) openDeviceModal(d);
}

async function deleteDevice(event, id) {
  event.stopPropagation();
  const d = state.devices.find(x => x.id === id);
  if (!d) return;
  if (!confirm(`"${d.name}" bağlantısını silmek istediğinize emin misiniz?`)) return;

  try {
    const res = await fetch(`/api/devices/${id}`, { method: 'DELETE' });
    if (res.ok) {
      await loadDevices();
      showToast('Bağlantı silindi');
    }
  } catch (e) {
    showToast('Silme işlemi başarısız');
  }
}

// Toast Notifications
function showToast(message) {
  const container = document.getElementById('toastContainer');
  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.innerHTML = `
    <span style="color: var(--accent);">●</span>
    <span>${message}</span>
  `;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.25s ease';
    setTimeout(() => toast.remove(), 250);
  }, 2500);
}
