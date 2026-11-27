/**
 * MeshStore Web Dashboard - Client Logic
 * Handles P2P state synchronization, REST API integration,
 * chunk topology visualization, file streaming, and transfer tracking.
 */

// State Object
const state = {
  apiBase: window.location.origin.includes('http') ? window.location.origin : 'http://localhost:8070',
  currentTab: 'screen-dashboard',
  status: null,
  storage: null,
  nodes: [],
  files: [],
  transfers: [],
  activeFileFilter: 'all',
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
  if (!str || str.length <= frontLen + backLen) return str;
  return str.substring(0, frontLen) + '...' + str.substring(str.length - backLen);
}

function getFileIcon(filename) {
  const ext = (filename.split('.').pop() || '').toLowerCase();
  if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'mp4', 'mov', 'mp3'].includes(ext)) {
    return `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>`;
  }
  if (['zip', 'tar', 'gz', '7z', 'rar'].includes(ext)) {
    return `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="21 8 21 21 3 21 3 8"></polyline><rect x="1" y="3" width="22" height="5"></rect><line x1="10" y1="12" x2="14" y2="12"></line></svg>`;
  }
  if (['go', 'js', 'ts', 'py', 'json', 'yaml', 'html', 'css', 'c', 'cpp'].includes(ext)) {
    return `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>`;
  }
  return `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>`;
}

function showToast(message, type = 'info') {
  const hub = document.getElementById('toast-hub');
  if (!hub) return;
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  hub.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    setTimeout(() => toast.remove(), 250);
  }, 4000);
}

// Copy to Clipboard Helper
function copyText(text, label = 'Copied!') {
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

// REST API Methods
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

// Data Fetchers
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
    updateClusterHealth('offline', 'Disconnected from Node');
  }
}

function updateClusterHealth(health, label) {
  const beacon = document.getElementById('health-beacon');
  const text = document.getElementById('health-status-text');
  const pill = document.getElementById('cluster-health-pill');
  if (!beacon || !text || !pill) return;

  text.textContent = label;
  if (health === 'healthy') {
    beacon.style.backgroundColor = 'var(--color-success)';
    beacon.style.boxShadow = '0 0 8px var(--color-success)';
    pill.style.borderColor = 'var(--color-success-border)';
  } else if (health === 'degraded') {
    beacon.style.backgroundColor = 'var(--color-warning)';
    beacon.style.boxShadow = '0 0 8px var(--color-warning)';
    pill.style.borderColor = 'var(--color-warning-border)';
  } else {
    beacon.style.backgroundColor = 'var(--color-danger)';
    beacon.style.boxShadow = '0 0 8px var(--color-danger)';
    pill.style.borderColor = 'var(--color-danger-border)';
  }
}

// Render Functions
function renderHeader() {
  if (state.status) {
    const nameEl = document.getElementById('header-node-name');
    const portEl = document.getElementById('header-node-port');
    if (nameEl) nameEl.textContent = state.status.display_name || 'MeshNode';
    if (portEl) {
      const parts = (state.status.listen_address || '').split(':');
      portEl.textContent = parts.length > 1 ? `:${parts[parts.length - 1]}` : '';
    }

    const lanAddr = state.status.lan_address || state.status.listen_address;
    const shareableEl = document.getElementById('shareable-lan-address');
    const modalLanEl = document.getElementById('modal-my-lan-address');
    if (shareableEl && lanAddr) shareableEl.textContent = lanAddr;
    if (modalLanEl && lanAddr) modalLanEl.textContent = lanAddr;
  }

  if (state.storage) {
    const online = state.storage.nodes_online || 1;
    const total = state.storage.nodes_total || 1;
    const health = state.storage.storage_health || 'healthy';
    let label = `Cluster Healthy (${online}/${total} Nodes)`;
    if (health === 'degraded') label = `Mesh Degraded (${online}/${total} Nodes)`;
    if (health === 'critical') label = `Mesh Critical (All Offline)`;
    updateClusterHealth(health, label);
  }

  // Update Nav Badges
  const filesCountBadge = document.getElementById('nav-files-count');
  if (filesCountBadge) filesCountBadge.textContent = state.files.length;

  const nodesCountBadge = document.getElementById('nav-nodes-count');
  if (nodesCountBadge) nodesCountBadge.textContent = state.nodes.length || 1;

  const activeTransfers = state.transfers.filter(t => t.status === 'in_progress').length;
  const transfersBadge = document.getElementById('nav-transfers-count');
  if (transfersBadge) transfersBadge.textContent = activeTransfers;
}

function renderDashboard() {
  if (!state.storage || !state.status) return;

  // Metric Cards
  const usableEl = document.getElementById('metric-usable-cap');
  const usedEl = document.getElementById('metric-used-space');
  const rawEl = document.getElementById('metric-raw-cap');
  const fillEl = document.getElementById('metric-capacity-fill');
  const repBadge = document.getElementById('metric-rep-factor');

  if (usableEl) usableEl.textContent = formatBytes(state.storage.usable_capacity_bytes);
  if (usedEl) usedEl.textContent = `Used: ${formatBytes(state.storage.used_bytes)}`;
  if (rawEl) rawEl.textContent = `Raw: ${formatBytes(state.storage.raw_capacity_bytes)}`;
  if (repBadge) repBadge.textContent = `${state.storage.replication_factor}x Replication`;

  if (fillEl && state.storage.raw_capacity_bytes > 0) {
    const pct = Math.min(100, Math.round((state.storage.used_bytes / state.storage.raw_capacity_bytes) * 100));
    fillEl.style.width = `${pct}%`;
  }

  const onlineNodesEl = document.getElementById('metric-online-nodes');
  const totalNodesEl = document.getElementById('metric-total-nodes');
  if (onlineNodesEl) onlineNodesEl.textContent = state.storage.nodes_online;
  if (totalNodesEl) totalNodesEl.textContent = state.storage.nodes_total;

  const filesCountEl = document.getElementById('metric-files-count');
  const chunksCountEl = document.getElementById('metric-chunks-count');
  if (filesCountEl) filesCountEl.textContent = state.files.length;
  if (chunksCountEl) {
    const totalChunks = state.files.reduce((acc, f) => acc + (f.chunks ? f.chunks.length : 0), 0);
    chunksCountEl.textContent = totalChunks;
  }

  const activeTransfersEl = document.getElementById('metric-active-transfers');
  if (activeTransfersEl) {
    const active = state.transfers.filter(t => t.status === 'in_progress').length;
    activeTransfersEl.textContent = active;
  }

  // Dashboard Recent Files Table (first 5)
  const tbody = document.getElementById('dash-recent-files-tbody');
  if (tbody) {
    if (state.files.length === 0) {
      tbody.innerHTML = `
        <tr>
          <td colspan="6" class="table-empty-cell">
            <div class="empty-state">
              <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
              <p>No files in the mesh yet. Upload your first file to begin distributing chunks!</p>
            </div>
          </td>
        </tr>`;
    } else {
      const recent = state.files.slice(-5).reverse();
      tbody.innerHTML = recent.map(file => `
        <tr>
          <td>
            <div class="file-name-cell">
              <div class="file-icon-box">${getFileIcon(file.file_name)}</div>
              <div>
                <span class="file-name-primary">${escapeHtml(file.file_name)}</span>
                <span class="file-name-sub">${truncateMiddle(file.file_id, 8, 6)}</span>
              </div>
            </div>
          </td>
          <td class="font-mono">${escapeHtml(file.logical_path || '/' + file.file_name)}</td>
          <td class="font-mono">${formatBytes(file.size_bytes)}</td>
          <td><span class="badge badge-cyan">${file.chunks ? file.chunks.length : 0} chunks</span></td>
          <td><span class="badge badge-success">${state.storage ? state.storage.replication_factor : 2}x verified</span></td>
          <td class="font-mono">${formatDate(file.created_at)}</td>
          <td class="text-right">
            <div class="table-actions">
              <button class="btn btn-secondary btn-xs btn-file-inspect" data-id="${file.file_id}">Inspect</button>
              <button class="btn btn-primary btn-xs btn-file-download" data-id="${file.file_id}" data-name="${escapeHtml(file.file_name)}">Download</button>
            </div>
          </td>
        </tr>
      `).join('');
    }
  }

  // Dashboard Mini Devices List
  const devList = document.getElementById('dash-device-mini-list');
  if (devList) {
    if (state.nodes.length === 0) {
      devList.innerHTML = `<p class="empty-state" style="padding: 16px;">No remote peers connected.</p>`;
    } else {
      devList.innerHTML = state.nodes.map(n => {
        const isLocal = state.status && n.node_id === state.status.node_id;
        return `
          <div class="device-mini-item">
            <div class="device-mini-info">
              <div class="status-beacon" style="background: ${n.status === 'online' ? 'var(--color-success)' : 'var(--color-danger)'};"></div>
              <div>
                <span class="device-mini-name">${escapeHtml(n.display_name || 'Node')} ${isLocal ? '<span class="badge badge-cyan">This Device</span>' : ''}</span>
                <span class="device-mini-addr">${escapeHtml(n.address)}</span>
              </div>
            </div>
            <div class="device-mini-status">
              <span class="font-mono">${formatBytes(n.used_bytes || 0)} / ${formatBytes(n.capacity_bytes || 0)}</span>
            </div>
          </div>
        `;
      }).join('');
    }
  }
}

function renderFiles() {
  const tbody = document.getElementById('files-list-tbody');
  const countBadge = document.getElementById('files-table-count');
  if (!tbody) return;

  // Filter & Search
  let filtered = state.files.slice();

  // Category filter
  if (state.activeFileFilter !== 'all') {
    filtered = filtered.filter(f => {
      const ext = (f.file_name.split('.').pop() || '').toLowerCase();
      if (state.activeFileFilter === 'doc') return ['pdf', 'doc', 'docx', 'txt', 'md', 'xlsx'].includes(ext);
      if (state.activeFileFilter === 'media') return ['jpg', 'png', 'gif', 'mp4', 'mov', 'mp3', 'svg'].includes(ext);
      if (state.activeFileFilter === 'archive') return ['zip', 'tar', 'gz', '7z', 'rar'].includes(ext);
      if (state.activeFileFilter === 'code') return ['go', 'js', 'ts', 'py', 'json', 'yaml', 'html', 'css'].includes(ext);
      return true;
    });
  }

  // Search input
  if (state.searchQuery) {
    const q = state.searchQuery.toLowerCase();
    filtered = filtered.filter(f => 
      f.file_name.toLowerCase().includes(q) || 
      (f.logical_path && f.logical_path.toLowerCase().includes(q)) ||
      (f.file_id && f.file_id.toLowerCase().includes(q))
    );
  }

  if (countBadge) countBadge.textContent = `${filtered.length} files`;

  if (filtered.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="8" class="table-empty-cell">
          <div class="empty-state">
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
            <p>${state.searchQuery ? 'No files match your search query.' : 'No files in this category. Upload files using the dropzone above.'}</p>
          </div>
        </td>
      </tr>`;
    return;
  }

  tbody.innerHTML = filtered.map(file => `
    <tr>
      <td>
        <div class="file-name-cell">
          <div class="file-icon-box">${getFileIcon(file.file_name)}</div>
          <div>
            <span class="file-name-primary">${escapeHtml(file.file_name)}</span>
            <span class="file-name-sub">${truncateMiddle(file.file_id, 8, 6)}</span>
          </div>
        </div>
      </td>
      <td class="font-mono">${escapeHtml(file.logical_path || '/' + file.file_name)}</td>
      <td class="font-mono">${formatBytes(file.size_bytes)}</td>
      <td><span class="badge badge-cyan">${file.chunks ? file.chunks.length : 0} chunks</span></td>
      <td><span class="badge badge-success">${state.storage ? state.storage.replication_factor : 2}x Distributed</span></td>
      <td><span class="badge badge-success">Available</span></td>
      <td class="font-mono">${formatDate(file.created_at)}</td>
      <td class="text-right">
        <div class="table-actions">
          <button class="btn btn-secondary btn-xs btn-file-inspect" data-id="${file.file_id}" title="Inspect Chunk Topology">Inspect</button>
          <button class="btn btn-primary btn-xs btn-file-download" data-id="${file.file_id}" data-name="${escapeHtml(file.file_name)}" title="Download from Mesh">Download</button>
          <button class="btn btn-danger-ghost btn-xs btn-file-delete" data-id="${file.file_id}" data-name="${escapeHtml(file.file_name)}" title="Delete from Mesh">Delete</button>
        </div>
      </td>
    </tr>
  `).join('');
}

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
      <div class="empty-state" style="padding: 48px;">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
        <p>No file transfers currently running or queued.</p>
      </div>`;
    return;
  }

  container.innerHTML = state.transfers.slice().reverse().map(tr => {
    const pct = tr.total_bytes > 0 ? Math.min(100, Math.round((tr.bytes_transferred / tr.total_bytes) * 100)) : 100;
    const isCompleted = tr.status === 'completed';
    const isFailed = tr.status === 'failed';
    const statusBadgeClass = isCompleted ? 'badge-success' : (isFailed ? 'badge-danger' : 'badge-cyan');
    return `
      <div class="transfer-item">
        <div class="transfer-info-col">
          <div class="transfer-info-row">
            <div>
              <span class="transfer-file-name">${escapeHtml(tr.file_name || 'Chunk Stream')}</span>
              <span class="badge ${statusBadgeClass}" style="margin-left: 8px;">${escapeHtml(tr.type || 'Transfer')}</span>
            </div>
            <span class="badge ${statusBadgeClass}">${escapeHtml(tr.status || 'Active')}</span>
          </div>
          <div class="progress-bar-track">
            <div class="progress-bar-fill" style="width: ${pct}%"></div>
          </div>
          <div class="progress-meta">
            <span>${formatBytes(tr.bytes_transferred)} of ${formatBytes(tr.total_bytes)}</span>
            <span class="font-mono">${pct}%</span>
          </div>
        </div>
      </div>
    `;
  }).join('');
}

function renderDevices() {
  const grid = document.getElementById('devices-grid');
  if (!grid) return;

  if (state.nodes.length === 0) {
    grid.innerHTML = `<div class="empty-state" style="grid-column: 1/-1; padding: 48px;"><p>No peer devices registered in the mesh cluster.</p></div>`;
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
          <div style="display: flex; align-items: center;">
            <div class="device-avatar">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
            </div>
            <div class="device-title-box">
              <div class="device-name">${escapeHtml(n.display_name || 'Node Agent')}</div>
              <div class="device-id">${truncateMiddle(n.node_id, 10, 8)}</div>
            </div>
          </div>
          <span class="badge ${isOnline ? 'badge-success' : 'badge-danger'}">
            ${isOnline ? 'Online' : 'Offline'}
          </span>
        </div>

        <div class="device-stats-grid">
          <div class="stat-item">
            <span class="stat-label">P2P Socket</span>
            <span class="stat-val font-mono">${escapeHtml(n.address)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Last Seen</span>
            <span class="stat-val">${isOnline ? 'Active' : formatDate(n.last_seen)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Contributed Space</span>
            <span class="stat-val">${formatBytes(capBytes)}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Used by Mesh</span>
            <span class="stat-val">${formatBytes(usedBytes)} (${pct}%)</span>
          </div>
        </div>

        <div class="capacity-bar-track">
          <div class="capacity-bar-fill" style="width: ${pct}%"></div>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 4px;">
          ${isLocal ? '<span class="badge badge-cyan">Primary Local Node</span>' : '<span class="badge">Mesh Peer</span>'}
          <button class="btn btn-secondary btn-xs btn-copy-device-id" data-id="${n.node_id}">Copy ID</button>
        </div>
      </div>
    `;
  }).join('');
}

function renderSettings() {
  if (!state.status) return;

  const nameInput = document.getElementById('setting-display-name');
  const idInput = document.getElementById('setting-node-id');
  const netInput = document.getElementById('setting-network-id');
  const addrInput = document.getElementById('setting-listen-addr');
  const chunkInput = document.getElementById('setting-chunk-size');
  const repInput = document.getElementById('setting-rep-factor');
  const slider = document.getElementById('setting-quota-slider');
  const sliderDisplay = document.getElementById('setting-quota-display');

  if (nameInput) nameInput.value = state.status.display_name || 'MeshNode';
  if (idInput) idInput.value = state.status.node_id || '';
  if (netInput) netInput.value = state.status.network_id || 'home-mesh';
  if (addrInput) addrInput.value = state.status.listen_address || '127.0.0.1:7000';
  if (chunkInput) chunkInput.value = `${state.status.chunk_size_mb || 1} MB`;
  if (repInput) repInput.value = `${state.status.replication_factor || 2}x Redundancy`;

  if (slider && !slider.dataset.userTouched) {
    const gb = Math.round((state.status.storage_max || (10 * 1024 * 1024 * 1024)) / (1024 * 1024 * 1024));
    slider.value = gb;
    if (sliderDisplay) sliderDisplay.textContent = `${gb} GB`;
  }
}

// Screen F: Inspect File Details & Chunk Topology Modal
async function openFileDetailsModal(fileId) {
  try {
    const res = await fetchAPI(`/api/v1/files/${fileId}`);
    const file = res.file;
    const chunks = res.chunks || [];
    state.selectedFileForModal = file;

    const modal = document.getElementById('modal-file-details');
    const titleEl = document.getElementById('file-modal-title');
    const pathEl = document.getElementById('file-modal-path');
    const sizeEl = document.getElementById('file-modal-size');
    const chunksCountEl = document.getElementById('file-modal-chunks-count');
    const hashEl = document.getElementById('file-modal-hash');
    const dateEl = document.getElementById('file-modal-date');
    const chunkGrid = document.getElementById('file-modal-chunks-grid');

    if (titleEl) titleEl.textContent = file.file_name;
    if (pathEl) pathEl.textContent = file.logical_path || '/' + file.file_name;
    if (sizeEl) sizeEl.textContent = formatBytes(file.size_bytes);
    if (chunksCountEl) chunksCountEl.textContent = `${chunks.length} Chunks`;
    if (hashEl) hashEl.textContent = file.content_hash || file.file_id;
    if (dateEl) dateEl.textContent = formatDate(file.created_at);

    // Build Chunk Topology Cards
    if (chunkGrid) {
      if (chunks.length === 0) {
        chunkGrid.innerHTML = `<p class="empty-state">No chunk records found for this file.</p>`;
      } else {
        chunkGrid.innerHTML = chunks.map(ch => {
          const locs = ch.locations || [];
          return `
            <div class="chunk-card">
              <div class="chunk-card-header">
                <span class="chunk-index-badge">Chunk #${ch.index}</span>
                <span class="chunk-size-label">${formatBytes(ch.size_bytes)}</span>
              </div>
              <div class="chunk-hash-text font-mono" title="${ch.chunk_id}">
                ${truncateMiddle(ch.chunk_id, 10, 8)}
              </div>
              <div class="chunk-replicas-row">
                ${locs.length === 0 ? '<span class="node-chip">Unassigned</span>' : locs.map(loc => {
                  const isLocal = state.status && loc === state.status.node_id;
                  return `<span class="node-chip ${isLocal ? 'local' : ''}" title="Node: ${loc}">${isLocal ? 'Local Node' : truncateMiddle(loc, 6, 4)}</span>`;
                }).join('')}
              </div>
            </div>
          `;
        }).join('');
      }
    }

    modal.classList.add('active');
  } catch (err) {
    showToast(`Failed to load file details: ${err.message}`, 'error');
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

  // Trigger sync to update transfer log
  setTimeout(syncMeshState, 1000);
}

// Upload File via XHR for Live Progress
function uploadFileToMesh(file, logicalPath = '') {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append('file', file);
    if (logicalPath) formData.append('path', logicalPath);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${state.apiBase}/api/v1/files`, true);

    const progressFill = document.getElementById('dropzone-progress-fill');
    const progressBox = document.getElementById('dropzone-progress');
    const filenameEl = document.getElementById('dropzone-filename');
    const percentEl = document.getElementById('dropzone-percent');

    if (progressBox) progressBox.style.display = 'block';
    if (filenameEl) filenameEl.textContent = `Uploading ${file.name}...`;

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100);
        if (progressFill) progressFill.style.width = `${pct}%`;
        if (percentEl) percentEl.textContent = `${pct}%`;
      }
    };

    xhr.onload = () => {
      if (progressBox) progressBox.style.display = 'none';
      if (xhr.status >= 200 && xhr.status < 300) {
        showToast(`"${file.name}" chunked & distributed across mesh!`, 'success');
        syncMeshState();
        resolve(JSON.parse(xhr.responseText));
      } else {
        const err = `Upload failed (HTTP ${xhr.status})`;
        showToast(err, 'error');
        reject(new Error(err));
      }
    };

    xhr.onerror = () => {
      if (progressBox) progressBox.style.display = 'none';
      showToast('Network error during upload', 'error');
      reject(new Error('Network error'));
    };

    xhr.send(formData);
  });
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

// Setup Event Listeners
function initEventListeners() {
  // Navigation Tabs
  const tabs = document.querySelectorAll('.nav-tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      const targetScreen = tab.dataset.screen;
      document.querySelectorAll('.screen-view').forEach(s => s.classList.remove('active'));
      const activeScreen = document.getElementById(targetScreen);
      if (activeScreen) activeScreen.classList.add('active');
      state.currentTab = targetScreen;
    });
  });

  // Dashboard shortcuts
  document.getElementById('btn-dash-view-all-files')?.addEventListener('click', () => {
    document.getElementById('tab-files')?.click();
  });
  document.getElementById('btn-dash-manage-devices')?.addEventListener('click', () => {
    document.getElementById('tab-devices')?.click();
  });
  document.getElementById('btn-dash-connect-device')?.addEventListener('click', () => {
    document.getElementById('modal-add-device')?.classList.add('active');
  });
  document.getElementById('btn-dash-upload')?.addEventListener('click', () => {
    document.getElementById('modal-upload')?.classList.add('active');
  });

  // Refresh All button
  document.getElementById('btn-refresh-all')?.addEventListener('click', () => {
    showToast('Refreshing mesh storage state...', 'info');
    syncMeshState();
  });

  // Filter chips in Shared Files
  const chips = document.querySelectorAll('#file-filter-chips .chip');
  chips.forEach(chip => {
    chip.addEventListener('click', () => {
      chips.forEach(c => c.classList.remove('active'));
      chip.classList.add('active');
      state.activeFileFilter = chip.dataset.filter;
      renderFiles();
    });
  });

  // Search input in Shared Files
  const searchInput = document.getElementById('file-search-input');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      state.searchQuery = e.target.value.trim();
      renderFiles();
    });
  }

  // Direct Page Dropzone
  const directDropzone = document.getElementById('files-page-dropzone');
  const directFileInput = document.getElementById('direct-file-input');
  if (directDropzone && directFileInput) {
    directDropzone.addEventListener('click', () => directFileInput.click());
    directFileInput.addEventListener('change', (e) => {
      if (e.target.files.length > 0) {
        uploadFileToMesh(e.target.files[0]);
        directFileInput.value = '';
      }
    });

    ['dragenter', 'dragover'].forEach(eventName => {
      directDropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        directDropzone.classList.add('drag-active');
      });
    });

    ['dragleave', 'drop'].forEach(eventName => {
      directDropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        directDropzone.classList.remove('drag-active');
      });
    });

    directDropzone.addEventListener('drop', (e) => {
      if (e.dataTransfer.files.length > 0) {
        uploadFileToMesh(e.dataTransfer.files[0]);
      }
    });
  }

  // Quick Upload Button & Upload Modal
  const uploadModal = document.getElementById('modal-upload');
  const modalDropzone = document.getElementById('modal-dropzone');
  const modalFileInput = document.getElementById('modal-file-input');
  const modalSubmitBtn = document.getElementById('btn-submit-upload');
  const modalPathInput = document.getElementById('modal-upload-path');
  let chosenUploadFile = null;

  document.getElementById('btn-quick-upload')?.addEventListener('click', () => {
    uploadModal?.classList.add('active');
  });
  document.getElementById('btn-open-upload-modal')?.addEventListener('click', () => {
    uploadModal?.classList.add('active');
  });

  document.getElementById('btn-close-upload-modal')?.addEventListener('click', () => {
    uploadModal?.classList.remove('active');
  });
  document.getElementById('btn-cancel-upload-modal')?.addEventListener('click', () => {
    uploadModal?.classList.remove('active');
  });

  if (modalDropzone && modalFileInput) {
    modalDropzone.addEventListener('click', () => modalFileInput.click());
    modalFileInput.addEventListener('change', (e) => {
      if (e.target.files.length > 0) {
        chosenUploadFile = e.target.files[0];
        document.getElementById('modal-file-chosen-text').textContent = chosenUploadFile.name;
        document.getElementById('modal-file-size-text').textContent = formatBytes(chosenUploadFile.size);
        modalSubmitBtn.removeAttribute('disabled');
      }
    });
  }

  modalSubmitBtn?.addEventListener('click', async () => {
    if (!chosenUploadFile) return;
    modalSubmitBtn.setAttribute('disabled', 'true');
    const statusBox = document.getElementById('modal-upload-status-box');
    const fillEl = document.getElementById('modal-upload-progress-fill');
    const textEl = document.getElementById('modal-upload-status-text');
    const pctEl = document.getElementById('modal-upload-percent');

    if (statusBox) statusBox.style.display = 'block';
    if (textEl) textEl.textContent = 'Chunking and replicating...';

    try {
      await uploadFileToMesh(chosenUploadFile, modalPathInput?.value || '');
      uploadModal?.classList.remove('active');
      chosenUploadFile = null;
      if (modalPathInput) modalPathInput.value = '';
      document.getElementById('modal-file-chosen-text').textContent = 'Choose a file or drag here';
      document.getElementById('modal-file-size-text').textContent = 'Supports datasets, archives, documents, media';
    } catch (err) {
      if (textEl) textEl.textContent = `Error: ${err.message}`;
    } finally {
      if (statusBox) statusBox.style.display = 'none';
      modalSubmitBtn.removeAttribute('disabled');
    }
  });

  // Table Delegated Event Handlers (Download, Inspect, Delete)
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

  // Screen F Modal Close & Download
  document.getElementById('btn-close-file-modal')?.addEventListener('click', () => {
    document.getElementById('modal-file-details')?.classList.remove('active');
  });
  document.getElementById('btn-modal-cancel')?.addEventListener('click', () => {
    document.getElementById('modal-file-details')?.classList.remove('active');
  });
  document.getElementById('btn-copy-file-hash')?.addEventListener('click', () => {
    const hashText = document.getElementById('file-modal-hash')?.textContent;
    if (hashText) copyText(hashText, 'Root Hash copied');
  });
  document.getElementById('btn-modal-download')?.addEventListener('click', () => {
    if (state.selectedFileForModal) {
      triggerFileDownload(state.selectedFileForModal.file_id, state.selectedFileForModal.file_name);
    }
  });

  // Add Device Modal
  const addDeviceModal = document.getElementById('modal-add-device');
  document.getElementById('btn-open-add-device-modal')?.addEventListener('click', () => {
    addDeviceModal?.classList.add('active');
  });
  document.getElementById('btn-close-device-modal')?.addEventListener('click', () => {
    addDeviceModal?.classList.remove('active');
  });
  document.getElementById('btn-cancel-device-modal')?.addEventListener('click', () => {
    addDeviceModal?.classList.remove('active');
  });

  // Copy LAN Address buttons
  document.getElementById('btn-copy-lan-address')?.addEventListener('click', () => {
    const addr = state.status?.lan_address || document.getElementById('shareable-lan-address')?.textContent;
    if (addr && addr !== 'Detecting...') {
      copyText(addr, 'Connection address copied! Enter this on the other computer.');
    }
  });
  document.getElementById('btn-copy-modal-lan-address')?.addEventListener('click', () => {
    const addr = state.status?.lan_address || document.getElementById('modal-my-lan-address')?.textContent;
    if (addr && addr !== 'Detecting...') {
      copyText(addr, 'Connection address copied! Enter this on the other computer.');
    }
  });

  // Submit Connect Peer
  document.getElementById('btn-submit-connect-peer')?.addEventListener('click', async () => {
    const input = document.getElementById('input-peer-address');
    const feedback = document.getElementById('peer-connect-feedback');
    const address = input ? input.value.trim() : '';

    if (!address) {
      showToast('Please enter a peer address (e.g. 127.0.0.1:7001)', 'error');
      return;
    }

    if (feedback) {
      feedback.style.display = 'block';
      feedback.textContent = `Connecting to peer at ${address}...`;
      feedback.style.color = 'var(--accent-cyan)';
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

  // Quota Slider Handling
  const slider = document.getElementById('setting-quota-slider');
  const sliderDisplay = document.getElementById('setting-quota-display');
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
    const statusLabel = document.getElementById('quota-save-status');

    if (statusLabel) {
      statusLabel.textContent = 'Saving quota...';
      statusLabel.style.color = 'var(--accent-cyan)';
    }

    try {
      await fetchAPI('/api/v1/storage', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ max_storage_gb: gb })
      });
      showToast(`Local storage quota updated to ${gb} GB`, 'success');
      if (statusLabel) {
        statusLabel.textContent = 'Saved successfully!';
        statusLabel.style.color = 'var(--color-success)';
        setTimeout(() => statusLabel.textContent = '', 3000);
      }
      syncMeshState();
    } catch (err) {
      if (statusLabel) {
        statusLabel.textContent = `Failed: ${err.message}`;
        statusLabel.style.color = 'var(--color-danger)';
      }
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

  // Copy Node ID in Settings
  document.getElementById('btn-copy-node-id')?.addEventListener('click', () => {
    const idInput = document.getElementById('setting-node-id');
    if (idInput && idInput.value) {
      copyText(idInput.value, 'Node ID copied to clipboard');
    }
  });
}

// Initialization
document.addEventListener('DOMContentLoaded', () => {
  initEventListeners();
  syncMeshState();
  // Live polling every 3 seconds
  state.pollTimer = setInterval(syncMeshState, 3000);
});
