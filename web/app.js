document.addEventListener('DOMContentLoaded', () => {
  const devicesGrid = document.getElementById('devices-grid');
  const deviceCount = document.getElementById('device-count');
  const daemonHost = document.getElementById('daemon-host');
  const refreshBtn = document.getElementById('refresh-btn');
  const sampleCurl = document.getElementById('sample-curl');
  const copyCurlBtn = document.getElementById('copy-curl-btn');

  // Display current hostname / IP
  const currentHost = window.location.host || 'mini.local:8080';
  daemonHost.textContent = currentHost;

  // Extract auth token from URL if present (?token=xyz)
  const urlParams = new URLSearchParams(window.location.search);
  const authToken = urlParams.get('token') || '';

  function getApiHeaders() {
    const headers = { 'Content-Type': 'application/json' };
    if (authToken) {
      headers['Authorization'] = `Bearer ${authToken}`;
    }
    return headers;
  }

  async function fetchDevices() {
    try {
      refreshBtn.classList.add('rotating');
      const response = await fetch('/api/devices', { headers: getApiHeaders() });
      if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
      }
      const data = await response.json();
      renderDevices(data.devices || []);
    } catch (err) {
      console.error('Failed to load devices:', err);
      showToast('Error connecting to TailWake daemon', 'error');
    } finally {
      setTimeout(() => refreshBtn.classList.remove('rotating'), 500);
    }
  }

  function renderDevices(devices) {
    deviceCount.textContent = `${devices.length} device${devices.length === 1 ? '' : 's'}`;

    if (devices.length === 0) {
      devicesGrid.innerHTML = `
        <div class="empty-state">
          <p>No devices configured in config.json.</p>
        </div>
      `;
      return;
    }

    // Update curl code box example with first device ID
    if (devices[0]) {
      const tokenParam = authToken ? `?token=${authToken}` : '';
      sampleCurl.textContent = `curl http://${currentHost}/wake/${devices[0].id}${tokenParam}`;
    }

    devicesGrid.innerHTML = devices.map(dev => {
      const statusClass = dev.status || 'offline';
      const statusLabel = statusClass === 'online' ? 'Online' : (statusClass === 'waking' ? 'Waking Up...' : 'Sleeping');

      return `
        <div class="device-card" id="card-${dev.id}">
          <div class="card-header">
            <div>
              <h3 class="device-name">${escapeHtml(dev.name)}</h3>
              <span class="device-id-code">ID: ${escapeHtml(dev.id)}</span>
            </div>
            <div class="status-badge ${statusClass}">
              <span class="status-dot"></span>
              <span>${statusLabel}</span>
            </div>
          </div>

          <div class="device-details">
            <div class="detail-row">
              <span>MAC:</span>
              <span>${escapeHtml(dev.mac)}</span>
            </div>
            <div class="detail-row">
              <span>Target IP:</span>
              <span>${escapeHtml(dev.ping_host || dev.broadcast_ip)}</span>
            </div>
          </div>

          <button class="btn-wake" onclick="triggerWake('${dev.id}', '${escapeHtml(dev.name)}')">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M18.36 6.64a9 9 0 1 1-12.73 0"></path>
              <line x1="12" y1="2" x2="12" y2="12"></line>
            </svg>
            <span>Wake Up Machine</span>
          </button>
        </div>
      `;
    }).join('');
  }

  window.triggerWake = async function(deviceId, deviceName) {
    const cardBtn = document.querySelector(`#card-${deviceId} .btn-wake`);
    if (cardBtn) {
      cardBtn.classList.add('loading');
      cardBtn.querySelector('span').textContent = 'Sending Magic Packet...';
    }

    try {
      const tokenParam = authToken ? `?token=${authToken}` : '';
      const response = await fetch(`/api/wake/${deviceId}${tokenParam}`, {
        method: 'POST',
        headers: getApiHeaders()
      });

      const result = await response.json();
      if (response.ok && result.success) {
        showToast(`⚡ Magic packet sent to ${deviceName}!`, 'success');
        // Instantly refresh devices list to reflect waking state
        setTimeout(fetchDevices, 800);
      } else {
        showToast(`Failed: ${result.error || 'Unknown error'}`, 'error');
      }
    } catch (err) {
      console.error('Wake trigger error:', err);
      showToast('Network error triggering Wake-on-LAN', 'error');
    } finally {
      if (cardBtn) {
        cardBtn.classList.remove('loading');
        cardBtn.querySelector('span').textContent = 'Wake Up Machine';
      }
    }
  };

  function showToast(message, type = 'success') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span>${escapeHtml(message)}</span>`;
    container.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateY(10px)';
      setTimeout(() => toast.remove(), 300);
    }, 4000);
  }

  copyCurlBtn.addEventListener('click', () => {
    navigator.clipboard.writeText(sampleCurl.textContent).then(() => {
      showToast('cURL command copied to clipboard!', 'success');
    });
  });

  refreshBtn.addEventListener('click', fetchDevices);

  function escapeHtml(str) {
    return String(str || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Initial load & poll every 10 seconds
  fetchDevices();
  setInterval(fetchDevices, 10000);
});
