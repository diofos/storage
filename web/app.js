/**
 * DIOFOS Operations Console - Client Logic
 * Handles P2P state synchronization, REST API integration,
 * chunk topology visualization, file streaming, and transfer tracking.
 */

// Global State
const state = {
  apiBase: window.location.origin.includes('http') ? window.location.origin : 'http://localhost:8070',
  currentTab: 'screen-dashboard',
  status: null,
  storage: null,
  nodes: [],
  files: [],
  transfers: [],
  searchQuery: '',
  selectedFileForModal: null,
  pollTimer: null,
};

// Utilities
function formatBytes(bytes) {
  if (!bytes || isNaN(bytes) || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDate(isoStr) {
  if (!isoStr) return '--';
  const d = new Date(isoStr);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function truncateMiddle(str, frontLen = 8, backLen = 6) {
  if (!str || str.length <= frontLen + backLen) return str || '--';
  return str.substring(0, frontLen) + '...' + str.substring(str.length - backLen);
}

function getFileIcon(filename) {
  const ext = (filename.split('.').pop() || '').toLowerCase();
  if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'mp4', 'mov', 'mp3'].includes(ext)) {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>`;
  }
  if (['zip', 'tar', 'gz', '7z', 'rar'].includes(ext)) {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="21 8 21 21 3 21 3 8"></polyline><rect x="1" y="3" width="22" height="5"></rect><line x1="10" y1="12" x2="14" y2="12"></line></svg>`;
  }
  if (['go', 'js', 'ts', 'py', 'json', 'yaml', 'html', 'css', 'c', 'cpp'].includes(ext)) {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>`;
  }
  return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>`;
}

function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container') || document.getElementById('toast-hub');
  if (!container) return;
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.2s ease';
    setTimeout(() => toast.remove(), 250);
  }, 4000);
}

function copyText(text, label = 'Copied to clipboard!') {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => showToast(label, 'success'));
  } else {
    const input = document.createElement('input');
    input.value = text;
    document.body.appendChild(input);
    input.select();
    document.execCommand('copy');
    document.body.removeChild(input);
    showToast(label, 'success');
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

// REST API Request Helper
async function fetchAPI(endpoint, options = {}) {
  const url = `${state.apiBase}${endpoint}`;
  try {
    const res = await fetch(url, options);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    return await res.json();
  } catch (err) {
    console.error(`API Error [${endpoint}]:`, err);
    throw err;
  }
}

// Apple Themes Management
const THEME_NAMES = {
  'apple-space-black': { label: 'Space Black', swatch: 'swatch-space-black' },
  'apple-titanium': { label: 'Titanium', swatch: 'swatch-titanium' },
  'apple-midnight': { label: 'Midnight', swatch: 'swatch-midnight' },
  'apple-silver-light': { label: 'Silver Light', swatch: 'swatch-silver' },
};

function applyTheme(themeKey) {
  if (!THEME_NAMES[themeKey]) themeKey = 'apple-space-black';
  document.documentElement.setAttribute('data-theme', themeKey);
  try { localStorage.setItem('diofos-theme', themeKey); } catch (e) {}

  const labelEl = document.getElementById('active-theme-label');
  const swatchEl = document.getElementById('active-theme-swatch');
  const meta = THEME_NAMES[themeKey];
  if (labelEl) labelEl.textContent = meta.label;
  if (swatchEl) swatchEl.className = `theme-swatch ${meta.swatch}`;

  document.querySelectorAll('[data-theme]').forEach(btn => {
    if (btn.classList.contains('theme-option') || btn.classList.contains('mobile-theme-btn')) {
      if (btn.dataset.theme === themeKey) {
        btn.classList.add('active');
      } else {
        btn.classList.remove('active');
      }
    }
  });
}

function openMobileMenu() {
  document.getElementById('mobile-nav-drawer')?.classList.add('active');
}

function closeMobileMenu() {
  document.getElementById('mobile-nav-drawer')?.classList.remove('active');
}

// Tab Switching (Unified for Desktop Horizontal and Mobile Vertical Nav)
function switchTab(screenId) {
  state.currentTab = screenId;
  const tabs = document.querySelectorAll('.nav-tab');
  tabs.forEach(t => {
    if (t.dataset.screen === screenId) {
      t.classList.add('active');
    } else {
      t.classList.remove('active');
    }
  });

  const mobileItems = document.querySelectorAll('.mobile-nav-item');
  mobileItems.forEach(m => {
    if (m.dataset.screen === screenId) {
      m.classList.add('active');
    } else {
      m.classList.remove('active');
    }
  });

  const screens = document.querySelectorAll('.screen-view');
  screens.forEach(s => {
    if (s.id === screenId) {
      s.classList.add('active');
    } else {
      s.classList.remove('active');
    }
  });

  closeMobileMenu();
}
window.switchTab = switchTab;

// Data Fetchers & Mesh Synchronization
async function syncMeshState() {
  try {
    const [statusData, storageData, nodesData, filesData, transfersData] = await Promise.all([
      fetchAPI('/api/v1/status').catch(() => null),
      fetchAPI('/api/v1/storage').catch(() => null),
      fetchAPI('/api/v1/nodes').catch(() => ({ nodes: [] })),
      fetchAPI('/api/v1/files').catch(() => ({ files: [] })),
      fetchAPI('/api/v1/transfers').catch(() => ({ transfers: [] })),
    ]);

    if (statusData) state.status = statusData;
    if (storageData) state.storage = storageData;
    state.nodes = (nodesData && nodesData.nodes) || [];
    state.files = (filesData && filesData.files) || [];
    state.transfers = (transfersData && transfersData.transfers) || [];

    renderHeader();
    renderDashboard();
    renderFiles();
    renderTransfers();
    renderDevices();
    renderSettings();
  } catch (err) {
    console.warn('Sync failed:', err);
  }
}

// Header & Navigation Rendering
function renderHeader() {
  if (state.status) {
    const nameEl = document.getElementById('header-node-name');
    const portEl = document.getElementById('header-node-port');
    if (nameEl) nameEl.textContent = state.status.display_name || 'Authority-Node';
    if (portEl) {
      const parts = (state.status.listen_address || '').split(':');
      portEl.textContent = parts.length > 1 ? `:${parts[parts.length - 1]}` : ':7001';
    }

    const lanAddr = state.status.lan_address || state.status.listen_address || '127.0.0.1:7001';
    const shareableEl = document.getElementById('shareable-lan-address');
    const modalLanEl = document.getElementById('modal-my-lan-address');
    if (shareableEl) shareableEl.textContent = lanAddr;
    if (modalLanEl) modalLanEl.textContent = lanAddr;

    const footerNetEl = document.getElementById('footer-network-id');
    if (footerNetEl) footerNetEl.textContent = state.status.network_id || 'default-mesh';
  }

  // Nav badges
  const filesCountBadge = document.getElementById('nav-files-count');
  const mobileFilesBadge = document.getElementById('mobile-nav-files-count');
  if (filesCountBadge) filesCountBadge.textContent = state.files.length;
  if (mobileFilesBadge) mobileFilesBadge.textContent = state.files.length;

  const nodesCountBadge = document.getElementById('nav-nodes-count');
  const mobileNodesBadge = document.getElementById('mobile-nav-nodes-count');
  if (nodesCountBadge) nodesCountBadge.textContent = state.nodes.length || 1;
  if (mobileNodesBadge) mobileNodesBadge.textContent = state.nodes.length || 1;

  const activeTransfers = state.transfers.filter(t => t.status === 'in_progress').length;
  const transfersBadge = document.getElementById('nav-transfers-count');
  const mobileTransfersBadge = document.getElementById('mobile-nav-transfers-count');
  if (transfersBadge) transfersBadge.textContent = activeTransfers;
  if (mobileTransfersBadge) mobileTransfersBadge.textContent = activeTransfers;

  const mobileNodeName = document.getElementById('mobile-header-node-name');
  if (mobileNodeName && state.status) {
    mobileNodeName.textContent = state.status.display_name || 'DIOFOS Node';
  }
}

// Dashboard View Rendering
function renderDashboard() {
  if (!state.storage || !state.status) return;

  // Resilient Volume Metric
  const usableEl = document.getElementById('metric-usable-cap');
  const usedEl = document.getElementById('metric-used-space');
  const rawEl = document.getElementById('metric-raw-cap');
  const fillEl = document.getElementById('metric-capacity-fill');
  const repBadge = document.getElementById('metric-rep-factor');

  if (usableEl) usableEl.textContent = formatBytes(state.storage.usable_capacity_bytes);
  if (usedEl) usedEl.textContent = `Used: ${formatBytes(state.storage.used_bytes)}`;
  if (rawEl) rawEl.textContent = `Raw: ${formatBytes(state.storage.raw_capacity_bytes)}`;
  if (repBadge) repBadge.textContent = `${state.storage.replication_factor}x Redundancy`;

  if (fillEl && state.storage.raw_capacity_bytes > 0) {
    const pct = Math.min(100, Math.round((state.storage.used_bytes / state.storage.raw_capacity_bytes) * 100));
    fillEl.style.width = `${pct}%`;
  }

  // Active Peers Metric
  const onlineNodesEl = document.getElementById('metric-online-nodes');
  const totalNodesEl = document.getElementById('metric-total-nodes');
  if (onlineNodesEl) onlineNodesEl.textContent = state.storage.nodes_online;
  if (totalNodesEl) totalNodesEl.textContent = state.storage.nodes_total;

  // Objects & Chunks Metric
  const filesCountEl = document.getElementById('metric-files-count');
  const chunksCountEl = document.getElementById('metric-chunks-count');
  if (filesCountEl) filesCountEl.textContent = state.files.length;
  if (chunksCountEl) {
    const totalChunks = state.files.reduce((acc, f) => acc + (f.chunks ? f.chunks.length : 0), 0);
    chunksCountEl.textContent = totalChunks;
  }

  // Active Transfers Metric
  const activeTransfersEl = document.getElementById('metric-active-transfers');
  if (activeTransfersEl) {
    const active = state.transfers.filter(t => t.status === 'in_progress').length;
    activeTransfersEl.textContent = active;
  }

  // Dashboard Recent Files Table (first 5)
  const tbody = document.getElementById('dash-files-table-body') || document.getElementById('dash-recent-files-tbody');
  if (tbody) {
    if (state.files.length === 0) {
      tbody.innerHTML = `
        <tr>
          <td colspan="4" class="empty-placeholder">
            <div class="empty-icon">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
            </div>
            <p>No distributed objects currently stored. Drop a file to slice and replicate.</p>
          </td>
        </tr>`;
    } else {
      const recent = state.files.slice(-5).reverse();
      tbody.innerHTML = recent.map(file => `
        <tr>
          <td>
            <div style="display: flex; align-items: center; gap: 8px;">
              <span style="color: var(--text-muted);">${getFileIcon(file.file_name)}</span>
              <span style="font-weight: 500; color: var(--text-primary);">${escapeHtml(file.file_name)}</span>
            </div>
          </td>
          <td class="tabular-nums font-mono">${formatBytes(file.size_bytes)}</td>
          <td><span class="badge badge-neutral">${file.chunks ? file.chunks.length : 0} chunks</span></td>
          <td class="tabular-nums font-mono text-muted" title="${file.content_hash || file.file_id}">
            ${truncateMiddle(file.content_hash || file.file_id, 8, 6)}
          </td>
        </tr>
      `).join('');
    }
  }

  // Dashboard Mini Devices List
  const devList = document.getElementById('dash-devices-list') || document.getElementById('dash-device-mini-list');
  if (devList) {
    if (state.nodes.length === 0) {
      devList.innerHTML = `<div class="empty-placeholder" style="padding: 16px;"><p>No remote peers connected.</p></div>`;
    } else {
      devList.innerHTML = state.nodes.map(n => {
        const isLocal = state.status && n.node_id === state.status.node_id;
        const isOnline = n.status === 'online';
        return `
          <div class="device-mini-item">
            <div class="device-mini-info">
              <span class="status-beacon ${isOnline ? 'beacon-active' : 'beacon-offline'}" style="color: ${isOnline ? 'var(--color-healthy)' : 'var(--color-danger)'};"></span>
              <div>
                <div class="device-mini-name">${escapeHtml(n.display_name || 'Node Agent')} ${isLocal ? '<span class="badge badge-healthy" style="font-size: 10px; margin-left: 4px;">Local Authority</span>' : ''}</div>
                <div class="device-mini-addr tabular-nums font-mono">${escapeHtml(n.address)}</div>
              </div>
            </div>
            <div class="device-mini-status">
              <span class="tabular-nums font-mono text-muted">${formatBytes(n.used_bytes || 0)}</span>
            </div>
          </div>
        `;
      }).join('');
    }
  }
}

// Shared Files View Rendering
function renderFiles() {
  const tbody = document.getElementById('files-table-body') || document.getElementById('files-list-tbody');
  if (!tbody) return;

  let filtered = state.files.slice();

  // Search filter
  if (state.searchQuery) {
    const q = state.searchQuery.toLowerCase();
    filtered = filtered.filter(f =>
      f.file_name.toLowerCase().includes(q) ||
      (f.logical_path && f.logical_path.toLowerCase().includes(q)) ||
      (f.file_id && f.file_id.toLowerCase().includes(q)) ||
      (f.content_hash && f.content_hash.toLowerCase().includes(q))
    );
  }

  if (filtered.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="7" class="empty-placeholder">
          <div class="empty-icon">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
          </div>
          <p>${state.searchQuery ? 'No stored files match your search filter.' : 'No files in the distributed mesh yet. Drop a file above to slice and distribute.'}</p>
        </td>
      </tr>`;
    return;
  }

  tbody.innerHTML = filtered.map(file => `
    <tr>
      <td>
        <div style="display: flex; align-items: center; gap: 8px;">
          <span style="color: var(--text-muted);">${getFileIcon(file.file_name)}</span>
          <span style="font-weight: 500; color: var(--text-primary);">${escapeHtml(file.file_name)}</span>
        </div>
      </td>
      <td class="tabular-nums font-mono text-muted">${escapeHtml(file.logical_path || '/' + file.file_name)}</td>
      <td class="tabular-nums font-mono">${formatBytes(file.size_bytes)}</td>
      <td><span class="badge badge-neutral">${file.chunks ? file.chunks.length : 0} chunks</span></td>
      <td class="tabular-nums font-mono text-muted" title="${file.content_hash || file.file_id}">
        ${truncateMiddle(file.content_hash || file.file_id, 10, 8)}
      </td>
      <td class="tabular-nums font-mono text-muted">${formatDate(file.created_at)}</td>
      <td style="text-align: right;">
        <div class="table-actions" style="justify-content: flex-end;">
          <button class="btn btn-secondary btn-xs btn-file-inspect" data-id="${file.file_id}" title="Inspect Chunk Topology">Inspect</button>
          <button class="btn btn-primary btn-xs btn-file-download" data-id="${file.file_id}" data-name="${escapeHtml(file.file_name)}" title="Download and verify SHA-256">Download</button>
          <button class="btn btn-danger-ghost btn-xs btn-file-delete" data-id="${file.file_id}" data-name="${escapeHtml(file.file_name)}" title="Delete from Mesh">Delete</button>
        </div>
      </td>
    </tr>
  `).join('');
}

// Transfers / Streams View Rendering
function renderTransfers() {
  const container = document.getElementById('transfers-container');
  if (!container) return;

  const activeCount = state.transfers.filter(t => t.status === 'in_progress').length;
  const completedCount = state.transfers.filter(t => t.status === 'completed').length;

  const activeEl = document.getElementById('transfer-stat-active');
  const compEl = document.getElementById('transfer-stat-completed');
  if (activeEl) activeEl.textContent = activeCount;
  if (compEl) compEl.textContent = completedCount;

  if (state.transfers.length === 0) {
    container.innerHTML = `
      <div class="empty-placeholder">
        <div class="empty-icon">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
        </div>
        <p>No active replication pipelines or streams in flight.</p>
      </div>`;
    return;
  }

  container.innerHTML = state.transfers.slice().reverse().map(tr => {
    const pct = tr.total_bytes > 0 ? Math.min(100, Math.round((tr.bytes_transferred / tr.total_bytes) * 100)) : 100;
    const isCompleted = tr.status === 'completed';
    const isFailed = tr.status === 'failed';
    const statusBadgeClass = isCompleted ? 'badge-healthy' : (isFailed ? 'badge-danger' : 'badge-neutral');

    return `
      <div class="transfer-item">
        <div class="transfer-info-col">
          <div class="transfer-info-row">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="transfer-file-name">${escapeHtml(tr.file_name || 'Chunk Replication Stream')}</span>
              <span class="badge badge-neutral" style="font-size: 10px;">${escapeHtml(tr.type || 'Transfer')}</span>
            </div>
            <span class="badge ${statusBadgeClass}">${escapeHtml(tr.status || 'Active')}</span>
          </div>
          <div class="capacity-bar-track" style="margin-top: 6px;">
            <div class="capacity-bar-fill" style="width: ${pct}%"></div>
          </div>
          <div class="progress-meta">
            <span class="tabular-nums font-mono">${formatBytes(tr.bytes_transferred)} / ${formatBytes(tr.total_bytes)}</span>
            <span class="tabular-nums font-mono">${pct}%</span>
          </div>
        </div>
      </div>
    `;
  }).join('');
}

// Participating Devices View Rendering
function renderDevices() {
  const grid = document.getElementById('devices-grid');
  if (!grid) return;

  if (state.nodes.length === 0) {
    grid.innerHTML = `
      <div class="empty-placeholder" style="grid-column: 1/-1;">
        <p>No peer devices registered in the mesh cluster.</p>
      </div>`;
    return;
  }

  grid.innerHTML = state.nodes.map(n => {
    const isLocal = state.status && n.node_id === state.status.node_id;
    const isOnline = n.status === 'online';
    const usedBytes = n.used_bytes || 0;
    const capBytes = n.capacity_bytes || (10 * 1024 * 1024 * 1024);
    const pct = capBytes > 0 ? Math.min(100, Math.round((usedBytes / capBytes) * 100)) : 0;

    return `
      <div class="device-card ${isLocal ? 'local-device' : ''}">
        <div class="device-card-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <div class="device-avatar">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
            </div>
            <div class="device-title-box">
              <div class="device-name">${escapeHtml(n.display_name || 'Node Agent')}</div>
              <div class="device-id font-mono tabular-nums">${truncateMiddle(n.node_id, 10, 8)}</div>
            </div>
          </div>
          <span class="badge ${isOnline ? 'badge-healthy' : 'badge-danger'}">
            ${isOnline ? 'Online' : 'Offline'}
          </span>
        </div>

        <div class="device-stats-grid">
          <div class="stat-item">
            <span class="stat-label">P2P Socket</span>
            <span class="stat-val font-mono tabular-nums">${escapeHtml(n.address)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Last Ping</span>
            <span class="stat-val font-mono tabular-nums">${isOnline ? 'Active' : formatDate(n.last_seen)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Allocated Quota</span>
            <span class="stat-val font-mono tabular-nums">${formatBytes(capBytes)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Used by Mesh</span>
            <span class="stat-val font-mono tabular-nums">${formatBytes(usedBytes)} (${pct}%)</span>
          </div>
        </div>

        <div class="capacity-bar-track" style="margin-top: 12px;">
          <div class="capacity-bar-fill" style="width: ${pct}%"></div>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 12px;">
          ${isLocal ? '<span class="badge badge-healthy">Primary Authority Node</span>' : '<span class="badge badge-neutral">Mesh Peer</span>'}
          <button class="btn btn-secondary btn-xs btn-copy-device-id" data-id="${n.node_id}">Copy ID</button>
        </div>
      </div>
    `;
  }).join('');
}

// Engine Settings View Rendering
function renderSettings() {
  if (!state.status) return;

  const repInput = document.getElementById('setting-rep-factor');
  const slider = document.getElementById('slider-storage-quota') || document.getElementById('setting-quota-slider');
  const sliderDisplay = document.getElementById('quota-display-value') || document.getElementById('setting-quota-display');

  if (repInput) repInput.value = `${state.status.replication_factor || 2}x Replication`;

  if (slider && !slider.dataset.userTouched) {
    const gb = Math.round((state.status.storage_max || (10 * 1024 * 1024 * 1024)) / (1024 * 1024 * 1024));
    slider.value = gb;
    if (sliderDisplay) sliderDisplay.textContent = `${gb} GB`;
  }
}

// Inspect File Details & Chunk Topology Modal
async function openFileDetailsModal(fileId) {
  try {
    const res = await fetchAPI(`/api/v1/files/${fileId}`);
    const file = res.file;
    const chunks = res.chunks || [];
    state.selectedFileForModal = file;

    const modal = document.getElementById('modal-file-details');
    const nameEl = document.getElementById('file-modal-name');
    const sizeEl = document.getElementById('file-modal-size');
    const chunksCountEl = document.getElementById('file-modal-chunks-count');
    const hashEl = document.getElementById('file-modal-hash');
    const chunksList = document.getElementById('file-modal-chunks-list');

    if (nameEl) nameEl.textContent = file.file_name;
    if (sizeEl) sizeEl.textContent = formatBytes(file.size_bytes);
    if (chunksCountEl) chunksCountEl.textContent = `${chunks.length} Chunks`;
    if (hashEl) hashEl.textContent = file.content_hash || file.file_id;

    if (chunksList) {
      if (chunks.length === 0) {
        chunksList.innerHTML = `<tr><td colspan="4" class="empty-placeholder">No chunk records found for this file.</td></tr>`;
      } else {
        chunksList.innerHTML = chunks.map(ch => {
          const locs = ch.locations || [];
          return `
            <tr>
              <td class="font-mono tabular-nums">#${ch.index}</td>
              <td class="font-mono tabular-nums" title="${ch.chunk_id}">
                ${truncateMiddle(ch.chunk_id, 10, 8)}
              </td>
              <td class="font-mono tabular-nums">${formatBytes(ch.size_bytes)}</td>
              <td>
                <div style="display: flex; gap: 4px; flex-wrap: wrap;">
                  ${locs.length === 0 ? '<span class="badge badge-neutral">Unassigned</span>' : locs.map(loc => {
                    const isLocal = state.status && loc === state.status.node_id;
                    return `<span class="badge ${isLocal ? 'badge-healthy' : 'badge-neutral'}" title="Node: ${loc}">
                      ${isLocal ? 'Local Node' : truncateMiddle(loc, 6, 4)}
                    </span>`;
                  }).join('')}
                </div>
              </td>
            </tr>
          `;
        }).join('');
      }
    }

    if (modal) modal.classList.add('active');
  } catch (err) {
    showToast(`Failed to load file topology: ${err.message}`, 'error');
  }
}

// Download File Helper
function triggerFileDownload(fileId, filename) {
  const downloadUrl = `${state.apiBase}/api/v1/files/${fileId}/download`;
  showToast(`Streaming and reconstructing ${filename}...`, 'info');

  const a = document.createElement('a');
  a.href = downloadUrl;
  a.download = filename || 'downloaded_file';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);

  setTimeout(syncMeshState, 1000);
}

// Direct File Upload with Modal Progress Tracking
function uploadFileToMesh(file) {
  return new Promise((resolve, reject) => {
    const uploadModal = document.getElementById('modal-upload');
    const uploadFileName = document.getElementById('upload-file-name');
    const uploadFileSize = document.getElementById('upload-file-size');
    const uploadProgressBox = document.getElementById('modal-upload-progress');
    const uploadProgressFill = document.getElementById('modal-upload-progress-fill');
    const uploadStatusText = document.getElementById('modal-upload-status-text');
    const uploadPercent = document.getElementById('modal-upload-percent');
    const submitBtn = document.getElementById('btn-submit-upload');

    if (uploadFileName) uploadFileName.textContent = file.name;
    if (uploadFileSize) uploadFileSize.textContent = formatBytes(file.size);
    if (uploadProgressBox) uploadProgressBox.style.display = 'block';
    if (uploadStatusText) uploadStatusText.textContent = 'Chunking, encrypting, and replicating...';
    if (uploadProgressFill) uploadProgressFill.style.width = '0%';
    if (uploadPercent) uploadPercent.textContent = '0%';
    if (submitBtn) submitBtn.setAttribute('disabled', 'true');

    if (uploadModal) uploadModal.classList.add('active');

    const formData = new FormData();
    formData.append('file', file);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${state.apiBase}/api/v1/files`, true);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100);
        if (uploadProgressFill) uploadProgressFill.style.width = `${pct}%`;
        if (uploadPercent) uploadPercent.textContent = `${pct}%`;
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        showToast(`"${file.name}" sliced, encrypted & distributed!`, 'success');
        if (uploadModal) uploadModal.classList.remove('active');
        syncMeshState();
        resolve(JSON.parse(xhr.responseText));
      } else {
        const err = `Upload failed (HTTP ${xhr.status})`;
        showToast(err, 'error');
        if (uploadStatusText) uploadStatusText.textContent = err;
        reject(new Error(err));
      }
      if (submitBtn) submitBtn.removeAttribute('disabled');
    };

    xhr.onerror = () => {
      showToast('Network error during upload', 'error');
      if (uploadStatusText) uploadStatusText.textContent = 'Network error during upload';
      if (submitBtn) submitBtn.removeAttribute('disabled');
      reject(new Error('Network error'));
    };

    xhr.send(formData);
  });
}

// Setup Event Listeners
function initEventListeners() {
  // Navigation Tabs Click
  const tabs = document.querySelectorAll('.nav-tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const screen = tab.dataset.screen;
      if (screen) switchTab(screen);
    });
  });

  // Global Keyboard Shortcuts
  document.addEventListener('keydown', (e) => {
    // If user presses Escape, dismiss active modal, mobile menu, or theme dropdown
    if (e.key === 'Escape') {
      const activeModals = document.querySelectorAll('.modal-backdrop.active');
      activeModals.forEach(m => m.classList.remove('active'));
      closeMobileMenu();
      document.getElementById('theme-dropdown-menu')?.classList.remove('active');
      return;
    }

    // Don't intercept shortcuts if typing inside an input or textarea
    const targetTag = e.target.tagName.toLowerCase();
    if (targetTag === 'input' || targetTag === 'textarea' || targetTag === 'select') {
      return;
    }

    // Number keys 1-5 for tab switching
    if (e.key === '1') { switchTab('screen-dashboard'); }
    else if (e.key === '2') { switchTab('screen-files'); }
    else if (e.key === '3') { switchTab('screen-transfers'); }
    else if (e.key === '4') { switchTab('screen-devices'); }
    else if (e.key === '5') { switchTab('screen-settings'); }
    // Slash (/) key to focus file search
    else if (e.key === '/') {
      e.preventDefault();
      switchTab('screen-files');
      const searchInput = document.getElementById('input-file-search');
      if (searchInput) {
        searchInput.focus();
        searchInput.select();
      }
    }
  });

  // Mobile Navigation Drawer Listeners
  document.getElementById('btn-mobile-menu')?.addEventListener('click', openMobileMenu);
  document.getElementById('btn-close-mobile-menu')?.addEventListener('click', closeMobileMenu);
  document.getElementById('mobile-nav-backdrop')?.addEventListener('click', closeMobileMenu);

  document.querySelectorAll('.mobile-nav-item').forEach(btn => {
    btn.addEventListener('click', () => {
      const screen = btn.dataset.screen;
      if (screen) switchTab(screen);
    });
  });

  // Apple Theme Picker Toggle
  const themePickerBtn = document.getElementById('btn-theme-picker');
  const themeDropdown = document.getElementById('theme-dropdown-menu');
  themePickerBtn?.addEventListener('click', (e) => {
    e.stopPropagation();
    themeDropdown?.classList.toggle('active');
  });

  document.addEventListener('click', (e) => {
    if (!e.target.closest('#theme-picker-container')) {
      themeDropdown?.classList.remove('active');
    }
    // Handle any theme option click
    const themeBtn = e.target.closest('[data-theme]');
    if (themeBtn && (themeBtn.classList.contains('theme-option') || themeBtn.classList.contains('mobile-theme-btn'))) {
      applyTheme(themeBtn.dataset.theme);
      themeDropdown?.classList.remove('active');
    }
  });

  // Refresh Buttons
  document.getElementById('btn-refresh-all')?.addEventListener('click', () => {
    showToast('Syncing distributed mesh state...', 'info');
    syncMeshState();
  });
  document.getElementById('btn-refresh-files')?.addEventListener('click', () => {
    showToast('Refreshing object store index...', 'info');
    syncMeshState();
  });

  // Search input in Shared Files
  const searchInput = document.getElementById('input-file-search');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      state.searchQuery = e.target.value.trim();
      renderFiles();
    });
  }

  // Tactile Slicing Dropzone (Screen B)
  const dropZone = document.getElementById('file-drop-zone');
  const fileInput = document.getElementById('input-file-select');
  if (dropZone && fileInput) {
    dropZone.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', (e) => {
      if (e.target.files.length > 0) {
        uploadFileToMesh(e.target.files[0]);
        fileInput.value = '';
      }
    });

    ['dragenter', 'dragover'].forEach(eventName => {
      dropZone.addEventListener(eventName, (e) => {
        e.preventDefault();
        dropZone.classList.add('dragover');
      });
    });

    ['dragleave', 'drop'].forEach(eventName => {
      dropZone.addEventListener(eventName, (e) => {
        e.preventDefault();
        dropZone.classList.remove('dragover');
      });
    });

    dropZone.addEventListener('drop', (e) => {
      if (e.dataTransfer.files.length > 0) {
        uploadFileToMesh(e.dataTransfer.files[0]);
      }
    });
  }

  // Quick Distribute and Upload Trigger Buttons
  const triggerUpload = () => {
    if (fileInput) fileInput.click();
  };
  document.getElementById('btn-quick-upload')?.addEventListener('click', triggerUpload);
  document.getElementById('btn-dash-upload')?.addEventListener('click', triggerUpload);
  document.getElementById('btn-files-upload')?.addEventListener('click', triggerUpload);

  // Upload Progress Modal Close Buttons
  const uploadModal = document.getElementById('modal-upload');
  document.getElementById('btn-close-upload-modal')?.addEventListener('click', () => {
    uploadModal?.classList.remove('active');
  });
  document.getElementById('btn-cancel-upload-modal')?.addEventListener('click', () => {
    uploadModal?.classList.remove('active');
  });

  // Table Delegated Event Handlers (Download, Inspect, Delete, Copy ID)
  document.addEventListener('click', async (e) => {
    // Inspect Chunk Topology
    const inspectBtn = e.target.closest('.btn-file-inspect');
    if (inspectBtn) {
      const fileId = inspectBtn.dataset.id;
      if (fileId) openFileDetailsModal(fileId);
      return;
    }

    // Download File
    const downloadBtn = e.target.closest('.btn-file-download');
    if (downloadBtn) {
      const fileId = downloadBtn.dataset.id;
      const fileName = downloadBtn.dataset.name;
      if (fileId) triggerFileDownload(fileId, fileName);
      return;
    }

    // Delete File
    const deleteBtn = e.target.closest('.btn-file-delete');
    if (deleteBtn) {
      const fileId = deleteBtn.dataset.id;
      const fileName = deleteBtn.dataset.name;
      if (confirm(`Are you sure you want to delete "${fileName}" from the distributed mesh?`)) {
        try {
          await fetchAPI(`/api/v1/files/${fileId}`, { method: 'DELETE' });
          showToast(`File "${fileName}" deleted from mesh`, 'success');
          syncMeshState();
        } catch (err) {
          showToast(`Delete failed: ${err.message}`, 'error');
        }
      }
      return;
    }

    // Copy Device ID
    const copyDevBtn = e.target.closest('.btn-copy-device-id');
    if (copyDevBtn) {
      copyText(copyDevBtn.dataset.id, 'Node ID copied to clipboard');
      return;
    }
  });

  // File Details Modal Close & Action Handlers
  const fileModal = document.getElementById('modal-file-details');
  document.getElementById('btn-close-file-modal')?.addEventListener('click', () => {
    fileModal?.classList.remove('active');
  });
  document.getElementById('btn-modal-cancel')?.addEventListener('click', () => {
    fileModal?.classList.remove('active');
  });
  document.getElementById('btn-copy-file-hash')?.addEventListener('click', () => {
    const hashText = document.getElementById('file-modal-hash')?.textContent;
    if (hashText) copyText(hashText, 'Root SHA-256 Hash copied');
  });
  document.getElementById('btn-modal-download')?.addEventListener('click', () => {
    if (state.selectedFileForModal) {
      triggerFileDownload(state.selectedFileForModal.file_id, state.selectedFileForModal.file_name);
    }
  });

  // Connect Peer Device Modal
  const addDeviceModal = document.getElementById('modal-add-device');
  const openDeviceModal = () => addDeviceModal?.classList.add('active');
  document.getElementById('btn-open-add-device-modal')?.addEventListener('click', openDeviceModal);
  document.getElementById('btn-dash-connect-device')?.addEventListener('click', openDeviceModal);

  document.getElementById('btn-close-device-modal')?.addEventListener('click', () => {
    addDeviceModal?.classList.remove('active');
  });
  document.getElementById('btn-cancel-device-modal')?.addEventListener('click', () => {
    addDeviceModal?.classList.remove('active');
  });

  // Copy LAN Address buttons
  const copyLANHandler = () => {
    const addr = state.status?.lan_address || document.getElementById('shareable-lan-address')?.textContent;
    if (addr && addr !== 'Detecting...') {
      copyText(addr, 'Shareable address copied! Enter this on the other computer.');
    }
  };
  document.getElementById('btn-copy-lan-address')?.addEventListener('click', copyLANHandler);
  document.getElementById('btn-copy-modal-lan-address')?.addEventListener('click', copyLANHandler);

  // Submit Connect Peer
  document.getElementById('btn-submit-connect-peer')?.addEventListener('click', async () => {
    const input = document.getElementById('input-peer-address');
    const feedback = document.getElementById('peer-connect-feedback');
    const address = input ? input.value.trim() : '';

    if (!address) {
      showToast('Please enter a peer address (e.g. 192.168.1.38:7001)', 'error');
      return;
    }

    if (feedback) {
      feedback.style.display = 'block';
      feedback.textContent = `Connecting to peer at ${address}...`;
      feedback.style.color = 'var(--color-healthy)';
    }

    try {
      const res = await fetchAPI('/api/v1/nodes/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ address })
      });
      showToast(res.message || 'Peer connection initiated!', 'success');
      addDeviceModal?.classList.remove('active');
      if (input) input.value = '';
      if (feedback) feedback.style.display = 'none';
      syncMeshState();
    } catch (err) {
      if (feedback) {
        feedback.style.display = 'block';
        feedback.textContent = `Connection failed: ${err.message}`;
        feedback.style.color = 'var(--color-danger)';
      }
    }
  });

  // Storage Quota Slider
  const slider = document.getElementById('slider-storage-quota') || document.getElementById('setting-quota-slider');
  const sliderDisplay = document.getElementById('quota-display-value') || document.getElementById('setting-quota-display');
  if (slider && sliderDisplay) {
    slider.addEventListener('input', (e) => {
      slider.dataset.userTouched = 'true';
      sliderDisplay.textContent = `${e.target.value} GB`;
    });
  }

  // Save Quota Button
  document.getElementById('btn-save-quota')?.addEventListener('click', async () => {
    if (!slider) return;
    const gb = parseInt(slider.value, 10);
    try {
      await fetchAPI('/api/v1/storage', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ max_storage_gb: gb })
      });
      showToast(`Local disk allocation quota updated to ${gb} GB`, 'success');
      syncMeshState();
    } catch (err) {
      showToast(`Failed to update quota: ${err.message}`, 'error');
    }
  });

  // Switch API URL
  document.getElementById('btn-apply-api-url')?.addEventListener('click', () => {
    const input = document.getElementById('setting-api-url');
    if (input && input.value.trim()) {
      state.apiBase = input.value.trim().replace(/\/+$/, '');
      showToast(`Switched API node target to ${state.apiBase}`, 'info');
      syncMeshState();
    }
  });
}

// Initialization on DOMContentLoaded
document.addEventListener('DOMContentLoaded', () => {
  // Initialize Apple Color Theme from persistent storage
  let savedTheme = 'apple-space-black';
  try {
    savedTheme = localStorage.getItem('diofos-theme') || 'apple-space-black';
  } catch (e) {}
  applyTheme(savedTheme);

  initEventListeners();
  syncMeshState();
  // Poll mesh telemetry every 3 seconds
  state.pollTimer = setInterval(syncMeshState, 3000);
});
