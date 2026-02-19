// Meshtastic Web UI Application

const API_BASE = '/api/v1';
let ws = null;
let wsReconnectAttempts = 0;
let selectedNode = null;
let selectedDevice = null;
let connectionType = 'serial';
let isConnected = false;
let nodes = {};
let messages = {};
let channels = [];
let deviceConfig = {};
let leafletMap = null;
let mapMarkers = {};
let selectedChannel = null;
let channelMessages = {}; // Messages keyed by channel index
let channelUnreadCounts = {}; // Unread message counts per channel
let replyingTo = null; // Message being replied to
let myNode = null; // Local node info
let nodeSearchQuery = '';
let nodeSortBy = 'lastHeard';
let nodeFilters = new Set(); // Multiple filters can be active

// DOM Elements
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const nodeList = document.getElementById('nodeList');
const nodeCount = document.getElementById('nodeCount');
const deviceList = document.getElementById('deviceList');
const messagesList = document.getElementById('messagesList');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const scanBtn = document.getElementById('scanBtn');
const connectBtn = document.getElementById('connectBtn');
const nodeDetailsSection = document.getElementById('nodeDetailsSection');
const nodeDetails = document.getElementById('nodeDetails');
const themeToggle = document.getElementById('themeToggle');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    console.log('Meshtastic Web UI initializing...');
    initTabs();
    initConnectionType();
    initWebSocket();
    initThemeToggle();
    initNodeControls();
    loadConnectionStatus();
    loadNodes();
});

// Node Search, Filter, Sort Controls
function initNodeControls() {
    const searchInput = document.getElementById('nodeSearch');
    const sortSelect = document.getElementById('nodeSort');
    const filterToggleBtn = document.getElementById('filterToggleBtn');
    const filterPanel = document.getElementById('nodeFilterPanel');
    const filterBadge = document.getElementById('filterBadge');

    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            nodeSearchQuery = e.target.value.toLowerCase();
            renderNodeList();
        });
    }

    if (sortSelect) {
        sortSelect.addEventListener('change', (e) => {
            nodeSortBy = e.target.value;
            renderNodeList();
        });
    }

    // Filter panel toggle
    if (filterToggleBtn && filterPanel) {
        filterToggleBtn.addEventListener('click', () => {
            filterPanel.classList.toggle('open');
        });
    }

    // Filter chip checkboxes
    const filterChips = document.querySelectorAll('.filter-chip');
    filterChips.forEach(chip => {
        const checkbox = chip.querySelector('input[type="checkbox"]');
        const filterType = chip.dataset.filter;

        chip.addEventListener('click', (e) => {
            e.preventDefault();
            checkbox.checked = !checkbox.checked;

            if (checkbox.checked) {
                nodeFilters.add(filterType);
                chip.classList.add('active');
            } else {
                nodeFilters.delete(filterType);
                chip.classList.remove('active');
            }

            updateFilterBadge();
            renderNodeList();
        });
    });
}

// Update filter badge count
function updateFilterBadge() {
    const filterBadge = document.getElementById('filterBadge');
    const filterToggleBtn = document.getElementById('filterToggleBtn');
    const count = nodeFilters.size;

    if (filterBadge) {
        if (count > 0) {
            filterBadge.textContent = count;
            filterBadge.style.display = 'flex';
            filterToggleBtn?.classList.add('has-filters');
        } else {
            filterBadge.style.display = 'none';
            filterToggleBtn?.classList.remove('has-filters');
        }
    }
}

// Theme Toggle
function initThemeToggle() {
    if (themeToggle) {
        themeToggle.addEventListener('click', () => {
            document.body.classList.toggle('dark');
            // Save preference
            localStorage.setItem('theme', document.body.classList.contains('dark') ? 'dark' : 'light');
        });
    }

    // Load saved preference
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme === 'light') {
        document.body.classList.remove('dark');
    }
}

// Tab Navigation
function initTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));

            tab.classList.add('active');
            const tabPanel = document.getElementById(tab.dataset.tab);
            if (tabPanel) {
                tabPanel.classList.add('active');
            }

            // Load data for specific tabs
            const tabName = tab.dataset.tab;
            if (tabName === 'channels') {
                loadChannels();
            } else if (tabName === 'config') {
                loadConfig();
            } else if (tabName === 'map') {
                initMap();
            } else if (tabName === 'telemetry') {
                updateTelemetryNodeSelector();
            } else if (tabName === 'neighbors') {
                loadNeighborChartNodes();
            }
        });
    });
}

// Connection Type Selection
function initConnectionType() {
    const connectionTypeButtons = document.querySelectorAll('.connection-type button');
    const manualAddressSection = document.getElementById('manualAddressSection');
    const manualAddressInput = document.getElementById('manualAddress');

    connectionTypeButtons.forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();

            // Remove active class from all buttons
            connectionTypeButtons.forEach(b => b.classList.remove('active'));

            // Add active class to clicked button
            btn.classList.add('active');

            // Update connection type
            connectionType = btn.dataset.type;

            // Show/hide manual address input for TCP
            if (manualAddressSection) {
                manualAddressSection.style.display = connectionType === 'tcp' ? 'block' : 'none';
            }

            // Reset device selection
            selectedDevice = null;
            connectBtn.disabled = connectionType !== 'tcp'; // TCP can use manual address

            // Update placeholder text based on type
            const typeLabels = { serial: 'serial ports', ble: 'BLE devices', tcp: 'network devices' };
            deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;"><small>Click Scan to find ${typeLabels[connectionType]}</small></li>`;
        });
    });

    // Manual address input for TCP
    if (manualAddressInput) {
        manualAddressInput.addEventListener('input', () => {
            if (connectionType === 'tcp' && manualAddressInput.value.trim()) {
                selectedDevice = {
                    address: manualAddressInput.value.trim(),
                    name: 'Manual: ' + manualAddressInput.value.trim()
                };
                connectBtn.disabled = false;
                // Deselect any device in the list
                deviceList.querySelectorAll('.device-item').forEach(i => i.classList.remove('selected'));
            } else if (connectionType === 'tcp') {
                selectedDevice = null;
                connectBtn.disabled = true;
            }
        });

        manualAddressInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && selectedDevice) {
                connect();
            }
        });
    }

    scanBtn.addEventListener('click', (e) => {
        e.preventDefault();
        scanDevices();
    });

    connectBtn.addEventListener('click', (e) => {
        e.preventDefault();
        if (isConnected) {
            disconnect();
        } else {
            connect();
        }
    });
}

// WebSocket Connection
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}${API_BASE}/ws`;

    console.log('Connecting to WebSocket:', wsUrl);
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log('WebSocket connected');
        wsReconnectAttempts = 0;
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            console.log('WebSocket message:', data);
            handleWebSocketMessage(data);
        } catch (e) {
            console.error('Failed to parse WebSocket message:', e);
        }
    };

    ws.onclose = () => {
        console.log('WebSocket disconnected');
        wsReconnectAttempts++;
        const delay = Math.min(3000 * wsReconnectAttempts, 30000);
        console.log(`Reconnecting in ${delay}ms...`);
        setTimeout(initWebSocket, delay);
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

// Handle WebSocket Messages
function handleWebSocketMessage(data) {
    console.log('WS event type:', data.type, data);
    switch (data.type) {
        case 'connection.state':
            updateConnectionStatus(data.data.state);
            break;
        case 'node.updated':
            if (data.data.node) {
                updateNode(data.data.node);
            }
            break;
        case 'node.removed':
            removeNode(data.data.num);
            break;
        case 'message.received':
            if (data.data.message) {
                addMessage(data.data.message);
            }
            break;
        case 'message.status':
            updateMessageStatus(data.data.id, data.data.status);
            break;
        case 'channel.updated':
            if (data.data.channel) {
                updateChannel(data.data.channel);
            }
            break;
        case 'config.updated':
            if (data.data.config) {
                deviceConfig = data.data.config;
                renderConfig();
            }
            break;
        case 'moduleConfig.updated':
            // Refresh module cards when a module config is received
            loadModuleStates();
            break;
        case 'mynode.updated':
            if (data.data.node) {
                myNode = data.data.node;
                showMyNode(myNode);
            }
            break;
        case 'metadata.updated':
            if (data.data.metadata) {
                Object.assign(deviceConfig, data.data.metadata);
                renderConfig();
            }
            break;
        case 'traceroute.response':
            handleTracerouteResponse(data.data);
            break;
        case 'scan.started':
            handleScanStarted(data.data);
            break;
        case 'scan.device':
            handleScanDevice(data.data);
            break;
        case 'scan.complete':
            handleScanComplete(data.data);
            break;
        case 'scan.error':
            handleScanError(data.data);
            break;
        case 'telemetry.device':
            handleDeviceTelemetry(data.data);
            break;
        case 'telemetry.environment':
            handleEnvironmentTelemetry(data.data);
            break;
        case 'telemetry.power':
            handlePowerTelemetry(data.data);
            break;
        case 'telemetry.airQuality':
            handleAirQualityTelemetry(data.data);
            break;
        case 'telemetry.localStats':
            handleLocalStatsTelemetry(data.data);
            break;
        case 'neighborinfo.updated':
            handleNeighborInfoUpdated(data.data);
            break;
        case 'position.updated':
            handlePositionUpdated(data.data);
            break;
        case 'waypoint.received':
        case 'waypoint.created':
        case 'waypoint.deleted':
            handleWaypointEvent(data);
            break;
        default:
            console.log('Unknown WebSocket message type:', data.type);
    }
}

// API Functions
async function api(method, endpoint, body = null) {
    const options = {
        method,
        headers: {
            'Content-Type': 'application/json'
        }
    };

    if (body) {
        options.body = JSON.stringify(body);
    }

    console.log(`API ${method} ${endpoint}`, body);

    try {
        const response = await fetch(`${API_BASE}${endpoint}`, options);
        const data = await response.json();
        console.log(`API response:`, data);

        if (!response.ok) {
            throw new Error(data.error || `HTTP ${response.status}`);
        }

        return data;
    } catch (e) {
        console.error(`API error:`, e);
        throw e;
    }
}

// Connection Functions
async function loadConnectionStatus() {
    try {
        const data = await api('GET', '/connection/status');
        updateConnectionStatus(data.state);
        if (data.connected) {
            connectionType = data.type || 'serial';
            loadNodes();
            loadChannels();
            loadConfig();
        }
    } catch (e) {
        console.error('Failed to load connection status:', e);
    }
}

function updateConnectionStatus(state) {
    statusDot.className = 'status-dot';

    if (state === 'connected') {
        statusDot.classList.add('connected');
        statusText.textContent = 'Connected';
        connectBtn.textContent = 'Disconnect';
        connectBtn.disabled = false;
        isConnected = true;
        loadChannels();
        loadConfig();
        loadMyNode();
    } else if (state === 'connecting') {
        statusDot.classList.add('connecting');
        statusText.textContent = 'Connecting...';
        connectBtn.disabled = true;
    } else {
        statusText.textContent = 'Disconnected';
        connectBtn.textContent = 'Connect';
        connectBtn.disabled = !selectedDevice;
        isConnected = false;
        hideMyNode();
    }
}

// Load local node info
async function loadMyNode() {
    try {
        const data = await api('GET', '/nodes/me');
        myNode = data;
        showMyNode(myNode);
    } catch (e) {
        console.error('Failed to load my node:', e);
        myNode = null;
    }
}

// Show local node in header
function showMyNode(node) {
    const myNodeInfo = document.getElementById('myNodeInfo');
    const myNodeAvatar = document.getElementById('myNodeAvatar');
    const myNodeName = document.getElementById('myNodeName');
    const myNodeId = document.getElementById('myNodeId');

    if (!myNodeInfo || !node) return;

    const shortName = node.shortName || '??';
    myNodeAvatar.textContent = shortName.substring(0, 4).toUpperCase();
    myNodeName.textContent = node.longName || node.shortName || 'My Node';
    myNodeId.textContent = `!${(node.num >>> 0).toString(16).toLowerCase()}`;
    myNodeInfo.style.display = 'flex';
}

// Hide local node info
function hideMyNode() {
    const myNodeInfo = document.getElementById('myNodeInfo');
    if (myNodeInfo) {
        myNodeInfo.style.display = 'none';
    }
    myNode = null;
}

let isScanning = false;
let scannedDevices = []; // Track devices found during streaming scan

async function scanDevices() {
    if (isScanning) {
        // Cancel the scan
        await cancelScan();
        return;
    }

    isScanning = true;
    scannedDevices = [];
    scanBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg> Cancel';
    scanBtn.classList.add('scanning');

    // Type-specific scan messages
    const scanMessages = {
        serial: 'Scanning USB ports...',
        ble: 'Scanning for Bluetooth devices...',
        tcp: 'Discovering network devices (mDNS)...'
    };

    deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" class="spin">
            <path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/>
        </svg>
        <small>${scanMessages[connectionType] || 'Scanning...'}</small>
    </li>`;

    // Clear manual address when scanning TCP
    const manualAddressInput = document.getElementById('manualAddress');
    if (manualAddressInput && connectionType === 'tcp') {
        manualAddressInput.value = '';
    }

    try {
        // Use streaming mode for BLE - devices arrive via WebSocket
        if (connectionType === 'ble') {
            const data = await api('GET', `/connection/devices?type=${connectionType}&streaming=true`);
            // Response is immediate, devices come via WebSocket events
            console.log('BLE streaming scan started:', data);
            return; // Don't reset scanning state - WebSocket events will handle it
        }

        // Regular scan for serial/tcp
        const data = await api('GET', `/connection/devices?type=${connectionType}`);
        displayScannedDevices(data.devices || []);
    } catch (e) {
        console.error('Scan failed:', e);
        if (e.message !== 'Scan cancelled') {
            deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;"><small>Scan failed: ${e.message}</small></li>`;
        }
        resetScanButton();
    }
}

function displayScannedDevices(devices) {
    const manualAddressInput = document.getElementById('manualAddress');

    if (devices && devices.length > 0) {
        deviceList.innerHTML = devices.map(device => {
            const icon = getDeviceIcon(device.type || connectionType);
            const rssiDisplay = device.rssi ? `<span class="device-rssi">${device.rssi} dBm</span>` : '';
            return `
            <li class="device-item" data-address="${device.address}" data-name="${device.name || device.address}">
                <div class="device-info">
                    ${icon}
                    <div>
                        <div class="device-name">${device.name || 'Meshtastic Device'}</div>
                        <div class="device-address">${device.address}</div>
                    </div>
                </div>
                ${rssiDisplay}
            </li>
        `}).join('');

        // Add click handlers
        deviceList.querySelectorAll('.device-item').forEach(item => {
            item.addEventListener('click', () => {
                deviceList.querySelectorAll('.device-item').forEach(i => i.classList.remove('selected'));
                item.classList.add('selected');
                selectedDevice = {
                    address: item.dataset.address,
                    name: item.dataset.name
                };
                connectBtn.disabled = false;
                // Clear manual input when selecting from list
                if (manualAddressInput) manualAddressInput.value = '';
            });
        });
    } else {
        const emptyMessages = {
            serial: 'No serial ports found. Check USB connection.',
            ble: 'No BLE devices found. Ensure device is powered on.',
            tcp: 'No network devices found. Try entering IP manually.'
        };
        deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;"><small>${emptyMessages[connectionType] || 'No devices found'}</small></li>`;
    }

    resetScanButton();
}

function resetScanButton() {
    isScanning = false;
    scanBtn.classList.remove('scanning');
    scanBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg> Scan';
}

// Streaming scan event handlers
function handleScanStarted(data) {
    console.log('Scan started:', data);
    scannedDevices = [];
}

function handleScanDevice(data) {
    console.log('Device found:', data.device);
    const device = data.device;

    // Add to tracked devices
    scannedDevices.push(device);

    // Update the UI immediately with the new device
    const manualAddressInput = document.getElementById('manualAddress');
    const icon = getDeviceIcon(device.type || connectionType);
    const rssiDisplay = device.rssi ? `<span class="device-rssi">${device.rssi} dBm</span>` : '';

    const deviceHtml = `
        <li class="device-item" data-address="${device.address}" data-name="${device.name || device.address}">
            <div class="device-info">
                ${icon}
                <div>
                    <div class="device-name">${device.name || 'Meshtastic Device'}</div>
                    <div class="device-address">${device.address}</div>
                </div>
            </div>
            ${rssiDisplay}
        </li>
    `;

    // If this is the first device, replace the "scanning" message
    if (scannedDevices.length === 1) {
        deviceList.innerHTML = deviceHtml;
    } else {
        // Append to existing list
        deviceList.insertAdjacentHTML('beforeend', deviceHtml);
    }

    // Add click handler to the new device item
    const newItem = deviceList.querySelector(`.device-item[data-address="${device.address}"]`);
    if (newItem) {
        newItem.addEventListener('click', () => {
            deviceList.querySelectorAll('.device-item').forEach(i => i.classList.remove('selected'));
            newItem.classList.add('selected');
            selectedDevice = {
                address: newItem.dataset.address,
                name: newItem.dataset.name
            };
            connectBtn.disabled = false;
            if (manualAddressInput) manualAddressInput.value = '';
        });
    }
}

function handleScanComplete(data) {
    console.log('Scan complete:', data);

    if (scannedDevices.length === 0) {
        const emptyMessages = {
            serial: 'No serial ports found. Check USB connection.',
            ble: 'No BLE devices found. Ensure device is powered on.',
            tcp: 'No network devices found. Try entering IP manually.'
        };
        deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;"><small>${emptyMessages[data.type || connectionType] || 'No devices found'}</small></li>`;
    }

    resetScanButton();
}

function handleScanError(data) {
    console.error('Scan error:', data.error);
    deviceList.innerHTML = `<li class="empty-state" style="padding: 1rem;"><small>Scan failed: ${data.error}</small></li>`;
    resetScanButton();
}

function getDeviceIcon(type) {
    switch(type) {
        case 'serial':
            return '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style="color: var(--primary); flex-shrink: 0;"><path d="M15 7v4h1v2h-3V5h2l-3-4-3 4h2v8H8v-2.07c.7-.37 1.2-1.08 1.2-1.93 0-1.21-.99-2.2-2.2-2.2-1.21 0-2.2.99-2.2 2.2 0 .85.5 1.56 1.2 1.93V13c0 1.11.89 2 2 2h3v3.05c-.71.37-1.2 1.1-1.2 1.95 0 1.22.99 2.2 2.2 2.2 1.21 0 2.2-.98 2.2-2.2 0-.85-.49-1.58-1.2-1.95V15h3c1.11 0 2-.89 2-2v-2h1V7h-4z"/></svg>';
        case 'ble':
            return '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style="color: #0082fc; flex-shrink: 0;"><path d="M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29 4.3-4.29zM13 5.83l1.88 1.88L13 9.59V5.83zm1.88 10.46L13 18.17v-3.76l1.88 1.88z"/></svg>';
        case 'tcp':
            return '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style="color: #4caf50; flex-shrink: 0;"><path d="M1 9l2 2c4.97-4.97 13.03-4.97 18 0l2-2C16.93 2.93 7.08 2.93 1 9zm8 8l3 3 3-3c-1.65-1.66-4.34-1.66-6 0zm-4-4l2 2c2.76-2.76 7.24-2.76 10 0l2-2C15.14 9.14 8.87 9.14 5 13z"/></svg>';
        default:
            return '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" style="color: var(--on-surface-variant); flex-shrink: 0;"><path d="M17 1.01L7 1c-1.1 0-2 .9-2 2v18c0 1.1.9 2 2 2h10c1.1 0 2-.9 2-2V3c0-1.1-.9-1.99-2-1.99zM17 19H7V5h10v14z"/></svg>';
    }
}

async function cancelScan() {
    try {
        await api('POST', '/connection/cancel-scan');
        deviceList.innerHTML = '<li class="empty-state" style="padding: 1rem;"><small>Scan cancelled</small></li>';
    } catch (e) {
        console.error('Cancel scan failed:', e);
    } finally {
        // Reset button state
        isScanning = false;
        scanBtn.classList.remove('scanning');
        scanBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg> Scan';
    }
}

async function connect() {
    if (!selectedDevice) return;

    connectBtn.disabled = true;
    updateConnectionStatus('connecting');

    try {
        await api('POST', '/connection/connect', {
            type: connectionType,
            address: selectedDevice.address
        });
    } catch (e) {
        console.error('Connect failed:', e);
        updateConnectionStatus('disconnected');
        alert('Connection failed: ' + e.message);
    }
}

async function disconnect() {
    try {
        await api('POST', '/connection/disconnect');
        updateConnectionStatus('disconnected');
    } catch (e) {
        console.error('Disconnect failed:', e);
    }
}

// Node Functions
async function loadNodes() {
    try {
        const data = await api('GET', '/nodes');
        nodes = {};
        (data.nodes || []).forEach(node => {
            nodes[node.num] = node;
        });
        renderNodeList();
        updateMapMarkers();
    } catch (e) {
        console.error('Failed to load nodes:', e);
    }
}

function updateNode(node) {
    nodes[node.num] = node;
    renderNodeList();
    updateMapMarker(node);
}

function removeNode(num) {
    delete nodes[num];
    renderNodeList();
    removeMapMarker(num);
}

function renderNodeList() {
    let nodeArray = Object.values(nodes);
    const totalCount = nodeArray.length;

    if (totalCount === 0) {
        nodeCount.textContent = '0';
        nodeList.innerHTML = '<li class="empty-state"><p>No nodes found</p><small>Connect to a device to discover nodes</small></li>';
        return;
    }

    // Apply search filter
    if (nodeSearchQuery) {
        nodeArray = nodeArray.filter(node => {
            const searchFields = [
                node.longName || '',
                node.shortName || '',
                (node.num >>> 0).toString(16),
                node.num.toString()
            ].map(s => s.toLowerCase());
            return searchFields.some(field => field.includes(nodeSearchQuery));
        });
    }

    // Apply checkbox filters (AND logic - node must match ALL active filters)
    const now = Date.now() / 1000;
    if (nodeFilters.size > 0) {
        nodeArray = nodeArray.filter(n => {
            for (const filter of nodeFilters) {
                switch (filter) {
                    case 'online':
                        if (!(n.lastHeard && (now - n.lastHeard) < 3600)) return false;
                        break;
                    case 'recent':
                        if (!(n.lastHeard && (now - n.lastHeard) < 300)) return false;
                        break;
                    case 'favorites':
                        if (!n.isFavorite) return false;
                        break;
                    case 'hasPosition':
                        if (!(n.latitude && n.longitude)) return false;
                        break;
                    case 'direct':
                        if (n.hopsAway !== 0) return false;
                        break;
                    case 'mqtt':
                        if (!n.viaMqtt) return false;
                        break;
                    case 'hasBattery':
                        if (!(n.batteryLevel && n.batteryLevel > 0 && n.batteryLevel <= 100)) return false;
                        break;
                }
            }
            return true;
        });
    }

    // Apply sort
    switch (nodeSortBy) {
        case 'lastHeard':
            nodeArray.sort((a, b) => (b.lastHeard || 0) - (a.lastHeard || 0));
            break;
        case 'name':
            nodeArray.sort((a, b) => {
                const nameA = (a.longName || a.shortName || '').toLowerCase();
                const nameB = (b.longName || b.shortName || '').toLowerCase();
                return nameA.localeCompare(nameB);
            });
            break;
        case 'distance':
            // Sort by distance from my node (if position available)
            if (myNode && myNode.latitude && myNode.longitude) {
                nodeArray.sort((a, b) => {
                    const distA = getDistance(myNode, a);
                    const distB = getDistance(myNode, b);
                    return distA - distB;
                });
            }
            break;
        case 'hops':
            nodeArray.sort((a, b) => (a.hopsAway ?? 99) - (b.hopsAway ?? 99));
            break;
        case 'snr':
            nodeArray.sort((a, b) => (b.snr ?? -999) - (a.snr ?? -999));
            break;
        case 'battery':
            nodeArray.sort((a, b) => (a.batteryLevel || 999) - (b.batteryLevel || 999));
            break;
        case 'chUtil':
            nodeArray.sort((a, b) => (b.channelUtilization || 0) - (a.channelUtilization || 0));
            break;
    }

    // Update count (filtered/total)
    nodeCount.textContent = nodeArray.length === totalCount ? totalCount : `${nodeArray.length}/${totalCount}`;

    if (nodeArray.length === 0) {
        nodeList.innerHTML = '<li class="empty-state"><p>No matching nodes</p><small>Try adjusting your search or filters</small></li>';
        return;
    }

    nodeList.innerHTML = nodeArray.map(node => {
        const shortName = node.shortName || '??';
        const initials = shortName.substring(0, 4).toUpperCase();
        const isSelected = selectedNode && selectedNode.num === node.num;
        const isOnline = node.lastHeard && (now - node.lastHeard) < 3600;
        const isRecent = node.lastHeard && (now - node.lastHeard) < 300; // Last 5 min

        // Build badges
        let badges = '';
        if (node.viaMqtt) {
            badges += '<span class="node-badge mqtt">MQTT</span>';
        } else if (node.hopsAway === 0) {
            badges += '<span class="node-badge direct">Direct</span>';
        } else if (node.hopsAway > 0) {
            badges += `<span class="node-badge">${node.hopsAway} hop${node.hopsAway > 1 ? 's' : ''}</span>`;
        }

        // Battery display
        let batteryHtml = '';
        if (node.batteryLevel && node.batteryLevel > 0 && node.batteryLevel <= 100) {
            const batteryClass = node.batteryLevel <= 20 ? 'danger' : node.batteryLevel <= 40 ? 'warning' : '';
            batteryHtml = `
                <span class="node-battery">
                    <span class="battery-icon"><span class="battery-level ${batteryClass}" style="width: ${node.batteryLevel}%"></span></span>
                    ${node.batteryLevel}%
                </span>`;
        }

        // Signal quality
        let signalHtml = '';
        if (node.snr !== undefined && node.snr !== 0 && node.snr < 100) {
            const snrQuality = node.snr > 5 ? 'good' : node.snr > 0 ? 'ok' : node.snr > -10 ? 'weak' : 'poor';
            signalHtml = `<span class="node-metric" title="SNR: ${node.snr.toFixed(1)} dB">${getSignalIcon(snrQuality)} ${node.snr.toFixed(1)}</span>`;
        }

        // Distance and bearing
        let positionHtml = '';
        if (node.distanceStr && node.bearingCardinal) {
            positionHtml = `
                <span class="node-distance">
                    <span class="bearing-arrow">${getBearingArrow(node.bearing)}</span>
                    ${node.distanceStr} ${node.bearingCardinal}
                </span>`;
        } else if (node.latitude && node.longitude && myNode && myNode.latitude && myNode.longitude) {
            const dist = getDistance(myNode, node);
            if (dist < Infinity && dist > 0) {
                const distStr = dist < 1 ? `${Math.round(dist * 1000)} m` : `${dist.toFixed(1)} km`;
                const bearing = calculateBearing(myNode.latitude, myNode.longitude, node.latitude, node.longitude);
                positionHtml = `
                    <span class="node-distance">
                        <span class="bearing-arrow">${getBearingArrow(bearing)}</span>
                        ${distStr} ${bearingToCardinal(bearing)}
                    </span>`;
            }
        }

        // Hardware model (shortened)
        let hwHtml = '';
        if (node.hardwareModel) {
            const hw = formatHwShort(node.hardwareModel);
            hwHtml = `<span class="node-hardware">${hw}</span>`;
        }

        // Metrics row (channel util, air util)
        let metricsRow = '';
        const hasMetrics = batteryHtml || signalHtml || node.channelUtilization || node.airUtilTx;
        if (hasMetrics) {
            metricsRow = `
                <div class="node-metrics-row">
                    ${batteryHtml}
                    ${signalHtml}
                    ${node.channelUtilization ? `<span class="node-metric" title="Channel Utilization">CH ${node.channelUtilization.toFixed(0)}%</span>` : ''}
                    ${node.airUtilTx ? `<span class="node-metric" title="Air Utilization TX">TX ${node.airUtilTx.toFixed(0)}%</span>` : ''}
                </div>`;
        }

        // Position row
        let posRow = '';
        if (positionHtml || hwHtml) {
            posRow = `
                <div class="node-position-row">
                    ${positionHtml}
                    ${hwHtml}
                    <span class="node-lastHeard">${formatLastHeard(node.lastHeard)}</span>
                </div>`;
        } else {
            posRow = `
                <div class="node-position-row">
                    <span class="node-lastHeard">${formatLastHeard(node.lastHeard)}</span>
                </div>`;
        }

        return `
        <li class="node-item-enhanced ${isSelected ? 'selected' : ''}" data-num="${node.num}">
            <div class="node-avatar-enhanced ${isOnline ? 'online' : ''} ${node.viaMqtt ? 'mqtt' : ''}">
                ${initials}
                ${node.isFavorite ? '<span class="node-favorite-badge">★</span>' : ''}
            </div>
            <div class="node-content-enhanced">
                <div class="node-header-row">
                    <span class="node-name-enhanced">${escapeHtml(node.longName || node.shortName || `Node ${node.num}`)}</span>
                    <div class="node-badges">${badges}</div>
                </div>
                ${metricsRow}
                ${posRow}
            </div>
        </li>
    `}).join('');

    // Add click handlers
    nodeList.querySelectorAll('.node-item-enhanced').forEach(item => {
        item.addEventListener('click', () => selectNode(parseInt(item.dataset.num)));
    });

    // Update mobile sidebar nodes
    if (typeof updateMobileSidebarNodes === 'function') {
        updateMobileSidebarNodes();
    }
}

// Calculate distance between two nodes (Haversine formula)
function getDistance(node1, node2) {
    if (!node1.latitude || !node1.longitude || !node2.latitude || !node2.longitude) {
        return Infinity;
    }
    const R = 6371; // Earth's radius in km
    const dLat = (node2.latitude - node1.latitude) * Math.PI / 180;
    const dLon = (node2.longitude - node1.longitude) * Math.PI / 180;
    const a = Math.sin(dLat/2) * Math.sin(dLat/2) +
              Math.cos(node1.latitude * Math.PI / 180) * Math.cos(node2.latitude * Math.PI / 180) *
              Math.sin(dLon/2) * Math.sin(dLon/2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
    return R * c;
}

function selectNode(nodeNum) {
    selectedNode = nodes[nodeNum];
    if (!selectedNode) return;

    selectedChannel = null; // Deselect channel when selecting node

    // Update node list UI
    nodeList.querySelectorAll('.node-item-enhanced').forEach(item => {
        item.classList.toggle('selected', parseInt(item.dataset.num) === nodeNum);
    });

    // Update channel list UI - deselect any selected channel
    document.querySelectorAll('.channel-card').forEach(card => {
        card.classList.remove('selected');
    });

    // Update chat header
    const chatHeader = document.getElementById('chatHeader');
    const chatHeaderAvatar = document.getElementById('chatHeaderAvatar');
    const chatHeaderName = document.getElementById('chatHeaderName');
    const chatHeaderType = document.getElementById('chatHeaderType');

    if (chatHeader && chatHeaderAvatar && chatHeaderName && chatHeaderType) {
        chatHeader.style.display = 'flex';
        const shortName = selectedNode.shortName || '??';
        chatHeaderAvatar.textContent = shortName.substring(0, 4).toUpperCase();
        chatHeaderAvatar.className = 'chat-header-avatar';
        chatHeaderName.textContent = selectedNode.longName || selectedNode.shortName || `Node ${nodeNum}`;
        chatHeaderType.textContent = `Direct Message · !${(nodeNum >>> 0).toString(16).toLowerCase()}`;
    }

    renderNodeDetails(selectedNode);
    loadMessages(selectedNode.num);

    // Enable message input
    const msgInput = document.getElementById('messageInput');
    const sendButton = document.getElementById('sendBtn');

    if (msgInput) {
        msgInput.disabled = false;
        msgInput.placeholder = `Message to ${selectedNode.shortName || selectedNode.longName || 'node'}...`;
    }
    if (sendButton) {
        sendButton.disabled = false;
    }
    const quickBtn = document.getElementById('quickChatBtn');
    if (quickBtn) {
        quickBtn.disabled = false;
    }

    // Update favorite button state
    updateFavoriteButton();
}

function renderNodeDetails(node) {
    nodeDetailsSection.style.display = 'block';

    // Calculate distance if we have positions
    let distanceStr = null;
    if (myNode && myNode.latitude && myNode.longitude && node.latitude && node.longitude) {
        const dist = getDistance(myNode, node);
        distanceStr = dist < 1 ? `${(dist * 1000).toFixed(0)} m` : `${dist.toFixed(2)} km`;
    }

    let html = '';

    // Node Info Section
    html += `<div class="detail-section">
        <div class="detail-section-title">Node Info</div>
        ${detailRow('Long Name', node.longName || '-')}
        ${detailRow('Short Name', node.shortName || '-')}
        ${detailRow('Node ID', `!${(node.num >>> 0).toString(16).toLowerCase()}`)}
        ${detailRow('Hardware', node.hardwareModel || '-')}
        ${detailRow('Role', node.role || '-')}
        ${detailRow('Last Heard', formatLastHeard(node.lastHeard))}
    </div>`;

    // Radio Section
    html += `<div class="detail-section">
        <div class="detail-section-title">Radio</div>
        ${detailRow('SNR', node.snr !== undefined && node.snr < 100 ? `${node.snr.toFixed(1)} dB` : '-')}
        ${detailRow('RSSI', node.rssi !== undefined && node.rssi > -1000 ? `${node.rssi} dBm` : '-')}
        ${detailRow('Hops Away', node.hopsAway !== undefined && node.hopsAway >= 0 ? node.hopsAway : '-')}
        ${detailRow('Channel', node.channel !== undefined ? node.channel : '-')}
        ${detailRow('Via MQTT', node.viaMqtt ? 'Yes' : 'No')}
    </div>`;

    // Position Section
    if (node.latitude && node.longitude) {
        html += `<div class="detail-section">
            <div class="detail-section-title">Position</div>
            ${detailRow('Latitude', node.latitude.toFixed(6) + '°')}
            ${detailRow('Longitude', node.longitude.toFixed(6) + '°')}
            ${node.altitude ? detailRow('Altitude', `${node.altitude} m`) : ''}
            ${distanceStr ? detailRow('Distance', distanceStr) : ''}
            ${node.positionTime ? detailRow('Updated', formatLastHeard(node.positionTime)) : ''}
        </div>`;
    }

    // Device Telemetry Section
    const hasDeviceTelemetry = node.batteryLevel || node.voltage || node.channelUtilization || node.airUtilTx || node.uptime;
    if (hasDeviceTelemetry) {
        html += `<div class="detail-section">
            <div class="detail-section-title">Device</div>
            ${node.batteryLevel ? detailRow('Battery', `${node.batteryLevel}%`, getBatteryClass(node.batteryLevel)) : ''}
            ${node.voltage ? detailRow('Voltage', `${node.voltage.toFixed(2)} V`) : ''}
            ${node.channelUtilization ? detailRow('Ch. Util', `${node.channelUtilization.toFixed(1)}%`) : ''}
            ${node.airUtilTx ? detailRow('Air Util TX', `${node.airUtilTx.toFixed(1)}%`) : ''}
            ${node.uptime ? detailRow('Uptime', formatUptime(node.uptime)) : ''}
        </div>`;
    }

    // Environment Telemetry Section
    const hasEnvTelemetry = node.temperature || node.relativeHumidity || node.barometricPressure;
    if (hasEnvTelemetry) {
        html += `<div class="detail-section">
            <div class="detail-section-title">Environment</div>
            ${node.temperature ? detailRow('Temperature', `${node.temperature.toFixed(1)} °C`) : ''}
            ${node.relativeHumidity ? detailRow('Humidity', `${node.relativeHumidity.toFixed(1)}%`) : ''}
            ${node.barometricPressure ? detailRow('Pressure', `${node.barometricPressure.toFixed(1)} hPa`) : ''}
            ${node.iaq ? detailRow('Air Quality', node.iaq) : ''}
        </div>`;
    }

    nodeDetails.innerHTML = html;
}

function detailRow(label, value, valueClass = '') {
    return `<div class="detail-row">
        <span class="detail-label">${label}</span>
        <span class="detail-value ${valueClass}">${value}</span>
    </div>`;
}

function getBatteryClass(level) {
    if (level <= 20) return 'error';
    if (level <= 40) return 'warning';
    return '';
}

function formatUptime(seconds) {
    if (!seconds) return '-';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m`;
}

// Messages
async function loadMessages(nodeNum) {
    const contactKey = `!${(nodeNum >>> 0).toString(16)}`;

    try {
        const data = await api('GET', `/messages/${encodeURIComponent(contactKey)}?limit=50`);
        messages[contactKey] = data.messages || [];
        renderMessages();
    } catch (e) {
        console.error('Failed to load messages:', e);
        messages[contactKey] = [];
        renderMessages();
    }
}

function addMessage(message) {
    if (!message) return;

    // Determine if this is a broadcast (channel message) or DM
    // Broadcast address is 0xFFFFFFFF (4294967295 unsigned, or -1 signed)
    const toUnsigned = message.to >>> 0;
    const isBroadcast = toUnsigned === 0xFFFFFFFF;

    // Handle channel messages (broadcast only)
    if (isBroadcast) {
        const channelIdx = message.channel || 0;
        if (!channelMessages[channelIdx]) {
            channelMessages[channelIdx] = [];
        }

        // Check if message already exists (by packetId)
        const existsInChannel = channelMessages[channelIdx].some(m => m.packetId === message.packetId);
        if (!existsInChannel) {
            channelMessages[channelIdx].push(message);

            // Update unread count if not viewing this channel
            if (!selectedChannel || selectedChannel.index !== channelIdx) {
                channelUnreadCounts[channelIdx] = (channelUnreadCounts[channelIdx] || 0) + 1;
                renderChannels();
            } else {
                // Re-render messages if viewing this channel
                renderMessages();
            }
        }
        return; // Don't process as DM
    }

    // Handle DM messages (non-broadcast, with contactKey)
    if (message.contactKey) {
        if (!messages[message.contactKey]) {
            messages[message.contactKey] = [];
        }

        // Check if message already exists (by packetId)
        const exists = messages[message.contactKey].some(m => m.packetId === message.packetId);
        if (!exists) {
            messages[message.contactKey].push(message);
        }

        if (selectedNode) {
            const contactKey = `!${(selectedNode.num >>> 0).toString(16)}`;
            if (message.contactKey === contactKey) {
                renderMessages();
            }
        }
    }
}

function updateMessageStatus(packetId, status) {
    console.log('Message status update:', packetId, status);

    // Find and update the message in our data
    for (const contactKey in messages) {
        const msgList = messages[contactKey];
        const msg = msgList.find(m => m.packetId === packetId);
        if (msg) {
            msg.status = status;
            break;
        }
    }

    // Update the DOM element if visible
    const statusEl = document.querySelector(`.message[data-packet-id="${packetId}"] .message-status`);
    if (statusEl) {
        statusEl.innerHTML = getStatusIcon(status);
        statusEl.className = `message-status status-${status}`;
    }
}

function getStatusIcon(status) {
    switch (status) {
        case 'queued':
            return '<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" title="Queued"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2"/></svg>';
        case 'enroute':
            return '<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" title="Sending"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"/></svg>';
        case 'delivered':
            return '<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" title="Delivered"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>';
        case 'error':
            return '<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" title="Error"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>';
        default:
            return '';
    }
}

// Common emoji reactions
const quickEmojis = ['👍', '❤️', '😂', '😮', '😢', '🙏'];

function renderReactions(reactions) {
    if (!reactions || reactions.length === 0) return '';

    // Group reactions by emoji
    const grouped = {};
    reactions.forEach(r => {
        if (!grouped[r.emoji]) {
            grouped[r.emoji] = [];
        }
        grouped[r.emoji].push(r.userId);
    });

    return `<div class="message-reactions">
        ${Object.entries(grouped).map(([emoji, users]) => `
            <span class="reaction" title="${users.join(', ')}">
                ${emoji} ${users.length > 1 ? users.length : ''}
            </span>
        `).join('')}
    </div>`;
}

function renderSignalInfo(msg) {
    const parts = [];
    if (msg.snr !== undefined && msg.snr !== 0) {
        parts.push(`SNR: ${msg.snr.toFixed(1)}`);
    }
    if (msg.rssi !== undefined && msg.rssi !== 0) {
        parts.push(`RSSI: ${msg.rssi}`);
    }
    if (msg.hopsAway !== undefined && msg.hopsAway >= 0) {
        parts.push(`${msg.hopsAway} hop${msg.hopsAway !== 1 ? 's' : ''}`);
    }
    if (msg.viaMqtt) {
        parts.push('via MQTT');
    }

    if (parts.length === 0) return '';
    return `<div class="message-signal">${parts.join(' · ')}</div>`;
}

let isLoadingMore = false;
let hasMoreMessages = true;

function renderMessages() {
    const chatHeader = document.getElementById('chatHeader');

    // If channel is selected, show channel messages
    if (selectedChannel) {
        const msgList = channelMessages[selectedChannel.index] || [];

        if (msgList.length === 0) {
            messagesList.innerHTML = `<div class="empty-state"><p>No messages in ${selectedChannel.name || 'this channel'}</p><small>Send a message to the channel</small></div>`;
            return;
        }

        // Sort messages chronologically (oldest first)
        const sortedList = [...msgList].sort((a, b) => (a.time || 0) - (b.time || 0));
        messagesList.innerHTML = sortedList.map(msg => renderMessageBubble(msg, true)).join('');
        messagesList.scrollTop = messagesList.scrollHeight;
        setupMessageInteractions();
        return;
    }

    // If node is selected, show direct messages
    if (!selectedNode) {
        // Hide chat header when nothing selected
        if (chatHeader) chatHeader.style.display = 'none';
        messagesList.innerHTML = '<div class="empty-state"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M20 2H4c-1.1 0-1.99.9-1.99 2L2 22l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z"/></svg><p>Select a node or channel to start chatting</p></div>';
        return;
    }

    const contactKey = `!${(selectedNode.num >>> 0).toString(16)}`;
    const msgList = messages[contactKey] || [];

    if (msgList.length === 0) {
        messagesList.innerHTML = '<div class="empty-state"><p>No messages yet</p><small>Send a message to start the conversation</small></div>';
        return;
    }

    // Add load more indicator at top if there might be more messages
    const loadMoreHtml = hasMoreMessages ? `
        <div class="load-more-indicator" id="loadMoreIndicator">
            <small>Scroll up to load more</small>
        </div>
    ` : '';

    // Sort messages chronologically (oldest first)
    const sortedList = [...msgList].sort((a, b) => (a.time || 0) - (b.time || 0));
    messagesList.innerHTML = loadMoreHtml + sortedList.map(msg => renderMessageBubble(msg, false)).join('');
    messagesList.scrollTop = messagesList.scrollHeight;
    setupMessageInteractions();
    setupInfiniteScroll();
}

function renderMessageBubble(msg, isChannel) {
    const isMe = myNode && (msg.from === myNode.num || msg.from === 0);
    let senderName = 'Remote';

    // Helper function to format node name nicely
    const formatNodeName = (node, nodeNum) => {
        if (!node) return `!${(nodeNum >>> 0).toString(16)}`;
        if (node.shortName && node.longName && node.shortName !== node.longName) {
            return `${node.shortName} (${node.longName})`;
        }
        return node.longName || node.shortName || `!${(nodeNum >>> 0).toString(16)}`;
    };

    if (isChannel) {
        // Use >>> 0 to convert signed int32 to unsigned for node lookup
        const senderNode = nodes[msg.from >>> 0];
        senderName = formatNodeName(senderNode, msg.from);
    } else if (selectedNode) {
        senderName = formatNodeName(selectedNode, selectedNode.num);
    }

    const statusHtml = isMe && msg.status ? `
        <span class="message-status status-${msg.status}" title="${msg.status}">
            ${getStatusIcon(msg.status)}
        </span>
    ` : '';

    const reactionsHtml = renderReactions(msg.emojis);
    const signalHtml = !isMe ? renderSignalInfo(msg) : '';

    // Build reply context if this message is a reply
    let replyContextHtml = '';
    if (msg.replyTo) {
        const repliedMsg = findMessageByPacketId(msg.replyTo);
        if (repliedMsg) {
            // Use >>> 0 to convert signed int32 to unsigned for node lookup
            const repliedSenderNode = nodes[repliedMsg.from >>> 0];
            const repliedSenderName = (repliedMsg.from >>> 0) === myNode?.num ? 'You' :
                (repliedSenderNode ? (repliedSenderNode.shortName || repliedSenderNode.longName) : `!${(repliedMsg.from >>> 0).toString(16)}`);
            replyContextHtml = `
                <div class="reply-context" onclick="scrollToMessage(${msg.replyTo})">
                    <span class="reply-context-sender">${repliedSenderName}</span>
                    <span class="reply-context-text">${escapeHtml(repliedMsg.text?.substring(0, 50) || '')}${repliedMsg.text?.length > 50 ? '...' : ''}</span>
                </div>
            `;
        } else {
            replyContextHtml = `
                <div class="reply-context">
                    <span class="reply-context-text">Original message not found</span>
                </div>
            `;
        }
    }

    return `
        <div class="message ${isMe ? 'sent' : ''}" data-packet-id="${msg.packetId || ''}" data-uuid="${msg.uuid || ''}" data-from="${msg.from || ''}" data-text="${escapeHtml(msg.text || '')}">
            ${replyContextHtml}
            <div class="message-header">
                <span class="message-sender">${isMe ? 'You' : senderName}</span>
                <span class="message-time">
                    ${formatTime(msg.time)}
                    ${statusHtml}
                </span>
            </div>
            <div class="message-text">${escapeHtml(msg.text)}</div>
            ${signalHtml}
            ${reactionsHtml}
            <div class="message-actions">
                <button class="msg-action-btn reply-btn" title="Reply">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M10 9V5l-7 7 7 7v-4.1c5 0 8.5 1.6 11 5.1-1-5-4-10-11-11z"/>
                    </svg>
                </button>
                <button class="msg-action-btn react-btn" title="React">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm3.5-9c.83 0 1.5-.67 1.5-1.5S16.33 8 15.5 8 14 8.67 14 9.5s.67 1.5 1.5 1.5zm-7 0c.83 0 1.5-.67 1.5-1.5S9.33 8 8.5 8 7 8.67 7 9.5 7.67 11 8.5 11zm3.5 6.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z"/>
                    </svg>
                </button>
            </div>
        </div>
    `;
}

function setupMessageInteractions() {
    // Setup reply buttons
    messagesList.querySelectorAll('.reply-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const msgEl = btn.closest('.message');
            const packetId = parseInt(msgEl.dataset.packetId);
            const from = parseInt(msgEl.dataset.from);
            const text = msgEl.dataset.text;
            if (packetId) {
                setReplyingTo({ packetId, from, text });
            }
        });
    });

    // Setup reaction buttons
    messagesList.querySelectorAll('.react-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const msgEl = btn.closest('.message');
            const packetId = msgEl.dataset.packetId;
            if (packetId) {
                showEmojiPicker(btn, packetId);
            }
        });
    });
}

// Find a message by packet ID across all message stores
function findMessageByPacketId(packetId) {
    // Search in channel messages
    for (const channelIdx in channelMessages) {
        const msg = channelMessages[channelIdx].find(m => m.packetId === packetId);
        if (msg) return msg;
    }
    // Search in direct messages
    for (const contactKey in messages) {
        const msg = messages[contactKey].find(m => m.packetId === packetId);
        if (msg) return msg;
    }
    return null;
}

// Scroll to a specific message
function scrollToMessage(packetId) {
    const msgEl = document.querySelector(`.message[data-packet-id="${packetId}"]`);
    if (msgEl) {
        msgEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
        msgEl.classList.add('highlight');
        setTimeout(() => msgEl.classList.remove('highlight'), 2000);
    }
}

// Set the message being replied to
function setReplyingTo(msg) {
    replyingTo = msg;
    renderReplyIndicator();
    document.getElementById('messageInput')?.focus();
}

// Cancel reply
function cancelReply() {
    replyingTo = null;
    renderReplyIndicator();
}

// Render the reply indicator above the message input
function renderReplyIndicator() {
    let indicator = document.getElementById('replyIndicator');

    if (!replyingTo) {
        if (indicator) indicator.remove();
        return;
    }

    if (!indicator) {
        indicator = document.createElement('div');
        indicator.id = 'replyIndicator';
        indicator.className = 'reply-indicator';
        const inputArea = document.querySelector('.message-input-area');
        if (inputArea) {
            inputArea.insertBefore(indicator, inputArea.firstChild);
        }
    }

    // Use >>> 0 to convert signed int32 to unsigned for node lookup
    const senderNode = nodes[replyingTo.from >>> 0];
    const senderName = (replyingTo.from >>> 0) === myNode?.num ? 'yourself' :
        (senderNode ? (senderNode.shortName || senderNode.longName) : `!${(replyingTo.from >>> 0).toString(16)}`);

    indicator.innerHTML = `
        <div class="reply-indicator-content">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M10 9V5l-7 7 7 7v-4.1c5 0 8.5 1.6 11 5.1-1-5-4-10-11-11z"/>
            </svg>
            <span>Replying to <strong>${senderName}</strong>: ${escapeHtml(replyingTo.text?.substring(0, 40) || '')}${replyingTo.text?.length > 40 ? '...' : ''}</span>
        </div>
        <button class="reply-cancel-btn" onclick="cancelReply()" title="Cancel reply">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
            </svg>
        </button>
    `;
}

function showEmojiPicker(anchorEl, packetId) {
    // Remove existing picker
    const existingPicker = document.querySelector('.emoji-picker');
    if (existingPicker) existingPicker.remove();

    const picker = document.createElement('div');
    picker.className = 'emoji-picker';
    picker.innerHTML = quickEmojis.map(emoji => `
        <button class="emoji-option" data-emoji="${emoji}">${emoji}</button>
    `).join('');

    // Position near the button
    const rect = anchorEl.getBoundingClientRect();
    picker.style.position = 'fixed';
    picker.style.left = `${rect.left}px`;
    picker.style.top = `${rect.top - 40}px`;

    document.body.appendChild(picker);

    // Handle emoji selection
    picker.querySelectorAll('.emoji-option').forEach(btn => {
        btn.addEventListener('click', async () => {
            const emoji = btn.dataset.emoji;
            try {
                await api('POST', `/messages/${packetId}/reaction`, { emoji });
                // Add reaction locally
                addLocalReaction(packetId, emoji);
            } catch (e) {
                console.error('Failed to add reaction:', e);
            }
            picker.remove();
        });
    });

    // Close picker when clicking outside
    setTimeout(() => {
        document.addEventListener('click', function closeHandler(e) {
            if (!picker.contains(e.target)) {
                picker.remove();
                document.removeEventListener('click', closeHandler);
            }
        });
    }, 0);
}

function addLocalReaction(packetId, emoji) {
    // Find the message and add reaction
    for (const contactKey in messages) {
        const msgList = messages[contactKey];
        const msg = msgList.find(m => m.packetId == packetId);
        if (msg) {
            if (!msg.emojis) msg.emojis = [];
            msg.emojis.push({
                userId: myNode ? `!${(myNode.num >>> 0).toString(16)}` : 'me',
                emoji: emoji,
                timestamp: Date.now() / 1000
            });
            renderMessages();
            return;
        }
    }
}

function setupInfiniteScroll() {
    messagesList.removeEventListener('scroll', handleMessagesScroll);
    messagesList.addEventListener('scroll', handleMessagesScroll);
}

async function handleMessagesScroll() {
    // Load more when scrolled near top
    if (messagesList.scrollTop < 100 && !isLoadingMore && hasMoreMessages && selectedNode) {
        await loadMoreMessages();
    }
}

async function loadMoreMessages() {
    if (isLoadingMore || !selectedNode) return;

    const contactKey = `!${(selectedNode.num >>> 0).toString(16)}`;
    const msgList = messages[contactKey] || [];

    if (msgList.length === 0) return;

    // Get oldest message timestamp
    const oldestTime = Math.min(...msgList.map(m => m.time || m.receivedTime || Infinity));
    if (oldestTime === Infinity) return;

    isLoadingMore = true;

    // Show loading indicator
    const loadIndicator = document.getElementById('loadMoreIndicator');
    if (loadIndicator) {
        loadIndicator.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" class="spin"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"/></svg> Loading...';
    }

    try {
        const data = await api('GET', `/messages/${encodeURIComponent(contactKey)}?limit=20&before=${oldestTime}`);
        const olderMessages = data.messages || [];

        if (olderMessages.length === 0) {
            hasMoreMessages = false;
        } else {
            // Prepend older messages
            messages[contactKey] = [...olderMessages, ...msgList];

            // Re-render preserving scroll position
            const scrollHeight = messagesList.scrollHeight;
            renderMessages();
            messagesList.scrollTop = messagesList.scrollHeight - scrollHeight;
        }
    } catch (e) {
        console.error('Failed to load more messages:', e);
    } finally {
        isLoadingMore = false;
    }
}

// Send Message
sendBtn.addEventListener('click', sendMessage);
messageInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') sendMessage();
});

async function sendMessage() {
    if (!messageInput.value.trim()) return;
    if (!selectedNode && !selectedChannel) return;

    const text = messageInput.value.trim();
    const replyToPacketId = replyingTo?.packetId || null;
    messageInput.value = '';
    sendBtn.disabled = true;

    // Clear reply state
    cancelReply();

    try {
        if (selectedChannel) {
            // Send to channel (broadcast)
            const payload = {
                to: 0xFFFFFFFF, // Broadcast address
                channel: selectedChannel.index,
                text: text
            };
            if (replyToPacketId) {
                payload.replyTo = replyToPacketId;
            }
            await api('POST', '/messages', payload);

            // Add message to local channel messages
            const newMsg = {
                from: 0,
                to: 0xFFFFFFFF,
                channel: selectedChannel.index,
                text: text,
                time: Math.floor(Date.now() / 1000),
                status: 'queued',
                replyTo: replyToPacketId
            };
            if (!channelMessages[selectedChannel.index]) {
                channelMessages[selectedChannel.index] = [];
            }
            channelMessages[selectedChannel.index].push(newMsg);
            renderMessages();
        } else if (selectedNode) {
            // Send to specific node (DM)
            // Use >>> 0 to ensure unsigned value for node number
            const to = selectedNode.num >>> 0;
            const payload = {
                to: to,
                channel: 0,
                text: text
            };
            if (replyToPacketId) {
                payload.replyTo = replyToPacketId;
            }
            await api('POST', '/messages', payload);

            const contactKey = `!${(to >>> 0).toString(16)}`;
            const newMsg = {
                contactKey: contactKey,
                from: 0,
                to: to,
                text: text,
                time: Math.floor(Date.now() / 1000),
                status: 'queued',
                replyTo: replyToPacketId
            };
            addMessage(newMsg);
        }
    } catch (e) {
        console.error('Failed to send message:', e);
        alert('Failed to send message: ' + e.message);
    } finally {
        sendBtn.disabled = false;
    }
}

// Quick Chat Messages
const defaultQuickMessages = [
    "Hello!",
    "On my way",
    "Running late",
    "Arrived safely",
    "Check in",
    "Need assistance",
    "All clear",
    "Copy that",
    "Standing by",
    "10-4"
];

let quickMessages = JSON.parse(localStorage.getItem('quickMessages')) || [...defaultQuickMessages];
let quickChatMode = localStorage.getItem('quickChatMode') || 'instant';

const quickChatBtn = document.getElementById('quickChatBtn');
const quickChatPanel = document.getElementById('quickChatPanel');
const quickChatList = document.getElementById('quickChatList');
const quickChatAddInput = document.getElementById('quickChatAddInput');

function renderQuickMessages() {
    if (!quickChatList) return;

    quickChatList.innerHTML = quickMessages.map((msg, idx) => `
        <div class="quick-chat-item" data-index="${idx}">
            <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/>
            </svg>
            <span class="quick-chat-item-text">${escapeHtml(msg)}</span>
            <svg class="quick-chat-delete" viewBox="0 0 24 24" fill="currentColor" style="cursor:pointer;opacity:0.5" onclick="deleteQuickMessage(event, ${idx})">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
            </svg>
        </div>
    `).join('');

    // Add click handlers for messages
    quickChatList.querySelectorAll('.quick-chat-item').forEach(item => {
        item.addEventListener('click', (e) => {
            if (e.target.closest('.quick-chat-delete')) return;
            const idx = parseInt(item.dataset.index);
            useQuickMessage(quickMessages[idx]);
        });
    });
}

function useQuickMessage(text) {
    if (quickChatMode === 'instant') {
        // Send immediately
        if (!selectedNode && !selectedChannel) {
            showToast('Select a node or channel first', 'error');
            return;
        }
        messageInput.value = text;
        sendMessage();
    } else {
        // Append to input
        messageInput.value = messageInput.value + (messageInput.value ? ' ' : '') + text;
        messageInput.focus();
    }
    quickChatPanel.classList.remove('show');
}

function deleteQuickMessage(event, idx) {
    event.stopPropagation();
    quickMessages.splice(idx, 1);
    localStorage.setItem('quickMessages', JSON.stringify(quickMessages));
    renderQuickMessages();
}

function addQuickMessage(text) {
    if (!text.trim()) return;
    if (quickMessages.includes(text.trim())) {
        showToast('Message already exists', 'error');
        return;
    }
    quickMessages.unshift(text.trim());
    localStorage.setItem('quickMessages', JSON.stringify(quickMessages));
    renderQuickMessages();
    quickChatAddInput.value = '';
}

// Initialize Quick Chat
if (quickChatBtn && quickChatPanel) {
    quickChatBtn.addEventListener('click', () => {
        quickChatPanel.classList.toggle('show');
    });

    // Close panel when clicking outside
    document.addEventListener('click', (e) => {
        if (!e.target.closest('.quick-chat-container')) {
            quickChatPanel.classList.remove('show');
        }
    });

    // Mode toggle
    quickChatPanel.querySelectorAll('.quick-chat-mode-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            quickChatMode = btn.dataset.mode;
            localStorage.setItem('quickChatMode', quickChatMode);
            quickChatPanel.querySelectorAll('.quick-chat-mode-btn').forEach(b => {
                b.classList.toggle('active', b.dataset.mode === quickChatMode);
            });
        });
        // Set initial state
        btn.classList.toggle('active', btn.dataset.mode === quickChatMode);
    });

    // Add new message on Enter
    quickChatAddInput?.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            addQuickMessage(quickChatAddInput.value);
        }
    });

    renderQuickMessages();
}

// Utility Functions
function formatLastHeard(timestamp) {
    if (!timestamp || timestamp === 0) return 'Never';

    const now = Date.now();
    const diff = now - timestamp * 1000;

    if (diff < 0) return 'Just now';
    if (diff < 60000) return 'Just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return `${Math.floor(diff / 86400000)}d ago`;
}

function formatTime(timestamp) {
    if (!timestamp) return '';
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const time = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    // Same calendar day: just time
    if (date.toDateString() === now.toDateString()) {
        return time;
    }

    // Yesterday
    const yesterday = new Date(now);
    yesterday.setDate(yesterday.getDate() - 1);
    if (date.toDateString() === yesterday.toDateString()) {
        return `Yesterday ${time}`;
    }

    const day = date.getDate();
    const month = date.toLocaleString([], { month: 'short' });

    // Different year
    if (date.getFullYear() !== now.getFullYear()) {
        return `${day} ${month} ${date.getFullYear()} ${time}`;
    }

    // Same year, older than yesterday
    return `${day} ${month} ${time}`;
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Copy text to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showToast('Copied', 'Text copied to clipboard', 'success');
    }).catch(err => {
        console.error('Failed to copy:', err);
        showToast('Error', 'Failed to copy to clipboard', 'error');
    });
}

// Signal icon based on quality
function getSignalIcon(quality) {
    const colors = { good: '#30C047', ok: '#FFD54F', weak: '#FF8800', poor: '#F44336' };
    const bars = { good: 4, ok: 3, weak: 2, poor: 1 };
    const color = colors[quality] || colors.poor;
    const count = bars[quality] || 1;

    let svg = `<svg viewBox="0 0 16 12" fill="${color}">`;
    for (let i = 0; i < 4; i++) {
        const h = 3 + i * 3;
        const opacity = i < count ? 1 : 0.2;
        svg += `<rect x="${i * 4}" y="${12 - h}" width="3" height="${h}" rx="0.5" opacity="${opacity}"/>`;
    }
    svg += '</svg>';
    return svg;
}

// Bearing arrow based on degrees
function getBearingArrow(bearing) {
    if (bearing === undefined || bearing === null) return '';
    // Unicode arrows for 8 cardinal directions
    const arrows = ['↑', '↗', '→', '↘', '↓', '↙', '←', '↖'];
    const index = Math.round(bearing / 45) % 8;
    return arrows[index];
}

// Calculate bearing between two points
function calculateBearing(lat1, lon1, lat2, lon2) {
    const toRad = deg => deg * Math.PI / 180;
    const toDeg = rad => rad * 180 / Math.PI;

    const dLon = toRad(lon2 - lon1);
    const y = Math.sin(dLon) * Math.cos(toRad(lat2));
    const x = Math.cos(toRad(lat1)) * Math.sin(toRad(lat2)) -
              Math.sin(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.cos(dLon);
    let bearing = toDeg(Math.atan2(y, x));
    return (bearing + 360) % 360;
}

// Bearing to cardinal direction
function bearingToCardinal(bearing) {
    const directions = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
    const index = Math.round(bearing / 45) % 8;
    return directions[index];
}

// Format hardware model (shortened)
function formatHwShort(model) {
    if (!model) return '';
    // Common hardware abbreviations
    const abbrevs = {
        'HELTEC_V3': 'Heltec V3',
        'HELTEC_V2_1': 'Heltec V2.1',
        'HELTEC_V2': 'Heltec V2',
        'TBEAM': 'T-Beam',
        'TBEAM_V0P7': 'T-Beam 0.7',
        'TLORA_V1': 'T-LoRa V1',
        'TLORA_V2': 'T-LoRa V2',
        'TLORA_V2_1_1P6': 'T-LoRa 2.1',
        'TLORA_V2_1_1P8': 'T-LoRa 2.1',
        'TLORA_T3_S3': 'T3-S3',
        'RAK4631': 'RAK4631',
        'RAK11200': 'RAK11200',
        'RAK11310': 'RAK11310',
        'STATION_G1': 'Station G1',
        'STATION_G2': 'Station G2',
        'NANO_G1': 'Nano G1',
        'NANO_G2': 'Nano G2',
        'PORTDUINO': 'Linux',
        'LINUX_NATIVE': 'Linux',
        'PRIVATE_HW': 'Custom',
        'DIY_V1': 'DIY',
        'NRF52_UNKNOWN': 'nRF52',
        'PICOMPUTER_S3': 'PiComputer',
        'TRACKER_T1000_E': 'T1000-E',
        'WIO_WM1110': 'WIO WM1110',
        'UNSET': 'Unknown'
    };

    return abbrevs[model] || model.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
}

// Channels
async function loadChannels() {
    try {
        const data = await api('GET', '/channels');
        channels = data.channels || [];
        renderChannels();
    } catch (e) {
        console.error('Failed to load channels:', e);
    }
}

function updateChannel(channel) {
    const index = channels.findIndex(c => c.index === channel.index);
    if (index >= 0) {
        channels[index] = channel;
    } else {
        channels.push(channel);
    }
    renderChannels();
}

function renderChannels() {
    const channelsList = document.getElementById('channelsList');
    if (!channelsList) return;

    // Create array of all 8 channels (0-7), merging with received channel data
    const allChannels = [];
    for (let i = 0; i < 8; i++) {
        const existingChannel = channels.find(c => c.index === i);
        if (existingChannel) {
            allChannels.push(existingChannel);
        } else {
            allChannels.push({
                index: i,
                name: '',
                role: 'DISABLED',
                psk: ''
            });
        }
    }

    channelsList.innerHTML = allChannels.map(ch => {
        const roleClass = ch.role === 'PRIMARY' ? 'primary' : ch.role === 'SECONDARY' ? 'secondary' : 'disabled';
        const isSelected = selectedChannel !== null && selectedChannel.index === ch.index;
        const isActive = ch.role !== 'DISABLED';
        const channelName = ch.name || (ch.role === 'PRIMARY' ? 'Primary' : `Channel ${ch.index}`);
        const unreadCount = channelUnreadCounts[ch.index] || 0;

        return `
        <div class="channel-card ${isSelected ? 'selected' : ''} ${isActive ? 'active' : ''} ${unreadCount > 0 ? 'has-unread' : ''}"
             data-index="${ch.index}"
             data-role="${ch.role}"
             data-name="${channelName}"
             data-psk="${ch.psk || ''}">
            <div class="channel-index ${roleClass}">${ch.index}</div>
            <div class="channel-info">
                <div class="channel-name">${channelName}</div>
                <div class="channel-role">${ch.role}</div>
            </div>
            ${unreadCount > 0 ? `<span class="unread-badge">${unreadCount > 99 ? '99+' : unreadCount}</span>` : ''}
            <div class="channel-actions">
                ${isActive ? '<button class="channel-btn chat-btn" title="Chat"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M20 2H4c-1.1 0-1.99.9-1.99 2L2 22l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z"/></svg></button>' : ''}
                <button class="channel-btn edit-btn" title="Edit channel"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"/></svg></button>
            </div>
        </div>
    `}).join('');

    // Add click handlers for chat buttons
    channelsList.querySelectorAll('.chat-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const card = btn.closest('.channel-card');
            const index = parseInt(card.dataset.index);
            const role = card.dataset.role;
            const name = card.dataset.name;
            selectChannel({ index, role, name });
        });
    });

    // Add click handlers for edit buttons
    channelsList.querySelectorAll('.edit-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const card = btn.closest('.channel-card');
            const index = parseInt(card.dataset.index);
            const channel = channels.find(c => c.index === index) || {
                index,
                name: '',
                role: 'DISABLED',
                psk: ''
            };
            openChannelEditModal(channel);
        });
    });

    // Click on card also opens chat for active channels
    channelsList.querySelectorAll('.channel-card.active').forEach(card => {
        card.addEventListener('click', (e) => {
            if (e.target.closest('.channel-btn')) return; // Ignore button clicks
            const index = parseInt(card.dataset.index);
            const role = card.dataset.role;
            const name = card.dataset.name;
            selectChannel({ index, role, name });
        });
    });
}

function selectChannel(channel) {
    console.log('selectChannel called:', channel);
    selectedChannel = channel;
    selectedNode = null; // Deselect node when selecting channel

    // Clear unread count for this channel
    if (channelUnreadCounts[channel.index]) {
        channelUnreadCounts[channel.index] = 0;
        renderChannels();
    }

    // Update channel list UI
    document.querySelectorAll('.channel-card').forEach(card => {
        card.classList.toggle('selected', parseInt(card.dataset.index) === channel.index);
    });

    // Update node list UI - deselect any selected node
    document.querySelectorAll('.node-item').forEach(item => {
        item.classList.remove('selected');
    });

    // Hide node details
    nodeDetailsSection.style.display = 'none';

    // Update chat header
    const chatHeader = document.getElementById('chatHeader');
    const chatHeaderAvatar = document.getElementById('chatHeaderAvatar');
    const chatHeaderName = document.getElementById('chatHeaderName');
    const chatHeaderType = document.getElementById('chatHeaderType');

    if (chatHeader && chatHeaderAvatar && chatHeaderName && chatHeaderType) {
        chatHeader.style.display = 'flex';
        chatHeaderAvatar.textContent = channel.index;
        chatHeaderAvatar.className = 'chat-header-avatar channel';
        chatHeaderName.textContent = channel.name || `Channel ${channel.index}`;
        chatHeaderType.textContent = `Channel · ${channel.role}`;
    }

    // Load channel messages
    loadChannelMessages(channel.index);

    // Enable message input
    const msgInput = document.getElementById('messageInput');
    const sendButton = document.getElementById('sendBtn');

    if (msgInput) {
        msgInput.disabled = false;
        msgInput.placeholder = `Message to ${channel.name || 'channel'}...`;
        msgInput.focus();
    }
    if (sendButton) {
        sendButton.disabled = false;
    }
    const quickBtn = document.getElementById('quickChatBtn');
    if (quickBtn) {
        quickBtn.disabled = false;
    }

    console.log('Input enabled:', msgInput?.disabled === false, 'Button enabled:', sendButton?.disabled === false);

    // Switch to messages tab
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
    document.querySelector('.tab[data-tab="messages"]').classList.add('active');
    document.getElementById('messages').classList.add('active');
}

async function loadChannelMessages(channelIndex) {
    try {
        const data = await api('GET', `/channel-messages/${channelIndex}?limit=50`);
        channelMessages[channelIndex] = data.messages || [];
        renderMessages();
    } catch (e) {
        console.error('Failed to load channel messages:', e);
        channelMessages[channelIndex] = [];
        renderMessages();
    }
}

// Channel Edit Modal
function openChannelEditModal(channel) {
    // Remove existing modal
    const existing = document.querySelector('.channel-edit-modal');
    if (existing) existing.remove();

    const roleOptions = ['DISABLED', 'PRIMARY', 'SECONDARY'].map(role =>
        `<option value="${role}" ${channel.role === role ? 'selected' : ''}>${role}</option>`
    ).join('');

    const html = `
        <div class="channel-edit-modal">
            <div class="channel-edit-content">
                <div class="channel-edit-header">
                    <h3>Edit Channel ${channel.index}</h3>
                    <button class="modal-close" onclick="closeChannelEditModal()">&times;</button>
                </div>
                <div class="channel-edit-body">
                    <div class="form-group">
                        <label for="channelName">Name</label>
                        <input type="text" id="channelName" value="${channel.name || ''}" placeholder="Channel name" maxlength="11">
                    </div>
                    <div class="form-group">
                        <label for="channelRole">Role</label>
                        <select id="channelRole">${roleOptions}</select>
                    </div>
                    <div class="form-group">
                        <label for="channelPsk">Pre-Shared Key</label>
                        <div class="psk-input-group">
                            <input type="password" id="channelPsk" value="" placeholder="${channel.psk ? '(unchanged)' : 'Leave empty for default'}">
                            <button type="button" class="psk-toggle" onclick="togglePskVisibility()">
                                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/></svg>
                            </button>
                        </div>
                        <small class="form-hint">Enter new PSK or leave empty to keep current. Use "random" for a random key, "none" for no encryption.</small>
                    </div>
                </div>
                <div class="channel-edit-footer">
                    <button class="btn btn-secondary" onclick="closeChannelEditModal()">Cancel</button>
                    <button class="btn btn-primary" onclick="saveChannel(${channel.index})">Save Channel</button>
                </div>
            </div>
        </div>
    `;

    document.body.insertAdjacentHTML('beforeend', html);

    // Animate in
    setTimeout(() => {
        document.querySelector('.channel-edit-modal').classList.add('show');
    }, 10);

    // Focus name input
    document.getElementById('channelName').focus();
}

function closeChannelEditModal() {
    const modal = document.querySelector('.channel-edit-modal');
    if (modal) {
        modal.classList.remove('show');
        setTimeout(() => modal.remove(), 300);
    }
}

function togglePskVisibility() {
    const input = document.getElementById('channelPsk');
    input.type = input.type === 'password' ? 'text' : 'password';
}

async function saveChannel(index) {
    const name = document.getElementById('channelName').value.trim();
    const role = document.getElementById('channelRole').value;
    const psk = document.getElementById('channelPsk').value;

    const channelData = {
        index,
        name,
        role,
    };

    // Only include PSK if it was changed
    if (psk) {
        channelData.psk = psk;
    }

    try {
        await api('PUT', `/channels/${index}`, channelData);
        showToast('Channel saved', 'Configuration sent to device');
        closeChannelEditModal();
        // Refresh channels after a short delay (device needs time to process)
        setTimeout(loadChannels, 1000);
    } catch (e) {
        console.error('Failed to save channel:', e);
        showToast('Error', e.message, 'error');
    }
}

// Config
let moduleStates = {};

async function loadConfig() {
    try {
        const data = await api('GET', '/config');
        deviceConfig = data.config || {};
        renderConfig();
        // Also load module states to update module cards
        loadModuleStates();
    } catch (e) {
        console.error('Failed to load config:', e);
    }
}

// Load module states and update cards
async function loadModuleStates() {
    try {
        const data = await api('GET', '/modules');
        moduleStates = data;
        updateModuleCards();
    } catch (e) {
        console.error('Failed to load module states:', e);
    }
}

// Update module card UI with actual enabled states
function updateModuleCards() {
    const moduleMapping = {
        'mqtt': moduleStates.mqtt,
        'serial': moduleStates.serial,
        'storeforward': moduleStates.storeForward,
        'telemetry': moduleStates.telemetry,
        'rangetest': moduleStates.rangeTest,
        'cannedmsg': moduleStates.cannedMessage,
        'extnotify': moduleStates.externalNotification,
        'neighborinfo': moduleStates.neighborInfo
    };

    for (const [id, config] of Object.entries(moduleMapping)) {
        const card = document.querySelector(`.module-card[onclick*="'${id}'"]`);
        if (card) {
            let isEnabled = false;
            if (id === 'telemetry') {
                // Telemetry doesn't have a global enabled flag - check if any subsystem is enabled
                isEnabled = config?.deviceUpdateInterval > 0 ||
                           config?.environmentMeasurementEnabled ||
                           config?.airQualityEnabled ||
                           config?.powerMeasurementEnabled;
            } else {
                isEnabled = config?.enabled || false;
            }
            card.classList.toggle('enabled', isEnabled);
            const statusEl = card.querySelector('.module-status');
            if (statusEl) {
                statusEl.textContent = isEnabled ? 'Enabled' : 'Disabled';
            }
        }
    }
}

function renderConfig() {
    const configContent = document.getElementById('configContent');
    if (!configContent) return;

    if (!deviceConfig || Object.keys(deviceConfig).length === 0 || !deviceConfig.firmwareVersion) {
        configContent.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/>
                </svg>
                <p>Connect to a device to view configuration</p>
            </div>
        `;
        return;
    }

    // Build channels section
    let channelsHtml = '';
    if (channels.length > 0) {
        const activeChannels = channels.filter(ch => ch.role !== 'DISABLED');
        channelsHtml = `
        <div class="config-section collapsible" data-section="channels">
            <div class="config-section-header" onclick="toggleConfigSection('channels')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z"/></svg>
                    Channels (${activeChannels.length} active)
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                ${activeChannels.map(ch => `
                    <div class="config-item channel-item">
                        <span class="config-item-label">
                            <span class="channel-badge ${ch.role === 'PRIMARY' ? 'primary' : 'secondary'}">${ch.index}</span>
                            ${ch.name || 'Unnamed'}
                        </span>
                        <span class="config-item-value">${ch.role}</span>
                    </div>
                `).join('')}
            </div>
        </div>
        `;
    }

    // Role options
    const roleOptions = ['CLIENT', 'CLIENT_MUTE', 'ROUTER', 'ROUTER_CLIENT', 'REPEATER', 'TRACKER', 'SENSOR', 'TAK', 'CLIENT_HIDDEN', 'LOST_AND_FOUND', 'TAK_TRACKER'];
    const regionOptions = ['UNSET', 'US', 'EU_433', 'EU_868', 'CN', 'JP', 'ANZ', 'KR', 'TW', 'RU', 'IN', 'NZ_865', 'TH', 'LORA_24', 'UA_433', 'UA_868', 'MY_433', 'MY_919', 'SG_923'];
    const modemOptions = ['LONG_FAST', 'LONG_SLOW', 'LONG_MODERATE', 'VERY_LONG_SLOW', 'MEDIUM_SLOW', 'MEDIUM_FAST', 'SHORT_SLOW', 'SHORT_FAST', 'SHORT_TURBO'];

    configContent.innerHTML = `
        <!-- Device Info (Read-only) -->
        <div class="config-section collapsible" data-section="info">
            <div class="config-section-header" onclick="toggleConfigSection('info')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>
                    Device Info
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                ${configItem('Firmware', deviceConfig.firmwareVersion)}
                ${configItem('Hardware', formatHardwareModel(deviceConfig.hardwareModel))}
                ${configItem('Has WiFi', deviceConfig.hasWifi, 'bool')}
                ${configItem('Has Bluetooth', deviceConfig.hasBluetooth, 'bool')}
                ${configItem('Has Ethernet', deviceConfig.hasEthernet, 'bool')}
            </div>
        </div>

        <!-- User Config (Editable) -->
        <div class="config-section collapsible" data-section="user">
            <div class="config-section-header" onclick="toggleConfigSection('user')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>
                    User / Owner
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">Long Name</span>
                    <input type="text" class="config-item-input" data-config="user.longName"
                           value="${myNode?.longName || ''}" maxlength="39"
                           onchange="markConfigChanged('user')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Short Name</span>
                    <input type="text" class="config-item-input" data-config="user.shortName"
                           value="${myNode?.shortName || ''}" maxlength="4"
                           onchange="markConfigChanged('user')">
                </div>
                ${configItem('Node ID', myNode ? '!' + (myNode.num >>> 0).toString(16) : 'Unknown')}
                ${configItem('Licensed', myNode?.isLicensed, 'bool')}
                ${myNode?.publicKey ? `
                <div class="config-item">
                    <span class="config-item-label">Public Key</span>
                    <div class="config-item-copyable">
                        <input type="text" class="config-item-input" value="${myNode.publicKey}" readonly
                               onclick="this.select()" title="Click to select, then copy">
                        <button class="btn-icon" onclick="copyToClipboard('${myNode.publicKey}')" title="Copy to clipboard">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                                <path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/>
                            </svg>
                        </button>
                    </div>
                </div>
                ` : ''}
            </div>
            <div class="config-actions" id="userActions" style="display: none;">
                <button class="btn btn-primary btn-sm" onclick="saveUserConfig()">Save User Config</button>
            </div>
        </div>

        <!-- Device Config (Editable) -->
        <div class="config-section collapsible" data-section="device">
            <div class="config-section-header" onclick="toggleConfigSection('device')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M17 1.01L7 1c-1.1 0-2 .9-2 2v18c0 1.1.9 2 2 2h10c1.1 0 2-.9 2-2V3c0-1.1-.9-1.99-2-1.99zM17 19H7V5h10v14z"/></svg>
                    Device Config
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">Role</span>
                    <select class="config-item-select" data-config="device.role" onchange="markConfigChanged('device')">
                        ${roleOptions.map(r => `<option value="${r}" ${deviceConfig.role === r ? 'selected' : ''}>${formatRole(r)}</option>`).join('')}
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Node Broadcast (sec)</span>
                    <input type="number" class="config-item-input" data-config="device.nodeInfoBroadcastSecs"
                           value="${deviceConfig.nodeInfoBroadcastSecs || 900}" min="0" max="86400"
                           onchange="markConfigChanged('device')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Timezone</span>
                    <input type="text" class="config-item-input" data-config="device.tzdef"
                           value="${deviceConfig.tzdef || ''}" placeholder="e.g. EST5EDT,M3.2.0,M11.1.0"
                           onchange="markConfigChanged('device')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Serial Enabled</span>
                    ${configToggle('device.serialEnabled', deviceConfig.serialEnabled, 'device')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Debug Log</span>
                    ${configToggle('device.debugLogEnabled', deviceConfig.debugLogEnabled, 'device')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Double Tap as Button</span>
                    ${configToggle('device.doubleTapAsButtonPress', deviceConfig.doubleTapAsButtonPress, 'device')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Disable LED Heartbeat</span>
                    ${configToggle('device.ledHeartbeatDisabled', deviceConfig.ledHeartbeatDisabled, 'device')}
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="device" onclick="saveDeviceConfig()" disabled>Save Device Config</button>
            </div>
        </div>

        <!-- LoRa Config (Editable) -->
        <div class="config-section collapsible" data-section="lora">
            <div class="config-section-header" onclick="toggleConfigSection('lora')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 6c2.62 0 4.88 1.86 5.39 4.43l.3 1.5 1.53.11c1.56.1 2.78 1.41 2.78 2.96 0 1.65-1.35 3-3 3H6c-2.21 0-4-1.79-4-4 0-2.05 1.53-3.76 3.56-3.97l1.07-.11.5-.95C8.08 7.14 9.94 6 12 6m0-2C9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96C18.67 6.59 15.64 4 12 4z"/></svg>
                    LoRa Radio
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">Region</span>
                    <select class="config-item-select" data-config="lora.region" onchange="markConfigChanged('lora')">
                        ${regionOptions.map(r => `<option value="${r}" ${deviceConfig.region === r ? 'selected' : ''}>${formatRegion(r)}</option>`).join('')}
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Modem Preset</span>
                    <select class="config-item-select" data-config="lora.modemPreset" onchange="markConfigChanged('lora')">
                        ${modemOptions.map(m => `<option value="${m}" ${deviceConfig.modemPreset === m ? 'selected' : ''}>${formatModemPreset(m)}</option>`).join('')}
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Hop Limit</span>
                    <input type="number" class="config-item-input" data-config="lora.hopLimit"
                           value="${deviceConfig.hopLimit || 3}" min="1" max="7"
                           onchange="markConfigChanged('lora')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Frequency Slot</span>
                    <input type="number" class="config-item-input" data-config="lora.channelNum"
                           value="${deviceConfig.channelNum || 0}" min="0" max="100"
                           onchange="markConfigChanged('lora')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Frequency Offset (Hz)</span>
                    <input type="number" class="config-item-input" data-config="lora.frequencyOffset"
                           value="${deviceConfig.frequencyOffset || 0}" step="0.1"
                           onchange="markConfigChanged('lora')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">TX Enabled</span>
                    ${configToggle('lora.txEnabled', deviceConfig.txEnabled, 'lora')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">TX Power (dBm)</span>
                    <input type="number" class="config-item-input" data-config="lora.txPower"
                           value="${deviceConfig.txPower || 0}" min="0" max="30"
                           onchange="markConfigChanged('lora')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Override Duty Cycle</span>
                    ${configToggle('lora.overrideDutyCycle', deviceConfig.overrideDutyCycle, 'lora')}
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem; display: block; margin-top: 0.25rem;">Only for testing - may violate regulations</small>
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="lora" onclick="saveLoRaConfig()" disabled>Save LoRa Config</button>
            </div>
        </div>

        ${channelsHtml}

        <!-- Position Config (Editable) -->
        <div class="config-section collapsible" data-section="position">
            <div class="config-section-header" onclick="toggleConfigSection('position')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/></svg>
                    Position
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">GPS Enabled</span>
                    ${configToggle('position.gpsEnabled', deviceConfig.gpsEnabled, 'position')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Fixed Position</span>
                    ${configToggle('position.fixedPosition', deviceConfig.fixedPosition, 'position')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Broadcast Interval (sec)</span>
                    <input type="number" class="config-item-input" data-config="position.positionBroadcastSecs"
                           value="${deviceConfig.positionBroadcastSecs || 900}" min="0" max="86400"
                           onchange="markConfigChanged('position')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Smart Broadcast</span>
                    ${configToggle('position.positionBroadcastSmartEnabled', deviceConfig.positionBroadcastSmartEnabled, 'position')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">GPS Update Interval (sec)</span>
                    <input type="number" class="config-item-input" data-config="position.gpsUpdateInterval"
                           value="${deviceConfig.gpsUpdateInterval || 120}" min="0" max="86400"
                           onchange="markConfigChanged('position')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Smart Min Distance (m)</span>
                    <input type="number" class="config-item-input" data-config="position.broadcastSmartMinimumDistance"
                           value="${deviceConfig.broadcastSmartMinimumDistance || 100}" min="0" max="10000"
                           onchange="markConfigChanged('position')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Smart Min Interval (sec)</span>
                    <input type="number" class="config-item-input" data-config="position.broadcastSmartMinimumIntervalSecs"
                           value="${deviceConfig.broadcastSmartMinimumIntervalSecs || 30}" min="0" max="86400"
                           onchange="markConfigChanged('position')">
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="position" onclick="savePositionConfig()" disabled>Save Position Config</button>
            </div>
        </div>

        <!-- Power Config -->
        <div class="config-section collapsible" data-section="power">
            <div class="config-section-header" onclick="toggleConfigSection('power')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M15.67 4H14V2h-4v2H8.33C7.6 4 7 4.6 7 5.33v15.33C7 21.4 7.6 22 8.33 22h7.33c.74 0 1.34-.6 1.34-1.33V5.33C17 4.6 16.4 4 15.67 4zM13 18h-2v-2h2v2zm0-4h-2V9h2v5z"/></svg>
                    Power
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                ${configItem('Shutdown Support', deviceConfig.canShutdown, 'bool')}
                <div class="config-item editable">
                    <span class="config-item-label">Power Saving Mode</span>
                    ${configToggle('power.isPowerSaving', deviceConfig.isPowerSaving, 'power')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Shutdown on Battery (sec)</span>
                    <input type="number" class="config-item-input" data-config="power.onBatteryShutdownAfterSecs"
                           value="${deviceConfig.onBatteryShutdownAfterSecs || 0}" min="0" max="86400"
                           onchange="markConfigChanged('power')">
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem;">0 = disabled</small>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Wait Bluetooth (sec)</span>
                    <input type="number" class="config-item-input" data-config="power.waitBluetoothSecs"
                           value="${deviceConfig.waitBluetoothSecs || 60}" min="0" max="3600"
                           onchange="markConfigChanged('power')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Light Sleep (sec)</span>
                    <input type="number" class="config-item-input" data-config="power.lsSecs"
                           value="${deviceConfig.lsSecs || 300}" min="0" max="86400"
                           onchange="markConfigChanged('power')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Min Wake Time (sec)</span>
                    <input type="number" class="config-item-input" data-config="power.minWakeSecs"
                           value="${deviceConfig.minWakeSecs || 10}" min="0" max="3600"
                           onchange="markConfigChanged('power')">
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="power" onclick="savePowerConfig()" disabled>Save Power Config</button>
            </div>
        </div>

        <!-- Display Config -->
        <div class="config-section collapsible" data-section="display">
            <div class="config-section-header" onclick="toggleConfigSection('display')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M21 3H3c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 16H3V5h18v14z"/></svg>
                    Display
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">Screen On Time (sec)</span>
                    <input type="number" class="config-item-input" data-config="display.screenOnSecs"
                           value="${deviceConfig.screenOnSecs || 0}" min="0" max="86400"
                           onchange="markConfigChanged('display')">
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem;">0 = always on</small>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Auto Carousel (sec)</span>
                    <input type="number" class="config-item-input" data-config="display.autoScreenCarouselSecs"
                           value="${deviceConfig.autoScreenCarouselSecs || 0}" min="0" max="3600"
                           onchange="markConfigChanged('display')">
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem;">0 = disabled</small>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">GPS Format</span>
                    <select class="config-item-select" data-config="display.gpsFormat" onchange="markConfigChanged('display')">
                        <option value="0" ${deviceConfig.gpsFormat === 0 ? 'selected' : ''}>Decimal Degrees</option>
                        <option value="1" ${deviceConfig.gpsFormat === 1 ? 'selected' : ''}>DMS</option>
                        <option value="2" ${deviceConfig.gpsFormat === 2 ? 'selected' : ''}>UTM</option>
                        <option value="3" ${deviceConfig.gpsFormat === 3 ? 'selected' : ''}>MGRS</option>
                        <option value="4" ${deviceConfig.gpsFormat === 4 ? 'selected' : ''}>OLC</option>
                        <option value="5" ${deviceConfig.gpsFormat === 5 ? 'selected' : ''}>OSGR</option>
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Units</span>
                    <select class="config-item-select" data-config="display.units" onchange="markConfigChanged('display')">
                        <option value="0" ${deviceConfig.units === 0 ? 'selected' : ''}>Metric</option>
                        <option value="1" ${deviceConfig.units === 1 ? 'selected' : ''}>Imperial</option>
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Flip Screen</span>
                    ${configToggle('display.flipScreen', deviceConfig.flipScreen, 'display')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Compass North Top</span>
                    ${configToggle('display.compassNorthTop', deviceConfig.compassNorthTop, 'display')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Wake on Tap/Motion</span>
                    ${configToggle('display.wakeOnTapOrMotion', deviceConfig.wakeOnTapOrMotion, 'display')}
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="display" onclick="saveDisplayConfig()" disabled>Save Display Config</button>
            </div>
        </div>

        <!-- Bluetooth Config -->
        <div class="config-section collapsible" data-section="bluetooth">
            <div class="config-section-header" onclick="toggleConfigSection('bluetooth')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29 4.3-4.29zM13 5.83l1.88 1.88L13 9.59V5.83zm1.88 10.46L13 18.17v-3.76l1.88 1.88z"/></svg>
                    Bluetooth
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">Bluetooth Enabled</span>
                    ${configToggle('bluetooth.enabled', deviceConfig.bluetoothEnabled, 'bluetooth')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Pairing Mode</span>
                    <select class="config-item-select" data-config="bluetooth.mode" onchange="markConfigChanged('bluetooth')">
                        <option value="RANDOM_PIN" ${deviceConfig.bluetoothMode === 'RANDOM_PIN' ? 'selected' : ''}>Random PIN</option>
                        <option value="FIXED_PIN" ${deviceConfig.bluetoothMode === 'FIXED_PIN' ? 'selected' : ''}>Fixed PIN</option>
                        <option value="NO_PIN" ${deviceConfig.bluetoothMode === 'NO_PIN' ? 'selected' : ''}>No PIN</option>
                    </select>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">Fixed PIN</span>
                    <input type="number" class="config-item-input" data-config="bluetooth.fixedPin"
                           value="${deviceConfig.bluetoothFixedPin || 123456}" min="0" max="999999"
                           onchange="markConfigChanged('bluetooth')">
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem;">Only used with Fixed PIN mode</small>
                </div>
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="bluetooth" onclick="saveBluetoothConfig()" disabled>Save Bluetooth Config</button>
            </div>
        </div>

        <!-- Network Config (WiFi) -->
        <div class="config-section collapsible" data-section="network" ${deviceConfig.hasWifi ? '' : 'style="display: none;"'}>
            <div class="config-section-header" onclick="toggleConfigSection('network')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M1 9l2 2c4.97-4.97 13.03-4.97 18 0l2-2C16.93 2.93 7.08 2.93 1 9zm8 8l3 3 3-3c-1.65-1.66-4.34-1.66-6 0zm-4-4l2 2c2.76-2.76 7.24-2.76 10 0l2-2C15.14 9.14 8.87 9.14 5 13z"/></svg>
                    Network (WiFi)
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items">
                <div class="config-item editable">
                    <span class="config-item-label">WiFi Enabled</span>
                    ${configToggle('network.wifiEnabled', deviceConfig.wifiEnabled, 'network')}
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">WiFi SSID</span>
                    <input type="text" class="config-item-input" data-config="network.wifiSsid"
                           value="${deviceConfig.wifiSsid || ''}" placeholder="Network name"
                           onchange="markConfigChanged('network')">
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">WiFi Password</span>
                    <input type="password" class="config-item-input" data-config="network.wifiPsk"
                           value="" placeholder="Enter new password"
                           onchange="markConfigChanged('network')">
                    <small style="color: var(--on-surface-variant); font-size: 0.7rem;">Leave blank to keep current</small>
                </div>
                <div class="config-item editable">
                    <span class="config-item-label">NTP Server</span>
                    <input type="text" class="config-item-input" data-config="network.ntpServer"
                           value="${deviceConfig.ntpServer || ''}" placeholder="pool.ntp.org"
                           onchange="markConfigChanged('network')">
                </div>
                ${deviceConfig.hasEthernet ? `
                <div class="config-item editable">
                    <span class="config-item-label">Ethernet Enabled</span>
                    ${configToggle('network.ethEnabled', deviceConfig.ethEnabled, 'network')}
                </div>
                ` : ''}
            </div>
            <div class="config-section-footer">
                <button class="btn btn-save" data-section="network" onclick="saveNetworkConfig()" disabled>Save Network Config</button>
            </div>
        </div>

        <!-- Modules Section -->
        <div class="config-section collapsible" data-section="modules">
            <div class="config-section-header" onclick="toggleConfigSection('modules')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z"/></svg>
                    Modules
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="module-grid">
                ${renderModuleCard('MQTT', 'mqtt', false)}
                ${renderModuleCard('Serial', 'serial', false)}
                ${renderModuleCard('Store & Forward', 'storeforward', false)}
                ${renderModuleCard('Telemetry', 'telemetry', true)}
                ${renderModuleCard('Range Test', 'rangetest', false)}
                ${renderModuleCard('Canned Messages', 'cannedmsg', false)}
                ${renderModuleCard('External Notify', 'extnotify', false)}
                ${renderModuleCard('Neighbor Info', 'neighborinfo', false)}
            </div>
        </div>

        <!-- Device Actions -->
        <div class="config-section collapsible" data-section="actions">
            <div class="config-section-header" onclick="toggleConfigSection('actions')">
                <span class="section-title">
                    <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>
                    Device Actions
                </span>
                <svg class="collapse-icon" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6 1.41-1.41z"/></svg>
            </div>
            <div class="config-items" style="padding: 1rem; display: flex; gap: 0.75rem; flex-wrap: wrap;">
                <button class="btn btn-secondary" onclick="rebootDevice()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg>
                    Reboot
                </button>
                <button class="btn btn-secondary" onclick="shutdownDevice()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M13 3h-2v10h2V3zm4.83 2.17l-1.42 1.42C17.99 7.86 19 9.81 19 12c0 3.87-3.13 7-7 7s-7-3.13-7-7c0-2.19 1.01-4.14 2.58-5.42L6.17 5.17C4.23 6.82 3 9.26 3 12c0 4.97 4.03 9 9 9s9-4.03 9-9c0-2.74-1.23-5.18-3.17-6.83z"/></svg>
                    Shutdown
                </button>
                <button class="btn btn-secondary" style="color: var(--error);" onclick="factoryReset()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>
                    Factory Reset
                </button>
            </div>
        </div>
    `;

    // Setup section toggle event handlers
    setupConfigSectionHandlers();
}

// Format helpers for config display
function formatHardwareModel(model) {
    if (!model) return '-';
    // Convert HELTEC_V3 to Heltec V3, TBEAM to T-Beam, etc.
    return model.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
}

function formatRole(role) {
    if (!role) return '-';
    const roles = {
        'CLIENT': 'Client',
        'CLIENT_MUTE': 'Client (Muted)',
        'ROUTER': 'Router',
        'ROUTER_CLIENT': 'Router + Client',
        'REPEATER': 'Repeater',
        'TRACKER': 'Tracker',
        'SENSOR': 'Sensor',
        'TAK': 'TAK',
        'CLIENT_HIDDEN': 'Client (Hidden)',
        'LOST_AND_FOUND': 'Lost & Found',
        'TAK_TRACKER': 'TAK Tracker'
    };
    return roles[role] || role;
}

function formatRegion(region) {
    if (!region) return '-';
    const regions = {
        'UNSET': 'Not Set',
        'US': 'United States',
        'EU_433': 'EU 433MHz',
        'EU_868': 'EU 868MHz',
        'CN': 'China',
        'JP': 'Japan',
        'ANZ': 'Australia/NZ',
        'KR': 'Korea',
        'TW': 'Taiwan',
        'RU': 'Russia',
        'IN': 'India',
        'NZ_865': 'New Zealand 865MHz',
        'TH': 'Thailand',
        'LORA_24': '2.4GHz',
        'UA_433': 'Ukraine 433MHz',
        'UA_868': 'Ukraine 868MHz',
        'MY_433': 'Malaysia 433MHz',
        'MY_919': 'Malaysia 919MHz',
        'SG_923': 'Singapore 923MHz'
    };
    return regions[region] || region;
}

function formatModemPreset(preset) {
    if (!preset) return '-';
    const presets = {
        'LONG_FAST': 'Long Range / Fast',
        'LONG_SLOW': 'Long Range / Slow',
        'LONG_MODERATE': 'Long Range / Moderate',
        'VERY_LONG_SLOW': 'Very Long Range / Slow',
        'MEDIUM_SLOW': 'Medium Range / Slow',
        'MEDIUM_FAST': 'Medium Range / Fast',
        'SHORT_SLOW': 'Short Range / Slow',
        'SHORT_FAST': 'Short Range / Fast',
        'SHORT_TURBO': 'Short Range / Turbo'
    };
    return presets[preset] || preset;
}

function formatInterval(seconds) {
    if (!seconds || seconds <= 0) return '-';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
}

function configItem(label, value, type = 'text') {
    let displayValue = value;
    let valueClass = '';

    if (type === 'bool') {
        displayValue = value ? 'Yes' : 'No';
        valueClass = value ? 'success' : '';
    } else if (value === undefined || value === null || value === '') {
        displayValue = '-';
    }

    return `
        <div class="config-item">
            <span class="config-item-label">${label}</span>
            <span class="config-item-value ${valueClass}">${displayValue}</span>
        </div>
    `;
}

function configToggle(name, checked, section) {
    return `
        <label class="toggle-switch">
            <input type="checkbox" data-config="${name}" ${checked ? 'checked' : ''} onchange="markConfigChanged('${section}')">
            <span class="toggle-slider"></span>
        </label>
    `;
}

function renderModuleCard(name, id, enabled) {
    return `
        <div class="module-card ${enabled ? 'enabled' : ''}" onclick="openModuleConfig('${id}')">
            <div>
                <div class="module-name">${name}</div>
                <div class="module-status">${enabled ? 'Enabled' : 'Disabled'}</div>
            </div>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6-1.41-1.41z"/></svg>
        </div>
    `;
}

function toggleConfigSection(sectionId) {
    const section = document.querySelector(`.config-section[data-section="${sectionId}"]`);
    if (section) {
        section.classList.toggle('collapsed');
    }
}

function setupConfigSectionHandlers() {
    // Any additional setup after rendering
}

// Track config changes per section
const configChanges = {};

function markConfigChanged(section) {
    configChanges[section] = true;
    const btn = document.querySelector(`.btn-save[data-section="${section}"]`);
    if (btn) {
        btn.disabled = false;
        btn.classList.add('has-changes');
    }
}

function clearConfigChanged(section) {
    configChanges[section] = false;
    const btn = document.querySelector(`.btn-save[data-section="${section}"]`);
    if (btn) {
        btn.disabled = true;
        btn.classList.remove('has-changes');
    }
}

function getConfigValue(name) {
    const input = document.querySelector(`[data-config="${name}"]`);
    if (!input) return null;

    if (input.type === 'checkbox') {
        return input.checked;
    } else if (input.type === 'number') {
        return parseInt(input.value, 10) || 0;
    } else if (input.tagName === 'SELECT' && input.value !== '' && !isNaN(input.value)) {
        return parseInt(input.value, 10);
    } else {
        return input.value;
    }
}

// Save functions for each config section
async function saveDeviceConfig() {
    const config = {
        role: getConfigValue('device.role'),
        serialEnabled: getConfigValue('device.serialEnabled'),
        debugLogEnabled: getConfigValue('device.debugLogEnabled'),
        nodeInfoBroadcastSecs: getConfigValue('device.nodeInfoBroadcastSecs'),
        doubleTapAsButtonPress: getConfigValue('device.doubleTapAsButtonPress'),
        ledHeartbeatDisabled: getConfigValue('device.ledHeartbeatDisabled'),
        tzdef: getConfigValue('device.tzdef')
    };

    try {
        await api('PUT', '/config/device', config);
        showToast('Device Config Saved', 'Changes sent to device');
        clearConfigChanged('device');
    } catch (e) {
        console.error('Failed to save device config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function saveUserConfig() {
    const config = {
        longName: getConfigValue('user.longName'),
        shortName: getConfigValue('user.shortName')
    };

    try {
        await api('PUT', '/user', config);
        showToast('User Config Saved', 'Changes sent to device');
        clearConfigChanged('user');
        // Update myNode with new values
        if (myNode) {
            myNode.longName = config.longName;
            myNode.shortName = config.shortName;
        }
    } catch (e) {
        console.error('Failed to save user config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function saveLoRaConfig() {
    const config = {
        region: getConfigValue('lora.region'),
        modemPreset: getConfigValue('lora.modemPreset'),
        hopLimit: getConfigValue('lora.hopLimit'),
        txEnabled: getConfigValue('lora.txEnabled'),
        txPower: getConfigValue('lora.txPower'),
        channelNum: getConfigValue('lora.channelNum'),
        overrideDutyCycle: getConfigValue('lora.overrideDutyCycle'),
        frequencyOffset: getConfigValue('lora.frequencyOffset')
    };

    try {
        await api('PUT', '/config/lora', config);
        showToast('LoRa Config Saved', 'Changes sent to device');
        clearConfigChanged('lora');
    } catch (e) {
        console.error('Failed to save lora config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function savePositionConfig() {
    const config = {
        gpsEnabled: getConfigValue('position.gpsEnabled'),
        positionBroadcastSecs: getConfigValue('position.positionBroadcastSecs'),
        fixedPosition: getConfigValue('position.fixedPosition'),
        positionBroadcastSmartEnabled: getConfigValue('position.positionBroadcastSmartEnabled'),
        gpsUpdateInterval: getConfigValue('position.gpsUpdateInterval'),
        broadcastSmartMinimumDistance: getConfigValue('position.broadcastSmartMinimumDistance'),
        broadcastSmartMinimumIntervalSecs: getConfigValue('position.broadcastSmartMinimumIntervalSecs')
    };

    try {
        await api('PUT', '/config/position', config);
        showToast('Position Config Saved', 'Changes sent to device');
        clearConfigChanged('position');
    } catch (e) {
        console.error('Failed to save position config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function savePowerConfig() {
    const config = {
        isPowerSaving: getConfigValue('power.isPowerSaving'),
        onBatteryShutdownAfterSecs: getConfigValue('power.onBatteryShutdownAfterSecs'),
        waitBluetoothSecs: getConfigValue('power.waitBluetoothSecs'),
        lsSecs: getConfigValue('power.lsSecs'),
        minWakeSecs: getConfigValue('power.minWakeSecs')
    };

    try {
        await api('PUT', '/config/power', config);
        showToast('Power Config Saved', 'Changes sent to device');
        clearConfigChanged('power');
    } catch (e) {
        console.error('Failed to save power config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function saveDisplayConfig() {
    const config = {
        screenOnSecs: getConfigValue('display.screenOnSecs'),
        gpsFormat: getConfigValue('display.gpsFormat'),
        autoScreenCarouselSecs: getConfigValue('display.autoScreenCarouselSecs'),
        flipScreen: getConfigValue('display.flipScreen'),
        compassNorthTop: getConfigValue('display.compassNorthTop'),
        units: getConfigValue('display.units'),
        wakeOnTapOrMotion: getConfigValue('display.wakeOnTapOrMotion')
    };

    try {
        await api('PUT', '/config/display', config);
        showToast('Display Config Saved', 'Changes sent to device');
        clearConfigChanged('display');
    } catch (e) {
        console.error('Failed to save display config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function saveBluetoothConfig() {
    const config = {
        enabled: getConfigValue('bluetooth.enabled'),
        mode: getConfigValue('bluetooth.mode'),
        fixedPin: getConfigValue('bluetooth.fixedPin')
    };

    try {
        await api('PUT', '/config/bluetooth', config);
        showToast('Bluetooth Config Saved', 'Changes sent to device');
        clearConfigChanged('bluetooth');
    } catch (e) {
        console.error('Failed to save bluetooth config:', e);
        showToast('Error', e.message, 'error');
    }
}

async function saveNetworkConfig() {
    const config = {
        wifiEnabled: getConfigValue('network.wifiEnabled'),
        wifiSsid: getConfigValue('network.wifiSsid'),
        ntpServer: getConfigValue('network.ntpServer'),
        ethEnabled: getConfigValue('network.ethEnabled')
    };

    // Only include password if it was changed
    const wifiPsk = getConfigValue('network.wifiPsk');
    if (wifiPsk) {
        config.wifiPsk = wifiPsk;
    }

    try {
        await api('PUT', '/config/network', config);
        showToast('Network Config Saved', 'Changes sent to device');
        clearConfigChanged('network');
        // Clear password field after save
        const pskInput = document.querySelector('[data-config="network.wifiPsk"]');
        if (pskInput) pskInput.value = '';
    } catch (e) {
        console.error('Failed to save network config:', e);
        showToast('Error', e.message, 'error');
    }
}

// Device actions
async function rebootDevice() {
    if (!confirm('Are you sure you want to reboot the device?')) return;

    try {
        await api('POST', '/device/reboot');
        showToast('Rebooting', 'Device is rebooting...');
    } catch (e) {
        console.error('Failed to reboot:', e);
        showToast('Error', e.message, 'error');
    }
}

async function shutdownDevice() {
    if (!confirm('Are you sure you want to shutdown the device?')) return;

    try {
        await api('POST', '/device/shutdown');
        showToast('Shutting Down', 'Device is shutting down...');
    } catch (e) {
        console.error('Failed to shutdown:', e);
        showToast('Error', e.message, 'error');
    }
}

async function factoryReset() {
    if (!confirm('WARNING: This will erase all settings! Are you sure?')) return;
    if (!confirm('This action cannot be undone. Really proceed with factory reset?')) return;

    try {
        await api('POST', '/device/factory-reset');
        showToast('Factory Reset', 'Device is resetting to factory defaults...');
    } catch (e) {
        console.error('Failed to factory reset:', e);
        showToast('Error', e.message, 'error');
    }
}

// Current module being edited
let currentModuleId = null;
let currentModuleConfig = null;

// Module display names
const moduleNames = {
    mqtt: 'MQTT',
    serial: 'Serial',
    storeforward: 'Store & Forward',
    telemetry: 'Telemetry',
    rangetest: 'Range Test',
    cannedmsg: 'Canned Messages',
    extnotify: 'External Notification',
    neighborinfo: 'Neighbor Info'
};

// Open module config modal
async function openModuleConfig(moduleId) {
    currentModuleId = moduleId;
    const modal = document.getElementById('moduleModal');
    const title = document.getElementById('moduleModalTitle');
    const body = document.getElementById('moduleModalBody');

    title.textContent = `${moduleNames[moduleId] || moduleId} Configuration`;
    body.innerHTML = '<div style="text-align: center; padding: 2rem;">Loading...</div>';

    modal.classList.add('show');

    try {
        // Fetch module config from API
        const endpoint = getModuleEndpoint(moduleId);
        const data = await api('GET', endpoint);
        currentModuleConfig = data[Object.keys(data)[0]] || {};

        // Render the form
        body.innerHTML = renderModuleForm(moduleId, currentModuleConfig);
    } catch (e) {
        console.error('Failed to load module config:', e);
        body.innerHTML = `<div style="text-align: center; padding: 2rem; color: var(--error);">
            Failed to load configuration: ${e.message}
        </div>`;
    }
}

// Close module modal
function closeModuleModal() {
    const modal = document.getElementById('moduleModal');
    modal.classList.remove('show');
    currentModuleId = null;
    currentModuleConfig = null;
}

// Get API endpoint for module
function getModuleEndpoint(moduleId) {
    const endpoints = {
        mqtt: '/modules/mqtt',
        serial: '/modules/serial',
        storeforward: '/modules/store-forward',
        telemetry: '/modules/telemetry',
        rangetest: '/modules/range-test',
        cannedmsg: '/modules/canned-message',
        extnotify: '/modules/external-notification',
        neighborinfo: '/modules/neighbor-info'
    };
    return endpoints[moduleId] || `/modules/${moduleId}`;
}

// Render module form based on type
function renderModuleForm(moduleId, config) {
    switch (moduleId) {
        case 'mqtt':
            return renderMQTTForm(config);
        case 'serial':
            return renderSerialForm(config);
        case 'storeforward':
            return renderStoreForwardForm(config);
        case 'telemetry':
            return renderTelemetryForm(config);
        case 'rangetest':
            return renderRangeTestForm(config);
        case 'cannedmsg':
            return renderCannedMessageForm(config);
        case 'extnotify':
            return renderExtNotifyForm(config);
        case 'neighborinfo':
            return renderNeighborInfoForm(config);
        default:
            return `<p>Configuration not available for this module.</p>`;
    }
}

// MQTT Form
function renderMQTTForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('mqtt.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Server Settings</div>
        <div class="module-form-group">
            <label class="module-form-label">Server Address</label>
            <input type="text" class="module-form-input" data-module="mqtt.address"
                   value="${config.address || ''}" placeholder="mqtt.meshtastic.org">
            <div class="module-form-hint">MQTT broker address (hostname:port)</div>
        </div>
        <div class="module-form-row">
            <div class="module-form-group">
                <label class="module-form-label">Username</label>
                <input type="text" class="module-form-input" data-module="mqtt.username"
                       value="${config.username || ''}" placeholder="meshdev">
            </div>
            <div class="module-form-group">
                <label class="module-form-label">Password</label>
                <input type="password" class="module-form-input" data-module="mqtt.password"
                       value="" placeholder="Enter new password">
            </div>
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Root Topic</label>
            <input type="text" class="module-form-input" data-module="mqtt.root"
                   value="${config.root || ''}" placeholder="msh">
        </div>

        <div class="module-section-title">Options</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">TLS Encryption</span>
            ${moduleToggle('mqtt.tlsEnabled', config.tlsEnabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">JSON Output</span>
            ${moduleToggle('mqtt.jsonEnabled', config.jsonEnabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Encryption Enabled</span>
            ${moduleToggle('mqtt.encryptionEnabled', config.encryptionEnabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Proxy to Client</span>
            ${moduleToggle('mqtt.proxyToClientEnabled', config.proxyToClientEnabled)}
        </div>

        <div class="module-section-title">Map Reporting</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Map Reporting</span>
            ${moduleToggle('mqtt.mapReportingEnabled', config.mapReportingEnabled)}
        </div>
    `;
}

// Serial Form
function renderSerialForm(config) {
    const baudRates = [
        { value: 0, label: 'Default' },
        { value: 7, label: '9600' },
        { value: 8, label: '19200' },
        { value: 9, label: '38400' },
        { value: 10, label: '57600' },
        { value: 11, label: '115200' },
        { value: 12, label: '230400' },
        { value: 13, label: '460800' },
        { value: 15, label: '921600' }
    ];

    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('serial.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Settings</div>
        <div class="module-form-group">
            <label class="module-form-label">Baud Rate</label>
            <select class="module-form-input" data-module="serial.baud">
                ${baudRates.map(b => `<option value="${b.value}" ${config.baud === b.value ? 'selected' : ''}>${b.label}</option>`).join('')}
            </select>
        </div>
        <div class="module-form-row">
            <div class="module-form-group">
                <label class="module-form-label">RX Pin</label>
                <input type="number" class="module-form-input" data-module="serial.rxd"
                       value="${config.rxd || 0}" min="0">
            </div>
            <div class="module-form-group">
                <label class="module-form-label">TX Pin</label>
                <input type="number" class="module-form-input" data-module="serial.txd"
                       value="${config.txd || 0}" min="0">
            </div>
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Timeout (ms)</label>
            <input type="number" class="module-form-input" data-module="serial.timeout"
                   value="${config.timeout || 0}" min="0">
        </div>

        <div class="module-toggle-row">
            <span class="module-toggle-label">Echo</span>
            ${moduleToggle('serial.echo', config.echo)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Override Console Serial</span>
            ${moduleToggle('serial.overrideConsoleSerialPort', config.overrideConsoleSerialPort)}
        </div>
    `;
}

// Store & Forward Form
function renderStoreForwardForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('storeforward.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Settings</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Heartbeat</span>
            ${moduleToggle('storeforward.heartbeat', config.heartbeat)}
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Max Records</label>
            <input type="number" class="module-form-input" data-module="storeforward.records"
                   value="${config.records || 0}" min="0" max="500">
            <div class="module-form-hint">Maximum number of messages to store (0 = use device default)</div>
        </div>
        <div class="module-form-group">
            <label class="module-form-label">History Return Max</label>
            <input type="number" class="module-form-input" data-module="storeforward.historyReturnMax"
                   value="${config.historyReturnMax || 0}" min="0" max="100">
            <div class="module-form-hint">Max messages to return in one request</div>
        </div>
        <div class="module-form-group">
            <label class="module-form-label">History Return Window (min)</label>
            <input type="number" class="module-form-input" data-module="storeforward.historyReturnWindow"
                   value="${config.historyReturnWindow || 0}" min="0">
            <div class="module-form-hint">Time window for history requests</div>
        </div>
    `;
}

// Telemetry Form
function renderTelemetryForm(config) {
    return `
        <div class="module-section-title">Device Telemetry</div>
        <div class="module-form-group">
            <label class="module-form-label">Update Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="telemetry.deviceUpdateInterval"
                   value="${config.deviceUpdateInterval || 900}" min="0">
            <div class="module-form-hint">How often to send device metrics (0 = disabled)</div>
        </div>

        <div class="module-section-title">Environment Telemetry</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Measurement Enabled</span>
            ${moduleToggle('telemetry.environmentMeasurementEnabled', config.environmentMeasurementEnabled)}
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Update Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="telemetry.environmentUpdateInterval"
                   value="${config.environmentUpdateInterval || 900}" min="0">
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Show on Screen</span>
            ${moduleToggle('telemetry.environmentScreenEnabled', config.environmentScreenEnabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Display Fahrenheit</span>
            ${moduleToggle('telemetry.environmentDisplayFahrenheit', config.environmentDisplayFahrenheit)}
        </div>

        <div class="module-section-title">Air Quality</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('telemetry.airQualityEnabled', config.airQualityEnabled)}
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Update Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="telemetry.airQualityInterval"
                   value="${config.airQualityInterval || 900}" min="0">
        </div>

        <div class="module-section-title">Power Metrics</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Measurement Enabled</span>
            ${moduleToggle('telemetry.powerMeasurementEnabled', config.powerMeasurementEnabled)}
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Update Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="telemetry.powerUpdateInterval"
                   value="${config.powerUpdateInterval || 900}" min="0">
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Show on Screen</span>
            ${moduleToggle('telemetry.powerScreenEnabled', config.powerScreenEnabled)}
        </div>
    `;
}

// Range Test Form
function renderRangeTestForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('rangetest.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Settings</div>
        <div class="module-form-group">
            <label class="module-form-label">Send Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="rangetest.sender"
                   value="${config.sender || 0}" min="0">
            <div class="module-form-hint">Interval between test messages (0 = receive only)</div>
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Save to SD Card</span>
            ${moduleToggle('rangetest.save', config.save)}
        </div>
    `;
}

// Canned Message Form
function renderCannedMessageForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('cannedmsg.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Options</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Send Bell</span>
            ${moduleToggle('cannedmsg.sendBell', config.sendBell)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Rotary Encoder</span>
            ${moduleToggle('cannedmsg.rotaryEnabled', config.rotaryEnabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Upside Down</span>
            ${moduleToggle('cannedmsg.upsideDown', config.upsideDown)}
        </div>

        <div class="module-section-title">Input Source</div>
        <div class="module-form-group">
            <label class="module-form-label">Allow Input Source</label>
            <input type="text" class="module-form-input" data-module="cannedmsg.allowInputSource"
                   value="${config.allowInputSource || ''}" placeholder="_any">
            <div class="module-form-hint">Allowed input source filter</div>
        </div>
    `;
}

// External Notification Form
function renderExtNotifyForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('extnotify.enabled', config.enabled)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Active High</span>
            ${moduleToggle('extnotify.active', config.active)}
        </div>

        <div class="module-section-title">Output Settings</div>
        <div class="module-form-row">
            <div class="module-form-group">
                <label class="module-form-label">LED GPIO</label>
                <input type="number" class="module-form-input" data-module="extnotify.output"
                       value="${config.output || 0}" min="0">
            </div>
            <div class="module-form-group">
                <label class="module-form-label">Duration (ms)</label>
                <input type="number" class="module-form-input" data-module="extnotify.outputMs"
                       value="${config.outputMs || 0}" min="0">
            </div>
        </div>
        <div class="module-form-row">
            <div class="module-form-group">
                <label class="module-form-label">Vibra GPIO</label>
                <input type="number" class="module-form-input" data-module="extnotify.outputVibra"
                       value="${config.outputVibra || 0}" min="0">
            </div>
            <div class="module-form-group">
                <label class="module-form-label">Buzzer GPIO</label>
                <input type="number" class="module-form-input" data-module="extnotify.outputBuzzer"
                       value="${config.outputBuzzer || 0}" min="0">
            </div>
        </div>
        <div class="module-form-group">
            <label class="module-form-label">Nag Timeout (sec)</label>
            <input type="number" class="module-form-input" data-module="extnotify.nagTimeout"
                   value="${config.nagTimeout || 0}" min="0">
        </div>

        <div class="module-section-title">Alert Triggers</div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Message (LED)</span>
            ${moduleToggle('extnotify.alertMessage', config.alertMessage)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Message (Vibra)</span>
            ${moduleToggle('extnotify.alertMessageVibra', config.alertMessageVibra)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Message (Buzzer)</span>
            ${moduleToggle('extnotify.alertMessageBuzzer', config.alertMessageBuzzer)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Bell (LED)</span>
            ${moduleToggle('extnotify.alertBell', config.alertBell)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Bell (Vibra)</span>
            ${moduleToggle('extnotify.alertBellVibra', config.alertBellVibra)}
        </div>
        <div class="module-toggle-row">
            <span class="module-toggle-label">Alert on Bell (Buzzer)</span>
            ${moduleToggle('extnotify.alertBellBuzzer', config.alertBellBuzzer)}
        </div>

        <div class="module-toggle-row">
            <span class="module-toggle-label">Use PWM</span>
            ${moduleToggle('extnotify.usePwm', config.usePwm)}
        </div>
    `;
}

// Neighbor Info Form
function renderNeighborInfoForm(config) {
    return `
        <div class="module-toggle-row">
            <span class="module-toggle-label">Enabled</span>
            ${moduleToggle('neighborinfo.enabled', config.enabled)}
        </div>

        <div class="module-section-title">Settings</div>
        <div class="module-form-group">
            <label class="module-form-label">Update Interval (sec)</label>
            <input type="number" class="module-form-input" data-module="neighborinfo.updateInterval"
                   value="${config.updateInterval || 900}" min="0">
            <div class="module-form-hint">How often to broadcast neighbor info</div>
        </div>
    `;
}

// Helper to create a toggle switch for modules
function moduleToggle(name, checked) {
    return `
        <label class="toggle-switch">
            <input type="checkbox" data-module="${name}" ${checked ? 'checked' : ''}>
            <span class="toggle-slider"></span>
        </label>
    `;
}

// Get module form value
function getModuleValue(name) {
    const input = document.querySelector(`[data-module="${name}"]`);
    if (!input) return null;

    if (input.type === 'checkbox') {
        return input.checked;
    } else if (input.type === 'number') {
        return parseInt(input.value, 10) || 0;
    } else {
        return input.value;
    }
}

// Save current module
async function saveCurrentModule() {
    if (!currentModuleId) return;

    const saveBtn = document.getElementById('moduleModalSave');
    saveBtn.disabled = true;
    saveBtn.textContent = 'Saving...';

    try {
        const config = buildModuleConfig(currentModuleId);
        const endpoint = getModuleEndpoint(currentModuleId);

        await api('PUT', endpoint, config);
        showToast('Module Saved', `${moduleNames[currentModuleId]} configuration saved`);
        closeModuleModal();
        loadConfig(); // Refresh to update module cards
    } catch (e) {
        console.error('Failed to save module config:', e);
        showToast('Error', e.message, 'error');
    } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = 'Save';
    }
}

// Build config object from form
function buildModuleConfig(moduleId) {
    switch (moduleId) {
        case 'mqtt':
            return {
                enabled: getModuleValue('mqtt.enabled'),
                address: getModuleValue('mqtt.address'),
                username: getModuleValue('mqtt.username'),
                password: getModuleValue('mqtt.password') || undefined,
                root: getModuleValue('mqtt.root'),
                tlsEnabled: getModuleValue('mqtt.tlsEnabled'),
                jsonEnabled: getModuleValue('mqtt.jsonEnabled'),
                encryptionEnabled: getModuleValue('mqtt.encryptionEnabled'),
                proxyToClientEnabled: getModuleValue('mqtt.proxyToClientEnabled'),
                mapReportingEnabled: getModuleValue('mqtt.mapReportingEnabled')
            };
        case 'serial':
            return {
                enabled: getModuleValue('serial.enabled'),
                baud: getModuleValue('serial.baud'),
                rxd: getModuleValue('serial.rxd'),
                txd: getModuleValue('serial.txd'),
                timeout: getModuleValue('serial.timeout'),
                echo: getModuleValue('serial.echo'),
                overrideConsoleSerialPort: getModuleValue('serial.overrideConsoleSerialPort')
            };
        case 'storeforward':
            return {
                enabled: getModuleValue('storeforward.enabled'),
                heartbeat: getModuleValue('storeforward.heartbeat'),
                records: getModuleValue('storeforward.records'),
                historyReturnMax: getModuleValue('storeforward.historyReturnMax'),
                historyReturnWindow: getModuleValue('storeforward.historyReturnWindow')
            };
        case 'telemetry':
            return {
                deviceUpdateInterval: getModuleValue('telemetry.deviceUpdateInterval'),
                environmentMeasurementEnabled: getModuleValue('telemetry.environmentMeasurementEnabled'),
                environmentUpdateInterval: getModuleValue('telemetry.environmentUpdateInterval'),
                environmentScreenEnabled: getModuleValue('telemetry.environmentScreenEnabled'),
                environmentDisplayFahrenheit: getModuleValue('telemetry.environmentDisplayFahrenheit'),
                airQualityEnabled: getModuleValue('telemetry.airQualityEnabled'),
                airQualityInterval: getModuleValue('telemetry.airQualityInterval'),
                powerMeasurementEnabled: getModuleValue('telemetry.powerMeasurementEnabled'),
                powerUpdateInterval: getModuleValue('telemetry.powerUpdateInterval'),
                powerScreenEnabled: getModuleValue('telemetry.powerScreenEnabled')
            };
        case 'rangetest':
            return {
                enabled: getModuleValue('rangetest.enabled'),
                sender: getModuleValue('rangetest.sender'),
                save: getModuleValue('rangetest.save')
            };
        case 'cannedmsg':
            return {
                enabled: getModuleValue('cannedmsg.enabled'),
                sendBell: getModuleValue('cannedmsg.sendBell'),
                rotaryEnabled: getModuleValue('cannedmsg.rotaryEnabled'),
                upsideDown: getModuleValue('cannedmsg.upsideDown'),
                allowInputSource: getModuleValue('cannedmsg.allowInputSource')
            };
        case 'extnotify':
            return {
                enabled: getModuleValue('extnotify.enabled'),
                active: getModuleValue('extnotify.active'),
                output: getModuleValue('extnotify.output'),
                outputMs: getModuleValue('extnotify.outputMs'),
                outputVibra: getModuleValue('extnotify.outputVibra'),
                outputBuzzer: getModuleValue('extnotify.outputBuzzer'),
                nagTimeout: getModuleValue('extnotify.nagTimeout'),
                alertMessage: getModuleValue('extnotify.alertMessage'),
                alertMessageVibra: getModuleValue('extnotify.alertMessageVibra'),
                alertMessageBuzzer: getModuleValue('extnotify.alertMessageBuzzer'),
                alertBell: getModuleValue('extnotify.alertBell'),
                alertBellVibra: getModuleValue('extnotify.alertBellVibra'),
                alertBellBuzzer: getModuleValue('extnotify.alertBellBuzzer'),
                usePwm: getModuleValue('extnotify.usePwm')
            };
        case 'neighborinfo':
            return {
                enabled: getModuleValue('neighborinfo.enabled'),
                updateInterval: getModuleValue('neighborinfo.updateInterval')
            };
        default:
            return {};
    }
}

// Close modal on backdrop click
document.addEventListener('click', (e) => {
    const modal = document.getElementById('moduleModal');
    if (e.target === modal) {
        closeModuleModal();
    }
});

// Close modal on Escape key
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeModuleModal();
    }
});

// Map Functions
let neighborLines = {};
let mapShowNeighbors = true;
let mapShowMqtt = true;
let mapShowOffline = true;

function initMap() {
    const mapContainer = document.getElementById('leafletMap');
    if (!mapContainer) return;

    // Initialize map if not already done
    if (!leafletMap) {
        // Default center on Spain - will be adjusted when nodes load
        leafletMap = L.map('leafletMap').setView([39.5, -0.5], 7);

        // Add OpenStreetMap tiles with dark mode support
        const isDark = document.documentElement.classList.contains('dark');
        const tileUrl = isDark
            ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
            : 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';

        L.tileLayer(tileUrl, {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
            maxZoom: 19
        }).addTo(leafletMap);

        // Add map legend
        addMapLegend();

        // Add map controls
        addMapControls();

        // Try to center on my node first, then geolocation
        if (myNode && myNode.latitude && myNode.longitude) {
            leafletMap.setView([myNode.latitude, myNode.longitude], 12);
        } else if (navigator.geolocation) {
            navigator.geolocation.getCurrentPosition(
                (position) => {
                    if (!myNode || !myNode.latitude) {
                        leafletMap.setView([position.coords.latitude, position.coords.longitude], 10);
                    }
                },
                () => console.log('Could not get user location')
            );
        }

        // Force map to recalculate size
        setTimeout(() => {
            leafletMap.invalidateSize();
            updateMapMarkers();
        }, 100);
    } else {
        leafletMap.invalidateSize();
    }

    updateMapMarkers();
}

function addMapLegend() {
    const legend = L.control({ position: 'bottomleft' });
    legend.onAdd = function() {
        const div = L.DomUtil.create('div', 'map-legend');
        div.innerHTML = `
            <div class="map-legend-item"><span class="map-legend-dot me"></span> My Node</div>
            <div class="map-legend-item"><span class="map-legend-dot online"></span> Online</div>
            <div class="map-legend-item"><span class="map-legend-dot offline"></span> Offline</div>
            <div class="map-legend-item"><span class="map-legend-dot mqtt"></span> Via MQTT</div>
        `;
        return div;
    };
    legend.addTo(leafletMap);
}

function addMapControls() {
    const controls = L.control({ position: 'topright' });
    controls.onAdd = function() {
        const div = L.DomUtil.create('div', 'map-controls');
        div.innerHTML = `
            <button class="map-control-btn ${mapShowNeighbors ? 'active' : ''}" id="mapToggleNeighbors" title="Show neighbor connections">⟷</button>
            <button class="map-control-btn ${mapShowMqtt ? 'active' : ''}" id="mapToggleMqtt" title="Show MQTT nodes">M</button>
            <button class="map-control-btn ${mapShowOffline ? 'active' : ''}" id="mapToggleOffline" title="Show offline nodes">◐</button>
            <button class="map-control-btn" id="mapCenterMyNode" title="Center on my node">⌖</button>
            <button class="map-control-btn" id="mapFitAll" title="Fit all nodes">□</button>
            <button class="map-control-btn" id="mapToggleWaypoints" title="Show/hide waypoints">📍</button>
        `;

        // Prevent map interactions when clicking controls
        L.DomEvent.disableClickPropagation(div);

        return div;
    };
    controls.addTo(leafletMap);

    // Add event listeners after controls are added
    setTimeout(() => {
        document.getElementById('mapToggleNeighbors')?.addEventListener('click', () => {
            mapShowNeighbors = !mapShowNeighbors;
            document.getElementById('mapToggleNeighbors').classList.toggle('active', mapShowNeighbors);
            updateNeighborLines();
        });

        document.getElementById('mapToggleMqtt')?.addEventListener('click', () => {
            mapShowMqtt = !mapShowMqtt;
            document.getElementById('mapToggleMqtt').classList.toggle('active', mapShowMqtt);
            updateMapMarkers();
        });

        document.getElementById('mapToggleOffline')?.addEventListener('click', () => {
            mapShowOffline = !mapShowOffline;
            document.getElementById('mapToggleOffline').classList.toggle('active', mapShowOffline);
            updateMapMarkers();
        });

        document.getElementById('mapCenterMyNode')?.addEventListener('click', () => {
            if (myNode && myNode.latitude && myNode.longitude) {
                leafletMap.setView([myNode.latitude, myNode.longitude], 14);
            }
        });

        document.getElementById('mapFitAll')?.addEventListener('click', () => {
            const nodesWithPosition = Object.values(nodes).filter(n =>
                n.latitude && n.longitude && n.latitude !== 0 && n.longitude !== 0
            );
            if (nodesWithPosition.length > 0) {
                const bounds = L.latLngBounds(nodesWithPosition.map(n => [n.latitude, n.longitude]));
                leafletMap.fitBounds(bounds, { padding: [50, 50], maxZoom: 15 });
            }
        });

        document.getElementById('mapToggleWaypoints')?.addEventListener('click', () => {
            const panel = document.getElementById('waypointPanel');
            if (panel) {
                if (panel.classList.contains('hidden')) {
                    showWaypointPanel();
                } else {
                    panel.classList.toggle('collapsed');
                }
            }
        });
    }, 100);
}

function updateMapMarkers() {
    if (!leafletMap) return;

    const now = Date.now() / 1000;

    let nodesWithPosition = Object.values(nodes).filter(n =>
        n.latitude && n.longitude && n.latitude !== 0 && n.longitude !== 0
    );

    // Apply filters
    if (!mapShowMqtt) {
        nodesWithPosition = nodesWithPosition.filter(n => !n.viaMqtt);
    }
    if (!mapShowOffline) {
        nodesWithPosition = nodesWithPosition.filter(n => n.lastHeard && (now - n.lastHeard) < 3600);
    }

    // Track which markers should exist
    const validNums = new Set(nodesWithPosition.map(n => n.num));

    // Remove old markers
    Object.keys(mapMarkers).forEach(num => {
        if (!validNums.has(parseInt(num))) {
            leafletMap.removeLayer(mapMarkers[num]);
            delete mapMarkers[num];
        }
    });

    // Add/update markers
    nodesWithPosition.forEach(node => {
        updateMapMarker(node);
    });

    // Update neighbor lines
    updateNeighborLines();

    // Only fit bounds on first load (when we have no markers yet)
    if (Object.keys(mapMarkers).length === nodesWithPosition.length && nodesWithPosition.length > 0) {
        // Check if this is first time loading markers
        if (!leafletMap._boundsSet && nodesWithPosition.length > 1) {
            // Center on connected node if position available, otherwise fit all
            if (myNode && myNode.latitude && myNode.longitude) {
                leafletMap.setView([myNode.latitude, myNode.longitude], 14);
            } else {
                const bounds = L.latLngBounds(nodesWithPosition.map(n => [n.latitude, n.longitude]));
                leafletMap.fitBounds(bounds, { padding: [50, 50], maxZoom: 15 });
            }
            leafletMap._boundsSet = true;
        }
    }
}

function updateMapMarker(node) {
    if (!leafletMap || !node.latitude || !node.longitude || node.latitude === 0 || node.longitude === 0) {
        return;
    }

    // Debug: log coordinates to verify they're correct
    console.debug(`Map marker ${node.shortName || node.num}: lat=${node.latitude}, lon=${node.longitude}`);

    const now = Date.now() / 1000;
    const shortName = node.shortName || '??';
    const isOnline = node.lastHeard && (now - node.lastHeard) < 3600;
    const isMyNode = myNode && myNode.num === node.num;

    // Determine marker class
    let markerClass = 'map-marker-node';
    if (isMyNode) {
        markerClass += ' me';
    } else if (node.viaMqtt) {
        markerClass += ' mqtt';
    } else if (isOnline) {
        markerClass += ' online';
    } else {
        markerClass += ' offline';
    }
    if (node.isFavorite && !isMyNode) {
        markerClass += ' favorite';
    }

    // Create custom icon - sizes must match CSS (.map-marker-node: 36px, .map-marker-node.me: 42px)
    const iconSize = isMyNode ? 42 : 36;
    const icon = L.divIcon({
        className: 'map-marker-container',
        html: isMyNode
            ? `<div style="position:relative;width:${iconSize}px;height:${iconSize}px;display:flex;align-items:center;justify-content:center;">
                 <div class="map-marker-pulse"></div>
                 <div class="${markerClass}">${shortName.substring(0, 4).toUpperCase()}</div>
               </div>`
            : `<div class="${markerClass}">${shortName.substring(0, 4).toUpperCase()}</div>`,
        iconSize: [iconSize, iconSize],
        iconAnchor: [iconSize/2, iconSize/2]
    });

    // Build enhanced popup content
    const popupContent = buildMapPopup(node, isOnline);

    if (mapMarkers[node.num]) {
        // Update existing marker
        mapMarkers[node.num].setLatLng([node.latitude, node.longitude]);
        mapMarkers[node.num].setIcon(icon);
        mapMarkers[node.num].setPopupContent(popupContent);
    } else {
        // Create new marker
        const marker = L.marker([node.latitude, node.longitude], { icon })
            .addTo(leafletMap);

        marker.bindPopup(popupContent, { maxWidth: 280 });

        marker.on('click', () => {
            selectNode(node.num);
        });

        mapMarkers[node.num] = marker;
    }
}

function buildMapPopup(node, isOnline) {
    const shortName = node.shortName || '??';
    const nodeId = `!${(node.num >>> 0).toString(16).toLowerCase()}`;

    let gridRows = '';

    // Position
    gridRows += `<span class="map-popup-label">Position</span><span class="map-popup-value">${node.latitude.toFixed(5)}, ${node.longitude.toFixed(5)}</span>`;

    // Altitude
    if (node.altitude) {
        gridRows += `<span class="map-popup-label">Altitude</span><span class="map-popup-value">${node.altitude}m</span>`;
    }

    // Distance (if not my node and we have reference)
    if (node.distanceStr && node.bearingCardinal) {
        gridRows += `<span class="map-popup-label">Distance</span><span class="map-popup-value">${node.distanceStr} ${node.bearingCardinal}</span>`;
    }

    // Battery
    if (node.batteryLevel && node.batteryLevel > 0 && node.batteryLevel <= 100) {
        gridRows += `<span class="map-popup-label">Battery</span><span class="map-popup-value">${node.batteryLevel}%</span>`;
    }

    // Signal
    if (node.snr !== undefined && node.snr !== 0 && node.snr < 100) {
        gridRows += `<span class="map-popup-label">SNR</span><span class="map-popup-value">${node.snr.toFixed(1)} dB</span>`;
    }

    // Hops
    if (node.hopsAway !== undefined && node.hopsAway >= 0) {
        gridRows += `<span class="map-popup-label">Hops</span><span class="map-popup-value">${node.hopsAway}</span>`;
    }

    // Last heard
    if (node.lastHeard) {
        gridRows += `<span class="map-popup-label">Last heard</span><span class="map-popup-value">${formatLastHeard(node.lastHeard)}</span>`;
    }

    // Hardware
    if (node.hardwareModel) {
        gridRows += `<span class="map-popup-label">Hardware</span><span class="map-popup-value">${formatHwShort(node.hardwareModel)}</span>`;
    }

    return `
        <div class="map-node-popup">
            <div class="map-popup-header">
                <div class="map-popup-avatar">${shortName.substring(0, 4).toUpperCase()}</div>
                <div>
                    <div class="map-popup-name">${escapeHtml(node.longName || node.shortName || 'Unknown')}</div>
                    <div class="map-popup-id">${nodeId}</div>
                </div>
            </div>
            <div class="map-popup-grid">${gridRows}</div>
            <div class="map-popup-actions">
                <button class="map-popup-btn" onclick="sendDirectMessage(${node.num})">Message</button>
                <button class="map-popup-btn" onclick="requestNodePosition(${node.num})">Request Pos</button>
            </div>
        </div>
    `;
}

function updateNeighborLines() {
    if (!leafletMap) return;

    // Remove all existing lines
    Object.values(neighborLines).forEach(line => {
        leafletMap.removeLayer(line);
    });
    neighborLines = {};

    if (!mapShowNeighbors) return;

    // Draw lines between nodes that have neighbor relationships
    Object.values(nodes).forEach(node => {
        if (!node.neighbors || !node.latitude || !node.longitude) return;

        node.neighbors.forEach(neighbor => {
            const neighborNode = nodes[neighbor.nodeId];
            if (!neighborNode || !neighborNode.latitude || !neighborNode.longitude) return;

            // Create unique key for this connection (smaller num first)
            const key = node.num < neighbor.nodeId
                ? `${node.num}-${neighbor.nodeId}`
                : `${neighbor.nodeId}-${node.num}`;

            if (neighborLines[key]) return; // Already drawn

            // Calculate line color based on SNR
            let color = '#306A42';
            if (neighbor.snr !== undefined) {
                if (neighbor.snr < -10) color = '#F44336';
                else if (neighbor.snr < 0) color = '#FF8800';
                else if (neighbor.snr < 5) color = '#FFD54F';
            }

            const line = L.polyline([
                [node.latitude, node.longitude],
                [neighborNode.latitude, neighborNode.longitude]
            ], {
                color: color,
                weight: 2,
                opacity: 0.6,
                dashArray: '5, 10'
            }).addTo(leafletMap);

            // Add tooltip showing SNR
            if (neighbor.snr !== undefined) {
                line.bindTooltip(`SNR: ${neighbor.snr.toFixed(1)} dB`, { sticky: true });
            }

            neighborLines[key] = line;
        });
    });
}

function removeMapMarker(num) {
    if (mapMarkers[num]) {
        leafletMap.removeLayer(mapMarkers[num]);
        delete mapMarkers[num];
    }
}

function sendDirectMessage(nodeNum) {
    selectNode(nodeNum);
    // Focus message input
    const msgInput = document.getElementById('msgInput');
    if (msgInput) {
        msgInput.focus();
    }
}

function requestNodePosition(nodeNum) {
    api('POST', `/nodes/${nodeNum}/request-position`)
        .then(() => showToast('Position request sent'))
        .catch(e => showToast('Failed to request position: ' + e.message, 'error'));
}

// Add spinning animation for scan button
const style = document.createElement('style');
style.textContent = `
    @keyframes spin {
        from { transform: rotate(0deg); }
        to { transform: rotate(360deg); }
    }
    .spin {
        animation: spin 1s linear infinite;
    }
`;
document.head.appendChild(style);

// Node Commands
let pendingTraceroute = null;

async function sendTraceroute() {
    if (!selectedNode) return;

    const targetNode = selectedNode;
    pendingTraceroute = targetNode.num;

    try {
        const data = await api('POST', `/traceroute/${targetNode.num}`);
        console.log('Traceroute initiated:', data);
        showToast('Traceroute sent', `Waiting for ${targetNode.shortName || targetNode.longName || 'node'} to respond...`);

        // Set timeout for no response
        setTimeout(() => {
            if (pendingTraceroute === targetNode.num) {
                pendingTraceroute = null;
                // Don't show error - response might still come
            }
        }, 30000);
    } catch (e) {
        pendingTraceroute = null;
        console.error('Traceroute failed:', e);
        showToast('Error', e.message, 'error');
    }
}

async function requestPosition() {
    if (!selectedNode) return;

    try {
        await api('POST', `/nodes/${selectedNode.num}/request-position`);
        showToast('Position requested', 'Waiting for response...');
    } catch (e) {
        console.error('Request position failed:', e);
        showToast('Error', e.message, 'error');
    }
}

async function requestNodeInfo() {
    if (!selectedNode) return;

    try {
        await api('POST', `/nodes/${selectedNode.num}/request-info`);
        showToast('Info requested', 'Waiting for response...');
    } catch (e) {
        console.error('Request info failed:', e);
        showToast('Error', e.message, 'error');
    }
}

async function requestTelemetry() {
    if (!selectedNode) return;

    try {
        await api('POST', `/nodes/${selectedNode.num}/request-telemetry`);
        showToast('Telemetry requested', 'Waiting for device/environment/stats data...');
    } catch (e) {
        console.error('Request telemetry failed:', e);
        showToast('Error', e.message, 'error');
    }
}

async function requestNeighborInfo() {
    if (!selectedNode) return;

    try {
        await api('POST', `/nodes/${selectedNode.num}/request-neighbor-info`);
        showToast('Neighbor info requested', 'Waiting for neighbor list...');
    } catch (e) {
        console.error('Request neighbor info failed:', e);
        showToast('Error', e.message, 'error');
    }
}

async function toggleFavorite() {
    if (!selectedNode) return;

    const newValue = !selectedNode.isFavorite;
    try {
        await api('PUT', `/nodes/${selectedNode.num}/favorite`, { favorite: newValue });
        selectedNode.isFavorite = newValue;
        nodes[selectedNode.num].isFavorite = newValue;
        updateFavoriteButton();
        renderNodeList();
        showToast(newValue ? 'Added to favorites' : 'Removed from favorites');
    } catch (e) {
        console.error('Toggle favorite failed:', e);
        showToast('Error', e.message, 'error');
    }
}

function updateFavoriteButton() {
    const btn = document.getElementById('favoriteBtn');
    if (!btn || !selectedNode) return;

    if (selectedNode.isFavorite) {
        btn.classList.add('active');
    } else {
        btn.classList.remove('active');
    }
}

// Handle traceroute response
let lastTracerouteData = null;
let tracerouteLines = [];

function handleTracerouteResponse(data) {
    console.log('Traceroute response:', data);
    pendingTraceroute = null; // Clear pending state
    lastTracerouteData = data; // Store for map visualization

    const { from, route, routeBack, snrTowards, snrBack } = data;

    // Build route display with node names
    const routeDisplay = route.map((nodeNum, i) => {
        const node = nodes[nodeNum];
        const name = node ? (node.shortName || node.longName || `!${(nodeNum >>> 0).toString(16)}`) : `!${(nodeNum >>> 0).toString(16)}`;
        const snr = snrTowards && snrTowards[i] !== undefined ? ` (${snrTowards[i]} dB)` : '';
        return `${name}${snr}`;
    });

    const routeBackDisplay = routeBack && routeBack.length > 0 ? routeBack.map((nodeNum, i) => {
        const node = nodes[nodeNum];
        const name = node ? (node.shortName || node.longName || `!${(nodeNum >>> 0).toString(16)}`) : `!${(nodeNum >>> 0).toString(16)}`;
        const snr = snrBack && snrBack[i] !== undefined ? ` (${snrBack[i]} dB)` : '';
        return `${name}${snr}`;
    }) : [];

    // Show modal with traceroute results
    showTracerouteModal(from, route, routeDisplay, routeBackDisplay);
}

// Show traceroute modal
function showTracerouteModal(from, route, routeDisplay, routeBackDisplay) {
    // Remove existing modal
    const existing = document.querySelector('.traceroute-modal');
    if (existing) existing.remove();

    const destNode = nodes[from];
    const destName = destNode ? (destNode.longName || destNode.shortName || `!${(from >>> 0).toString(16)}`) : `!${(from >>> 0).toString(16)}`;

    // Check if we can show on map (need positions for nodes)
    const canShowOnMap = route.some(nodeNum => {
        const node = nodes[nodeNum];
        return node && node.latitude && node.longitude;
    });

    let html = `
        <div class="traceroute-modal">
            <div class="traceroute-content">
                <div class="traceroute-header">
                    <h3>Traceroute to ${destName}</h3>
                    <button class="traceroute-close" onclick="closeTracerouteModal()">&times;</button>
                </div>
                <div class="traceroute-body">
                    <div class="traceroute-section">
                        <div class="traceroute-label">Route (${routeDisplay.length} hops)</div>
                        <div class="traceroute-route">
    `;

    // Show route as path
    html += `<div class="traceroute-path">`;
    html += `<div class="traceroute-node origin">Me</div>`;
    routeDisplay.forEach((hop, i) => {
        html += `<div class="traceroute-arrow">→</div>`;
        html += `<div class="traceroute-node ${i === routeDisplay.length - 1 ? 'destination' : ''}">${hop}</div>`;
    });
    html += `</div>`;

    html += `</div></div>`;

    if (routeBackDisplay.length > 0) {
        html += `
            <div class="traceroute-section">
                <div class="traceroute-label">Return route (${routeBackDisplay.length} hops)</div>
                <div class="traceroute-route">
                    <div class="traceroute-path">
        `;
        html += `<div class="traceroute-node destination">${destName}</div>`;
        routeBackDisplay.forEach((hop, i) => {
            html += `<div class="traceroute-arrow">→</div>`;
            html += `<div class="traceroute-node ${i === routeBackDisplay.length - 1 ? 'origin' : ''}">${hop}</div>`;
        });
        html += `<div class="traceroute-arrow">→</div>`;
        html += `<div class="traceroute-node origin">Me</div>`;
        html += `</div></div></div>`;
    }

    // Add View on Map button if we have positions
    if (canShowOnMap) {
        html += `
            <div class="traceroute-actions" style="margin-top: 1rem; display: flex; gap: 0.5rem;">
                <button class="btn btn-primary" onclick="showTracerouteOnMap()">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" style="margin-right: 0.25rem;">
                        <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
                    </svg>
                    View on Map
                </button>
                <button class="btn btn-secondary" onclick="closeTracerouteModal()">Close</button>
            </div>
        `;
    }

    html += `
                </div>
            </div>
        </div>
    `;

    document.body.insertAdjacentHTML('beforeend', html);

    // Animate in
    setTimeout(() => {
        document.querySelector('.traceroute-modal').classList.add('show');
    }, 10);
}

// Close traceroute modal
function closeTracerouteModal() {
    const modal = document.querySelector('.traceroute-modal');
    if (modal) {
        modal.classList.remove('show');
        setTimeout(() => modal.remove(), 300);
    }
}

// Show traceroute on map
function showTracerouteOnMap() {
    if (!lastTracerouteData) {
        showToast('No traceroute data available', 'error');
        return;
    }

    closeTracerouteModal();

    // Switch to map tab (this will call initMap if needed)
    selectMobileTab('map');

    // Delay to ensure map is initialized and visible
    setTimeout(() => {
        if (!leafletMap) {
            showToast('Map not available', 'error');
            return;
        }
        drawTracerouteOnMap(lastTracerouteData);
    }, 200);
}

// Draw traceroute lines on the map
function drawTracerouteOnMap(data) {
    if (!leafletMap) return;

    // Clear existing traceroute lines
    clearTracerouteLines();

    const { route, snrTowards } = data;

    // Get coordinates for each node in the route
    const coordinates = [];
    const routeWithMe = [myNode?.num || 0, ...route];

    for (let i = 0; i < routeWithMe.length; i++) {
        const nodeNum = routeWithMe[i];
        const node = nodes[nodeNum];

        if (node && node.latitude && node.longitude) {
            coordinates.push({
                nodeNum,
                lat: node.latitude,
                lon: node.longitude,
                name: node.shortName || node.longName || `!${(nodeNum >>> 0).toString(16)}`,
                snr: snrTowards && snrTowards[i - 1] !== undefined ? snrTowards[i - 1] : null
            });
        } else if (i === 0 && myNode?.latitude && myNode?.longitude) {
            // Use myNode position for the first hop
            coordinates.push({
                nodeNum: myNode.num,
                lat: myNode.latitude,
                lon: myNode.longitude,
                name: 'Me',
                snr: null
            });
        }
    }

    if (coordinates.length < 2) {
        const missingNodes = routeWithMe
            .filter(num => !nodes[num]?.latitude || !nodes[num]?.longitude)
            .map(num => nodes[num]?.shortName || `!${(num >>> 0).toString(16)}`)
            .join(', ');
        showToast(`Cannot display route: missing position for ${missingNodes || 'nodes in route'}`, 'warning');
        return;
    }

    // Draw lines between consecutive nodes
    for (let i = 0; i < coordinates.length - 1; i++) {
        const from = coordinates[i];
        const to = coordinates[i + 1];

        // Determine color based on SNR
        let color = '#E91E63'; // Pink/magenta for traceroute
        if (to.snr !== null) {
            if (to.snr >= 5) color = '#4CAF50'; // Good SNR - green
            else if (to.snr >= 0) color = '#FFC107'; // Medium SNR - yellow
            else color = '#F44336'; // Poor SNR - red
        }

        // Create polyline
        const line = L.polyline(
            [[from.lat, from.lon], [to.lat, to.lon]],
            {
                color: color,
                weight: 4,
                opacity: 0.8,
                dashArray: '10, 5',
                className: 'traceroute-line'
            }
        ).addTo(leafletMap);

        // Add popup with info
        const snrText = to.snr !== null ? `SNR: ${to.snr} dB` : '';
        line.bindPopup(`<strong>${from.name}</strong> → <strong>${to.name}</strong><br>${snrText}`);

        // Add animated arrow
        const arrowIcon = L.divIcon({
            className: 'traceroute-arrow-marker',
            html: `<div style="transform: rotate(${getBearing(from.lat, from.lon, to.lat, to.lon)}deg); color: ${color};">➤</div>`,
            iconSize: [20, 20],
            iconAnchor: [10, 10]
        });

        const midLat = (from.lat + to.lat) / 2;
        const midLon = (from.lon + to.lon) / 2;
        const arrowMarker = L.marker([midLat, midLon], { icon: arrowIcon }).addTo(leafletMap);

        tracerouteLines.push(line);
        tracerouteLines.push(arrowMarker);
    }

    // Fit map to show entire route, then center on local node
    const bounds = L.latLngBounds(coordinates.map(c => [c.lat, c.lon]));
    leafletMap.fitBounds(bounds, { padding: [50, 50] });
    if (myNode?.latitude && myNode?.longitude) {
        leafletMap.setView([myNode.latitude, myNode.longitude], leafletMap.getZoom());
    }

    // Add legend for traceroute
    addTracerouteLegend();

    showToast(`Showing route with ${coordinates.length} nodes`);
}

// Get bearing between two points
function getBearing(lat1, lon1, lat2, lon2) {
    const dLon = (lon2 - lon1) * Math.PI / 180;
    const y = Math.sin(dLon) * Math.cos(lat2 * Math.PI / 180);
    const x = Math.cos(lat1 * Math.PI / 180) * Math.sin(lat2 * Math.PI / 180) -
              Math.sin(lat1 * Math.PI / 180) * Math.cos(lat2 * Math.PI / 180) * Math.cos(dLon);
    return (Math.atan2(y, x) * 180 / Math.PI + 360) % 360;
}

// Clear traceroute lines from map
function clearTracerouteLines() {
    if (!leafletMap) return;

    tracerouteLines.forEach(item => {
        leafletMap.removeLayer(item);
    });
    tracerouteLines = [];

    // Remove legend
    const legend = document.getElementById('tracerouteLegend');
    if (legend) legend.remove();
}

// Add traceroute legend to map
function addTracerouteLegend() {
    // Remove existing
    const existing = document.getElementById('tracerouteLegend');
    if (existing) existing.remove();

    const mapContainer = document.getElementById('mapContainer');
    if (!mapContainer) return;

    const legend = document.createElement('div');
    legend.id = 'tracerouteLegend';
    legend.style.cssText = `
        position: absolute;
        top: 10px;
        left: 10px;
        z-index: 1000;
        background: var(--surface);
        padding: 0.75rem;
        border-radius: 8px;
        box-shadow: 0 2px 8px rgba(0,0,0,0.2);
        font-size: 0.75rem;
    `;
    legend.innerHTML = `
        <div style="font-weight: 600; margin-bottom: 0.5rem;">Traceroute</div>
        <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
            <div style="width: 20px; height: 3px; background: #4CAF50;"></div>
            <span>Good SNR (≥5 dB)</span>
        </div>
        <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
            <div style="width: 20px; height: 3px; background: #FFC107;"></div>
            <span>Medium SNR (0-5 dB)</span>
        </div>
        <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.5rem;">
            <div style="width: 20px; height: 3px; background: #F44336;"></div>
            <span>Poor SNR (&lt;0 dB)</span>
        </div>
        <button class="btn btn-sm" onclick="clearTracerouteLines()" style="width: 100%;">Clear Route</button>
    `;
    mapContainer.appendChild(legend);
}

// Toast notifications
function showToast(title, message = '', type = 'info') {
    // Remove existing toast
    const existing = document.querySelector('.toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.innerHTML = `
        <div class="toast-title">${title}</div>
        ${message ? `<div class="toast-message">${message}</div>` : ''}
    `;
    document.body.appendChild(toast);

    // Animate in
    setTimeout(() => toast.classList.add('show'), 10);

    // Remove after 3 seconds
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ============================================
// TELEMETRY DASHBOARD
// ============================================

let telemetryData = {}; // nodeNum -> { device: {}, environment: {}, power: {}, airQuality: {} }
let selectedTelemetryNode = null;

// Initialize telemetry dashboard
function initTelemetryDashboard() {
    const nodeSelect = document.getElementById('telemetryNodeSelect');
    const refreshBtn = document.getElementById('refreshTelemetryBtn');

    if (nodeSelect) {
        nodeSelect.addEventListener('change', (e) => {
            const nodeNum = parseInt(e.target.value);
            if (nodeNum) {
                selectedTelemetryNode = nodeNum;
                loadNodeTelemetry(nodeNum);
            } else {
                selectedTelemetryNode = null;
                clearTelemetryDisplay();
            }
        });
    }

    if (refreshBtn) {
        refreshBtn.addEventListener('click', () => {
            if (selectedTelemetryNode) {
                requestTelemetryForNode(selectedTelemetryNode);
            }
        });
    }
}

// Update node selector - only show nodes that have received telemetry data
async function updateTelemetryNodeSelector() {
    const nodeSelect = document.getElementById('telemetryNodeSelect');
    if (!nodeSelect) return;

    const currentValue = nodeSelect.value;

    // Get nodes from in-memory data
    const inMemoryNodes = new Set(
        Object.keys(telemetryData)
            .map(num => parseInt(num))
            .filter(num => {
                const data = telemetryData[num];
                return data && (data.device || data.environment || data.power || data.airQuality);
            })
    );

    // Also fetch nodes with historical data from database
    let dbNodes = [];
    try {
        const data = await api('GET', '/telemetry/nodes');
        if (data.nodes) {
            dbNodes = data.nodes;
        }
    } catch (e) {
        console.warn('Failed to load telemetry nodes from database:', e);
    }

    // Merge both sources
    const allNodes = new Set([...inMemoryNodes, ...dbNodes]);
    const nodesWithTelemetry = Array.from(allNodes).sort((a, b) => {
        // Sort by name
        const nodeA = nodes[a];
        const nodeB = nodes[b];
        const nameA = nodeA?.shortName || nodeA?.longName || '';
        const nameB = nodeB?.shortName || nodeB?.longName || '';
        return nameA.localeCompare(nameB);
    });

    nodeSelect.innerHTML = '<option value="">Select a node...</option>';

    if (nodesWithTelemetry.length === 0) {
        const option = document.createElement('option');
        option.disabled = true;
        option.textContent = 'No telemetry data available';
        nodeSelect.appendChild(option);
        return;
    }

    nodesWithTelemetry.forEach(nodeNum => {
        const node = nodes[nodeNum];
        const name = node?.longName || node?.shortName || `Node ${nodeNum}`;
        const nodeId = `!${(nodeNum >>> 0).toString(16)}`;
        const hasLiveData = inMemoryNodes.has(nodeNum);
        const option = document.createElement('option');
        option.value = nodeNum;
        option.textContent = `${name} (${nodeId})${hasLiveData ? '' : ' [historical]'}`;
        nodeSelect.appendChild(option);
    });

    // Restore selection if still valid
    if (currentValue && allNodes.has(parseInt(currentValue))) {
        nodeSelect.value = currentValue;
    }
}

// Load telemetry for a specific node
async function loadNodeTelemetry(nodeNum) {
    try {
        const data = await api('GET', `/telemetry/${nodeNum}`);
        if (data.telemetry) {
            telemetryData[nodeNum] = data.telemetry;
            renderTelemetryDashboard(nodeNum);
        }
    } catch (e) {
        console.error('Failed to load telemetry:', e);
    }
}

// Request fresh telemetry from node
async function requestTelemetryForNode(nodeNum) {
    try {
        await api('POST', `/telemetry/${nodeNum}/request`);
        showToast('Telemetry requested', 'Waiting for response...');
    } catch (e) {
        console.error('Failed to request telemetry:', e);
        showToast('Error', e.message, 'error');
    }
}

// Handle incoming device telemetry via WebSocket
function handleDeviceTelemetry(data) {
    const { nodeNum, batteryLevel, voltage, channelUtilization, airUtilTx, uptimeSeconds } = data;

    const isNewNode = !telemetryData[nodeNum];
    if (!telemetryData[nodeNum]) telemetryData[nodeNum] = {};
    telemetryData[nodeNum].device = {
        batteryLevel,
        voltage,
        channelUtilization,
        airUtilTx,
        uptimeSeconds,
        timestamp: Date.now()
    };

    // Also update the node object
    if (nodes[nodeNum]) {
        nodes[nodeNum].batteryLevel = batteryLevel;
        nodes[nodeNum].voltage = voltage;
        nodes[nodeNum].channelUtilization = channelUtilization;
        nodes[nodeNum].airUtilTx = airUtilTx;
        nodes[nodeNum].uptime = uptimeSeconds;
    }

    // Update selector if this is first telemetry from this node
    if (isNewNode) {
        updateTelemetryNodeSelector();
    }

    if (selectedTelemetryNode === nodeNum) {
        renderTelemetryDashboard(nodeNum);
    }
}

// Handle incoming environment telemetry via WebSocket
function handleEnvironmentTelemetry(data) {
    const { nodeNum, temperature, relativeHumidity, barometricPressure, iaq } = data;

    const isNewNode = !telemetryData[nodeNum];
    if (!telemetryData[nodeNum]) telemetryData[nodeNum] = {};
    telemetryData[nodeNum].environment = {
        temperature,
        relativeHumidity,
        barometricPressure,
        iaq,
        timestamp: Date.now()
    };

    // Also update the node object
    if (nodes[nodeNum]) {
        nodes[nodeNum].temperature = temperature;
        nodes[nodeNum].relativeHumidity = relativeHumidity;
        nodes[nodeNum].barometricPressure = barometricPressure;
        nodes[nodeNum].iaq = iaq;
    }

    // Update selector if this is first telemetry from this node
    if (isNewNode) {
        updateTelemetryNodeSelector();
    }

    if (selectedTelemetryNode === nodeNum) {
        renderTelemetryDashboard(nodeNum);
    }
}

// Handle power telemetry
function handlePowerTelemetry(data) {
    const { nodeNum } = data;

    const isNewNode = !telemetryData[nodeNum];
    if (!telemetryData[nodeNum]) telemetryData[nodeNum] = {};
    telemetryData[nodeNum].power = { ...data, timestamp: Date.now() };

    if (isNewNode) updateTelemetryNodeSelector();

    if (selectedTelemetryNode === nodeNum) {
        renderTelemetryDashboard(nodeNum);
    }
}

// Handle air quality telemetry
function handleAirQualityTelemetry(data) {
    const { nodeNum } = data;

    const isNewNode = !telemetryData[nodeNum];
    if (!telemetryData[nodeNum]) telemetryData[nodeNum] = {};
    telemetryData[nodeNum].airQuality = { ...data, timestamp: Date.now() };

    if (isNewNode) updateTelemetryNodeSelector();

    if (selectedTelemetryNode === nodeNum) {
        renderTelemetryDashboard(nodeNum);
    }
}

// Handle local stats telemetry
function handleLocalStatsTelemetry(data) {
    const { nodeNum, uptimeSeconds, channelUtilization, airUtilTx } = data;

    const isNewNode = !telemetryData[nodeNum];
    if (!telemetryData[nodeNum]) telemetryData[nodeNum] = {};
    telemetryData[nodeNum].localStats = { ...data, timestamp: Date.now() };

    // Update device metrics too if we have local stats
    if (!telemetryData[nodeNum].device) telemetryData[nodeNum].device = {};
    if (uptimeSeconds) telemetryData[nodeNum].device.uptimeSeconds = uptimeSeconds;
    if (channelUtilization) telemetryData[nodeNum].device.channelUtilization = channelUtilization;
    if (airUtilTx) telemetryData[nodeNum].device.airUtilTx = airUtilTx;

    if (isNewNode) updateTelemetryNodeSelector();

    if (selectedTelemetryNode === nodeNum) {
        renderTelemetryDashboard(nodeNum);
    }
}

// Render the telemetry dashboard for a node
function renderTelemetryDashboard(nodeNum) {
    const data = telemetryData[nodeNum] || {};
    const device = data.device || {};
    const env = data.environment || {};

    // Device metrics
    updateTelemetryMetric('Battery', device.batteryLevel, '%', device.batteryLevel);
    updateTelemetryMetric('Voltage', device.voltage, ' V', null, v => v?.toFixed(2));
    updateTelemetryMetric('ChannelUtil', device.channelUtilization, '%', device.channelUtilization);
    updateTelemetryMetric('AirUtil', device.airUtilTx, '%', device.airUtilTx);
    updateTelemetryMetric('Uptime', device.uptimeSeconds, '', null, formatUptimeSeconds);

    // Environment metrics
    updateTelemetryMetric('Temperature', env.temperature, '°C', null, v => v?.toFixed(1));
    updateTelemetryMetric('Humidity', env.relativeHumidity, '%', null, v => v?.toFixed(0));
    updateTelemetryMetric('Pressure', env.barometricPressure, ' hPa', null, v => v?.toFixed(0));
    updateTelemetryMetric('IAQ', env.iaq, '', null);

    // Update battery bar color
    const batteryBar = document.getElementById('barBattery');
    if (batteryBar && device.batteryLevel !== undefined) {
        batteryBar.classList.remove('warning', 'danger');
        if (device.batteryLevel <= 20) {
            batteryBar.classList.add('danger');
        } else if (device.batteryLevel <= 40) {
            batteryBar.classList.add('warning');
        }
    }

    // Update last update time
    const lastUpdate = document.getElementById('telemetryLastUpdate');
    if (lastUpdate) {
        const timestamps = [device.timestamp, env.timestamp].filter(Boolean);
        if (timestamps.length > 0) {
            const latest = Math.max(...timestamps);
            lastUpdate.textContent = `Last updated: ${formatTime(latest / 1000)}`;
        } else {
            lastUpdate.textContent = 'No recent data';
        }
    }
}

// Update a single telemetry metric display
function updateTelemetryMetric(name, value, unit, barValue, formatter) {
    const valueEl = document.getElementById(`value${name}`);
    const barEl = document.getElementById(`bar${name}`);

    if (valueEl) {
        if (value !== undefined && value !== null) {
            const displayValue = formatter ? formatter(value) : value;
            valueEl.textContent = `${displayValue}${unit}`;
        } else {
            valueEl.textContent = '--';
        }
    }

    if (barEl && barValue !== undefined && barValue !== null) {
        barEl.style.width = `${Math.min(100, Math.max(0, barValue))}%`;
    }
}

// Format uptime seconds to human readable
function formatUptimeSeconds(seconds) {
    if (!seconds) return '--';

    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) {
        return `${days}d ${hours}h`;
    } else if (hours > 0) {
        return `${hours}h ${minutes}m`;
    } else {
        return `${minutes}m`;
    }
}

// Clear telemetry display
function clearTelemetryDisplay() {
    ['Battery', 'Voltage', 'ChannelUtil', 'AirUtil', 'Uptime',
     'Temperature', 'Humidity', 'Pressure', 'IAQ'].forEach(name => {
        updateTelemetryMetric(name, null, '', null);
    });

    const lastUpdate = document.getElementById('telemetryLastUpdate');
    if (lastUpdate) {
        lastUpdate.textContent = 'Select a node to view telemetry';
    }
}

// Initialize telemetry dashboard on load
document.addEventListener('DOMContentLoaded', initTelemetryDashboard);

// Hook to update telemetry node selector when nodes are loaded
const _originalLoadNodes = typeof loadNodes === 'function' ? loadNodes : null;
if (_originalLoadNodes) {
    loadNodes = async function() {
        await _originalLoadNodes.apply(this, arguments);
        updateTelemetryNodeSelector();
        renderNeighborLists();
        renderTopologyGraph();
    };
}

// ==========================================
// POSITION UPDATES
// ==========================================

// Handle position update WebSocket event (e.g., from MQTT)
function handlePositionUpdated(data) {
    const nodeNum = data.nodeNum || data.from;
    if (!nodeNum) {
        console.warn('Invalid position update data:', data);
        return;
    }

    // Update or create node with new position
    let node = nodes[nodeNum];
    if (!node) {
        node = { num: nodeNum };
        nodes[nodeNum] = node;
    }

    if (data.latitude !== undefined) node.latitude = data.latitude;
    if (data.longitude !== undefined) node.longitude = data.longitude;
    if (data.altitude !== undefined) node.altitude = data.altitude;
    if (data.viaMqtt !== undefined) node.viaMqtt = data.viaMqtt;
    node.lastHeard = Date.now() / 1000;

    console.debug(`Position update for ${nodeNum}: lat=${node.latitude}, lon=${node.longitude}`);

    renderNodeList();
    updateMapMarker(node);
}

// ==========================================
// NEIGHBOR INFO PANEL
// ==========================================

// Store neighbor info data: { nodeNum: { neighbors: [...], lastUpdate: timestamp } }
let neighborInfoData = {};

// Handle neighbor info WebSocket event
function handleNeighborInfoUpdated(data) {
    // Backend sends: { from, nodeId, neighbors: [{ nodeId, snr, lastRxTime }] }
    const nodeNum = data.nodeId || data.from;
    if (!data || !nodeNum) {
        console.warn('Invalid neighbor info data:', data);
        return;
    }

    console.log('Neighbor info updated:', data);

    const neighbors = data.neighbors || [];

    neighborInfoData[nodeNum] = {
        neighbors: neighbors,
        lastUpdate: Date.now()
    };

    // Sync with nodes store so updateNeighborLines() works
    if (nodes[nodeNum]) {
        nodes[nodeNum].neighbors = neighbors;
    }

    renderNeighborLists();
    renderTopologyGraph();
    updateNeighborLines();
}

// Render neighbor lists for all nodes with neighbor info
function renderNeighborLists() {
    const container = document.getElementById('neighborLists');
    if (!container) return;

    const nodeNums = Object.keys(neighborInfoData);

    if (nodeNums.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                </svg>
                <p>Waiting for neighbor info...</p>
                <small>Neighbor data is received periodically from nodes</small>
            </div>
        `;
        return;
    }

    container.innerHTML = nodeNums.map(nodeNum => {
        const info = neighborInfoData[nodeNum];
        const node = nodes[nodeNum];
        const shortName = node?.shortName || '';
        const longName = node?.longName || '';
        const nodeId = `!${parseInt(nodeNum).toString(16)}`;
        const displayName = longName || shortName || nodeId;
        const avatarText = shortName || longName?.substring(0, 4) || nodeId.substring(1, 5);
        const neighbors = info.neighbors || [];

        return `
            <div class="neighbor-card">
                <div class="neighbor-card-header">
                    <div class="neighbor-card-avatar" style="background: ${getNodeColor(nodeNum)}">
                        ${avatarText.substring(0, 4).toUpperCase()}
                    </div>
                    <div class="neighbor-card-info">
                        <div class="neighbor-card-name">${escapeHtml(displayName)}${shortName && longName ? ` (${escapeHtml(shortName)})` : ''}</div>
                        <div class="neighbor-card-id">${nodeId}</div>
                    </div>
                    <div class="neighbor-card-count">${neighbors.length} neighbors</div>
                </div>
                <ul class="neighbor-list">
                    ${neighbors.length === 0 ? `
                        <li class="neighbor-item" style="justify-content: center; color: var(--on-surface-variant);">
                            No direct neighbors reported
                        </li>
                    ` : neighbors.map(neighbor => {
                        const neighborNode = nodes[neighbor.nodeId];
                        const nShortName = neighborNode?.shortName || '';
                        const nLongName = neighborNode?.longName || '';
                        const nId = `!${neighbor.nodeId.toString(16)}`;
                        const nDisplayName = nLongName || nShortName || nId;
                        const nAvatarText = nShortName || nLongName?.substring(0, 4) || nId.substring(1, 5);
                        const snr = neighbor.snr !== undefined ? neighbor.snr : '--';
                        const snrClass = getSnrClass(neighbor.snr);

                        return `
                            <li class="neighbor-item">
                                <div class="neighbor-item-info">
                                    <div class="neighbor-item-avatar" style="background: ${getNodeColor(neighbor.nodeId)}">
                                        ${nAvatarText.substring(0, 4).toUpperCase()}
                                    </div>
                                    <span class="neighbor-item-name">${escapeHtml(nDisplayName)}${nShortName && nLongName ? ` (${escapeHtml(nShortName)})` : ''}</span>
                                </div>
                                <div class="neighbor-item-snr">
                                    <div class="snr-bar">
                                        <div class="snr-bar-fill ${snrClass}" style="width: ${getSnrBarWidth(neighbor.snr)}%"></div>
                                    </div>
                                    <span class="snr-value">${snr} dB</span>
                                </div>
                            </li>
                        `;
                    }).join('')}
                </ul>
            </div>
        `;
    }).join('');
}

// Get CSS class for SNR value
function getSnrClass(snr) {
    if (snr === undefined || snr === null) return '';
    if (snr >= 5) return 'excellent';
    if (snr >= 0) return 'good';
    if (snr >= -5) return 'fair';
    return 'poor';
}

// Get bar width percentage for SNR value (range -20 to +10)
function getSnrBarWidth(snr) {
    if (snr === undefined || snr === null) return 0;
    // Map -20 to +10 dB to 0-100%
    const normalized = Math.max(0, Math.min(100, ((snr + 20) / 30) * 100));
    return normalized;
}

// Generate a consistent color for a node
function getNodeColor(nodeNum) {
    // Generate a hue based on node number
    const hue = (parseInt(nodeNum) * 137) % 360;
    return `hsl(${hue}, 65%, 45%)`;
}

// Render the topology graph showing node connections
function renderTopologyGraph() {
    const svg = document.getElementById('topologyGraph');
    if (!svg) return;

    const nodeNums = Object.keys(neighborInfoData);

    if (nodeNums.length === 0) {
        svg.innerHTML = `
            <text x="50%" y="50%" text-anchor="middle" fill="var(--on-surface-variant)" font-size="14">
                No topology data available
            </text>
        `;
        return;
    }

    // Collect all nodes involved in neighbor relationships
    const involvedNodes = new Set();
    const links = [];

    nodeNums.forEach(nodeNum => {
        involvedNodes.add(parseInt(nodeNum));
        const neighbors = neighborInfoData[nodeNum].neighbors || [];
        neighbors.forEach(neighbor => {
            involvedNodes.add(neighbor.nodeId);
            links.push({
                source: parseInt(nodeNum),
                target: neighbor.nodeId,
                snr: neighbor.snr
            });
        });
    });

    const nodeArray = Array.from(involvedNodes);
    const nodeCount = nodeArray.length;

    if (nodeCount === 0) {
        svg.innerHTML = `
            <text x="50%" y="50%" text-anchor="middle" fill="var(--on-surface-variant)" font-size="14">
                No nodes in topology
            </text>
        `;
        return;
    }

    // Calculate positions in a circle
    const width = svg.clientWidth || 400;
    const height = svg.clientHeight || 250;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.min(width, height) / 2 - 50;

    const positions = {};
    nodeArray.forEach((nodeNum, i) => {
        const angle = (i / nodeCount) * 2 * Math.PI - Math.PI / 2;
        positions[nodeNum] = {
            x: centerX + radius * Math.cos(angle),
            y: centerY + radius * Math.sin(angle)
        };
    });

    // Build SVG content
    let svgContent = '';

    // Draw links first (behind nodes)
    links.forEach(link => {
        const source = positions[link.source];
        const target = positions[link.target];
        if (!source || !target) return;

        const snrClass = getSnrClass(link.snr);
        let strokeColor = 'var(--outline)';
        if (snrClass === 'excellent') strokeColor = 'var(--status-green)';
        else if (snrClass === 'good') strokeColor = 'var(--primary)';
        else if (snrClass === 'fair') strokeColor = 'var(--status-yellow)';
        else if (snrClass === 'poor') strokeColor = 'var(--status-red)';

        svgContent += `
            <line
                x1="${source.x}" y1="${source.y}"
                x2="${target.x}" y2="${target.y}"
                class="topology-link ${snrClass}"
                stroke="${strokeColor}"
                stroke-width="2"
            />
        `;
    });

    // Draw nodes
    nodeArray.forEach(nodeNum => {
        const pos = positions[nodeNum];
        const node = nodes[nodeNum];
        const shortName = node?.shortName || '';
        const longName = node?.longName || '';
        const nodeId = `!${nodeNum.toString(16)}`;
        const avatarText = shortName || longName?.substring(0, 4) || nodeId.substring(1, 5);
        const displayName = longName || shortName || nodeId;
        const truncatedName = displayName.length > 12 ? displayName.substring(0, 12) + '…' : displayName;
        const color = getNodeColor(nodeNum);
        const hasNeighborInfo = neighborInfoData[nodeNum] !== undefined;

        svgContent += `
            <g class="topology-node" transform="translate(${pos.x}, ${pos.y})">
                <circle r="20" fill="${color}" stroke="${hasNeighborInfo ? 'var(--primary)' : 'var(--outline)'}" stroke-width="${hasNeighborInfo ? 3 : 1}"/>
                <text y="5" text-anchor="middle" fill="white" font-size="10" font-weight="600">
                    ${avatarText.substring(0, 4).toUpperCase()}
                </text>
                <text y="38" text-anchor="middle" fill="var(--on-surface)" font-size="10">
                    ${escapeHtml(truncatedName)}
                </text>
            </g>
        `;
    });

    svg.innerHTML = svgContent;
}

// ==========================================
// GPS UTILITIES
// ==========================================

const EARTH_RADIUS = 6371000; // meters
const DEG_TO_RAD = Math.PI / 180;
const RAD_TO_DEG = 180 / Math.PI;

// Calculate distance between two coordinates using Haversine formula
function calculateDistance(lat1, lon1, lat2, lon2) {
    if (!lat1 || !lon1 || !lat2 || !lon2) return 0;

    const dLat = (lat2 - lat1) * DEG_TO_RAD;
    const dLon = (lon2 - lon1) * DEG_TO_RAD;

    const a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
              Math.cos(lat1 * DEG_TO_RAD) * Math.cos(lat2 * DEG_TO_RAD) *
              Math.sin(dLon / 2) * Math.sin(dLon / 2);

    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

    return EARTH_RADIUS * c;
}

// Calculate bearing from point 1 to point 2
function calculateBearing(lat1, lon1, lat2, lon2) {
    if (!lat1 || !lon1 || !lat2 || !lon2) return 0;

    const dLon = (lon2 - lon1) * DEG_TO_RAD;
    const lat1Rad = lat1 * DEG_TO_RAD;
    const lat2Rad = lat2 * DEG_TO_RAD;

    const y = Math.sin(dLon) * Math.cos(lat2Rad);
    const x = Math.cos(lat1Rad) * Math.sin(lat2Rad) -
              Math.sin(lat1Rad) * Math.cos(lat2Rad) * Math.cos(dLon);

    let bearing = Math.atan2(y, x) * RAD_TO_DEG;
    return (bearing + 360) % 360; // Normalize to 0-360
}

// Format distance for display
function formatDistance(meters) {
    if (meters === undefined || meters === null || meters === 0) return '--';
    if (meters < 1000) {
        return `${Math.round(meters)} m`;
    }
    return `${(meters / 1000).toFixed(1)} km`;
}

// Format distance in imperial units
function formatDistanceImperial(meters) {
    if (meters === undefined || meters === null || meters === 0) return '--';
    const feet = meters * 3.28084;
    if (feet < 5280) {
        return `${Math.round(feet)} ft`;
    }
    return `${(feet / 5280).toFixed(1)} mi`;
}

// Convert bearing to 8-point cardinal direction
function bearingToCardinal(bearing) {
    const directions = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
    const index = Math.round(bearing / 45) % 8;
    return directions[index];
}

// Convert bearing to 16-point cardinal direction
function bearingToCardinal16(bearing) {
    const directions = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE',
                       'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW'];
    const index = Math.round(bearing / 22.5) % 16;
    return directions[index];
}

// Format coordinates as decimal degrees
function formatCoordDecimal(lat, lon) {
    if (!lat && !lon) return '--';
    return `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
}

// Format coordinates as DMS (Degrees Minutes Seconds)
function formatCoordDMS(lat, lon) {
    if (!lat && !lon) return '--';

    function toDMS(coord, isLat) {
        const dir = isLat ? (coord >= 0 ? 'N' : 'S') : (coord >= 0 ? 'E' : 'W');
        coord = Math.abs(coord);
        const deg = Math.floor(coord);
        const min = Math.floor((coord - deg) * 60);
        const sec = ((coord - deg - min / 60) * 3600).toFixed(1);
        return `${deg}°${min}'${sec}"${dir}`;
    }

    return `${toDMS(lat, true)} ${toDMS(lon, false)}`;
}

// Format coordinates as Degrees Decimal Minutes
function formatCoordDM(lat, lon) {
    if (!lat && !lon) return '--';

    function toDM(coord, isLat) {
        const dir = isLat ? (coord >= 0 ? 'N' : 'S') : (coord >= 0 ? 'E' : 'W');
        coord = Math.abs(coord);
        const deg = Math.floor(coord);
        const min = ((coord - deg) * 60).toFixed(4);
        return `${deg}°${min}'${dir}`;
    }

    return `${toDM(lat, true)} ${toDM(lon, false)}`;
}

// Get position info string for a node relative to local node
function getNodePositionInfo(node) {
    if (!node || (!node.latitude && !node.longitude)) {
        return { hasPosition: false };
    }

    const info = {
        hasPosition: true,
        lat: node.latitude,
        lon: node.longitude,
        altitude: node.altitude,
        coordDecimal: formatCoordDecimal(node.latitude, node.longitude),
        coordDMS: formatCoordDMS(node.latitude, node.longitude)
    };

    // Use pre-calculated distance/bearing from server if available
    if (node.distance !== undefined && node.distance > 0) {
        info.distance = node.distance;
        info.distanceStr = node.distanceStr || formatDistance(node.distance);
        info.bearing = node.bearing;
        info.bearingCardinal = node.bearingCardinal || bearingToCardinal(node.bearing);
    } else if (myNode && myNode.latitude && myNode.longitude) {
        // Calculate client-side if not from server
        info.distance = calculateDistance(myNode.latitude, myNode.longitude, node.latitude, node.longitude);
        info.distanceStr = formatDistance(info.distance);
        info.bearing = calculateBearing(myNode.latitude, myNode.longitude, node.latitude, node.longitude);
        info.bearingCardinal = bearingToCardinal(info.bearing);
    }

    return info;
}

// Render position info HTML
function renderPositionInfo(node) {
    const info = getNodePositionInfo(node);
    if (!info.hasPosition) {
        return '<span class="no-position">No GPS data</span>';
    }

    let html = `<div class="position-info">`;
    html += `<div class="coord">${info.coordDecimal}</div>`;

    if (info.altitude) {
        html += `<div class="altitude">${info.altitude} m alt</div>`;
    }

    if (info.distance !== undefined && info.distance > 0) {
        html += `<div class="distance-bearing">`;
        html += `<span class="distance">${info.distanceStr}</span>`;
        html += `<span class="bearing">${info.bearingCardinal} (${Math.round(info.bearing)}°)</span>`;
        html += `</div>`;
    }

    html += `</div>`;
    return html;
}

// ============================================
// MOBILE NAVIGATION FUNCTIONS
// ============================================

function openMobileSidebar() {
    const sidebar = document.getElementById('mobileSidebar');
    const overlay = document.getElementById('mobileNavOverlay');
    if (sidebar && overlay) {
        overlay.style.display = 'block';
        sidebar.classList.add('show');
        requestAnimationFrame(() => {
            overlay.classList.add('show');
        });
        document.body.style.overflow = 'hidden';
    }
}

function closeMobileSidebar() {
    const sidebar = document.getElementById('mobileSidebar');
    const overlay = document.getElementById('mobileNavOverlay');
    if (sidebar && overlay) {
        sidebar.classList.remove('show');
        overlay.classList.remove('show');
        setTimeout(() => {
            overlay.style.display = 'none';
        }, 300);
        document.body.style.overflow = '';
    }
}

function selectMobileTab(tabName) {
    closeMobileSidebar();

    // Update mobile tab bar
    document.querySelectorAll('.mobile-tab').forEach(tab => {
        tab.classList.toggle('active', tab.dataset.tab === tabName);
    });

    // Update mobile sidebar nav items
    document.querySelectorAll('.mobile-nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.tab === tabName);
    });

    // Activate the desktop tab
    document.querySelectorAll('.tab').forEach(tab => {
        tab.classList.toggle('active', tab.dataset.tab === tabName);
    });

    // Show the corresponding panel
    document.querySelectorAll('.tab-panel').forEach(panel => {
        panel.classList.toggle('active', panel.id === tabName);
    });

    // Load data for specific tabs
    if (tabName === 'channels') {
        loadChannels();
    } else if (tabName === 'config') {
        loadConfig();
    } else if (tabName === 'map') {
        initMap();
    }
}

// Initialize mobile navigation toggle
document.getElementById('mobileNavToggle')?.addEventListener('click', openMobileSidebar);

// Handle swipe gestures for mobile sidebar
let touchStartX = 0;
let touchEndX = 0;

document.addEventListener('touchstart', (e) => {
    touchStartX = e.changedTouches[0].screenX;
}, { passive: true });

document.addEventListener('touchend', (e) => {
    touchEndX = e.changedTouches[0].screenX;
    handleSwipeGesture();
}, { passive: true });

function handleSwipeGesture() {
    const swipeThreshold = 50;
    const sidebar = document.getElementById('mobileSidebar');

    if (touchStartX < 30 && touchEndX - touchStartX > swipeThreshold) {
        // Swipe right from left edge - open sidebar
        openMobileSidebar();
    } else if (sidebar?.classList.contains('show') && touchStartX - touchEndX > swipeThreshold) {
        // Swipe left while sidebar is open - close sidebar
        closeMobileSidebar();
    }
}

// Update mobile sidebar with node list when nodes change
function updateMobileSidebarNodes() {
    const container = document.getElementById('mobileSidebarNodes');
    if (!container) return;

    const nodeArr = Object.values(nodes).slice(0, 10);
    if (nodeArr.length === 0) {
        container.innerHTML = '<div style="opacity: 0.6; text-align: center; padding: 1rem;">No nodes yet</div>';
        return;
    }

    container.innerHTML = nodeArr.map(node => {
        const isSelected = selectedNode && selectedNode.num === node.num;
        return `
            <div class="mobile-nav-item ${isSelected ? 'active' : ''}"
                 style="padding: 0.625rem 1rem;"
                 onclick="selectNodeFromMobile(${node.num})">
                <span style="font-size: 1rem;">${node.shortName || '??'}</span>
                <span style="flex: 1; font-size: 0.8125rem;">${escapeHtml(node.longName || 'Unknown')}</span>
            </div>
        `;
    }).join('');
}

function selectNodeFromMobile(nodeNum) {
    selectNode(nodeNum);
    closeMobileSidebar();
    selectMobileTab('messages');
}

// ============================================
// WAYPOINT FUNCTIONS
// ============================================

let waypoints = [];
let waypointMarkers = {};
let selectedWaypointIcon = '📍';
let editingWaypointId = null;

// Icon code mapping (Meshtastic uses uint32 for icons)
const waypointIcons = {
    0x1F4CD: '📍',  // round pushpin
    0x26FA: '⛺',   // tent
    0x1F3E0: '🏠', // house
    0x1F697: '🚗', // car
    0x26A0: '⚠️',  // warning
    0x1F527: '🔧', // wrench
    0x1F4A7: '💧', // droplet
    0x1F525: '🔥', // fire
    0x1F3D4: '🏔️', // mountain
    0x1F332: '🌲', // tree
    0x2B50: '⭐',  // star
    0x2764: '❤️',  // heart
};

const emojiToCode = {
    '📍': 0x1F4CD,
    '⛺': 0x26FA,
    '🏠': 0x1F3E0,
    '🚗': 0x1F697,
    '⚠️': 0x26A0,
    '🔧': 0x1F527,
    '💧': 0x1F4A7,
    '🔥': 0x1F525,
    '🏔️': 0x1F3D4,
    '🌲': 0x1F332,
    '⭐': 0x2B50,
    '❤️': 0x2764,
};

function toggleWaypointPanel() {
    const panel = document.getElementById('waypointPanel');
    const icon = document.getElementById('waypointToggleIcon');
    if (panel) {
        panel.classList.toggle('collapsed');
        // Rotate icon when expanded
        if (icon) {
            icon.style.transform = panel.classList.contains('collapsed') ? '' : 'rotate(180deg)';
        }
    }
}

function hideWaypointPanel() {
    const panel = document.getElementById('waypointPanel');
    if (panel) {
        panel.classList.add('hidden');
    }
}

function showWaypointPanel() {
    const panel = document.getElementById('waypointPanel');
    if (panel) {
        panel.classList.remove('hidden');
    }
}

function loadWaypoints() {
    api('GET', '/waypoints').then(data => {
        waypoints = data.waypoints || [];
        renderWaypointList();
        renderWaypointMarkers();
    }).catch(err => {
        console.error('Failed to load waypoints:', err);
    });
}

function renderWaypointList() {
    const list = document.getElementById('waypointList');
    if (!list) return;

    if (waypoints.length === 0) {
        list.innerHTML = '<div class="waypoint-empty">No waypoints yet</div>';
        return;
    }

    list.innerHTML = waypoints.map(wp => {
        const icon = waypointIcons[wp.icon] || '📍';
        const lat = wp.latitude?.toFixed(5) || '0';
        const lon = wp.longitude?.toFixed(5) || '0';

        return `
            <div class="waypoint-item" onclick="focusWaypoint(${wp.id})">
                <div class="waypoint-icon">${icon}</div>
                <div class="waypoint-info">
                    <div class="waypoint-name">${escapeHtml(wp.name)}</div>
                    <div class="waypoint-coords">${lat}, ${lon}</div>
                </div>
                <div class="waypoint-actions">
                    <button class="waypoint-action-btn" onclick="event.stopPropagation(); editWaypoint(${wp.id})" title="Edit">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"/>
                        </svg>
                    </button>
                    <button class="waypoint-action-btn" onclick="event.stopPropagation(); deleteWaypoint(${wp.id})" title="Delete">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
                        </svg>
                    </button>
                </div>
            </div>
        `;
    }).join('');
}

function renderWaypointMarkers() {
    if (!leafletMap) return;

    // Clear existing waypoint markers
    Object.values(waypointMarkers).forEach(marker => {
        leafletMap.removeLayer(marker);
    });
    waypointMarkers = {};

    // Add markers for each waypoint
    waypoints.forEach(wp => {
        if (!wp.latitude || !wp.longitude) return;

        const icon = waypointIcons[wp.icon] || '📍';
        const markerIcon = L.divIcon({
            className: 'waypoint-marker-container',
            html: `<div class="waypoint-marker"><span class="waypoint-marker-icon">${icon}</span></div>`,
            iconSize: [32, 32],
            iconAnchor: [8, 32],
            popupAnchor: [8, -32]
        });

        const marker = L.marker([wp.latitude, wp.longitude], { icon: markerIcon });

        // Create popup content
        const popupContent = `
            <div style="min-width: 150px;">
                <strong>${escapeHtml(wp.name)}</strong>
                ${wp.description ? `<p style="margin: 0.5rem 0; font-size: 0.875rem;">${escapeHtml(wp.description)}</p>` : ''}
                <div style="font-size: 0.75rem; opacity: 0.7;">
                    ${wp.latitude.toFixed(6)}, ${wp.longitude.toFixed(6)}
                </div>
                ${wp.from ? `<div style="font-size: 0.75rem; margin-top: 0.25rem;">From: !${(wp.from >>> 0).toString(16)}</div>` : ''}
            </div>
        `;
        marker.bindPopup(popupContent);

        marker.addTo(leafletMap);
        waypointMarkers[wp.id] = marker;
    });
}

function focusWaypoint(id) {
    const wp = waypoints.find(w => w.id === id);
    if (wp && wp.latitude && wp.longitude && leafletMap) {
        leafletMap.setView([wp.latitude, wp.longitude], 15);
        const marker = waypointMarkers[id];
        if (marker) {
            marker.openPopup();
        }
    }
}

function openWaypointModal(lat, lon) {
    const modal = document.getElementById('waypointModal');
    const title = document.getElementById('waypointModalTitle');

    // Reset form
    document.getElementById('waypointName').value = '';
    document.getElementById('waypointDescription').value = '';
    document.getElementById('waypointLat').value = lat || '';
    document.getElementById('waypointLon').value = lon || '';
    document.getElementById('waypointBroadcast').checked = true;

    // Reset icon selection
    selectedWaypointIcon = '📍';
    document.querySelectorAll('.waypoint-icon-option').forEach(opt => {
        opt.classList.toggle('selected', opt.dataset.icon === '📍');
    });

    editingWaypointId = null;
    title.textContent = 'New Waypoint';

    modal.classList.remove('hidden');
}

function closeWaypointModal() {
    const modal = document.getElementById('waypointModal');
    modal.classList.add('hidden');
    editingWaypointId = null;
}

function editWaypoint(id) {
    const wp = waypoints.find(w => w.id === id);
    if (!wp) return;

    const modal = document.getElementById('waypointModal');
    const title = document.getElementById('waypointModalTitle');

    document.getElementById('waypointName').value = wp.name || '';
    document.getElementById('waypointDescription').value = wp.description || '';
    document.getElementById('waypointLat').value = wp.latitude || '';
    document.getElementById('waypointLon').value = wp.longitude || '';
    document.getElementById('waypointBroadcast').checked = false;

    // Set icon
    const icon = waypointIcons[wp.icon] || '📍';
    selectedWaypointIcon = icon;
    document.querySelectorAll('.waypoint-icon-option').forEach(opt => {
        opt.classList.toggle('selected', opt.dataset.icon === icon);
    });

    editingWaypointId = id;
    title.textContent = 'Edit Waypoint';

    modal.classList.remove('hidden');
}

async function saveWaypoint() {
    const name = document.getElementById('waypointName').value.trim();
    const description = document.getElementById('waypointDescription').value.trim();
    const lat = parseFloat(document.getElementById('waypointLat').value);
    const lon = parseFloat(document.getElementById('waypointLon').value);
    const broadcast = document.getElementById('waypointBroadcast').checked;

    if (!name) {
        showToast('Please enter a waypoint name', 'error');
        return;
    }

    if (isNaN(lat) || isNaN(lon)) {
        showToast('Please enter valid coordinates', 'error');
        return;
    }

    if (lat < -90 || lat > 90) {
        showToast('Latitude must be between -90 and 90', 'error');
        return;
    }

    if (lon < -180 || lon > 180) {
        showToast('Longitude must be between -180 and 180', 'error');
        return;
    }

    const waypointData = {
        id: editingWaypointId || 0,
        name: name,
        description: description,
        latitude: lat,
        longitude: lon,
        icon: emojiToCode[selectedWaypointIcon] || 0x1F4CD,
        broadcast: broadcast
    };

    try {
        const result = await api('POST', '/waypoints', waypointData);
        closeWaypointModal();
        loadWaypoints();
        showToast(editingWaypointId ? 'Waypoint updated' : 'Waypoint created', 'success');

        if (result.warning) {
            showToast(result.warning, 'warning');
        }
    } catch (err) {
        showToast('Failed to save waypoint: ' + err.message, 'error');
    }
}

async function deleteWaypoint(id) {
    if (!confirm('Delete this waypoint?')) return;

    try {
        await api('DELETE', `/waypoints/${id}`);
        loadWaypoints();
        showToast('Waypoint deleted', 'success');
    } catch (err) {
        showToast('Failed to delete waypoint: ' + err.message, 'error');
    }
}

// Initialize waypoint icon picker
document.querySelectorAll('.waypoint-icon-option').forEach(opt => {
    opt.addEventListener('click', () => {
        document.querySelectorAll('.waypoint-icon-option').forEach(o => o.classList.remove('selected'));
        opt.classList.add('selected');
        selectedWaypointIcon = opt.dataset.icon;
    });
});

// Handle waypoint events from WebSocket
function handleWaypointEvent(event) {
    if (event.type === 'waypoint.received' || event.type === 'waypoint.created') {
        // Add or update waypoint in list
        const existingIdx = waypoints.findIndex(w => w.id === event.data.id);
        if (existingIdx >= 0) {
            waypoints[existingIdx] = event.data;
        } else {
            waypoints.push(event.data);
        }
        renderWaypointList();
        renderWaypointMarkers();
    } else if (event.type === 'waypoint.deleted') {
        waypoints = waypoints.filter(w => w.id !== event.data.id);
        renderWaypointList();
        renderWaypointMarkers();
    }
}

// Load waypoints when map is initialized
const originalInitMap = initMap;
initMap = function() {
    originalInitMap();
    loadWaypoints();

    // Allow creating waypoints by clicking on map
    if (leafletMap) {
        leafletMap.on('contextmenu', function(e) {
            openWaypointModal(e.latlng.lat.toFixed(6), e.latlng.lng.toFixed(6));
        });
    }
};

// ============================================
// Historical Charts - Telemetry and Neighbors
// ============================================

let telemetryChart = null;
let neighborChart = null;
let selectedTelemetryChartPeriod = 'day';
let selectedNeighborChartPeriod = 'day';

// Initialize chart controls
document.addEventListener('DOMContentLoaded', () => {
    initTelemetryChartControls();
    initNeighborChartControls();
});

// Initialize telemetry chart controls
function initTelemetryChartControls() {
    const metricSelect = document.getElementById('telemetryChartMetric');
    const periodButtons = document.querySelectorAll('#telemetryChartsSection .period-btn');

    if (metricSelect) {
        metricSelect.addEventListener('change', () => {
            loadTelemetryChart();
        });
    }

    periodButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            periodButtons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            selectedTelemetryChartPeriod = btn.dataset.period;
            loadTelemetryChart();
        });
    });
}

// Initialize neighbor chart controls
function initNeighborChartControls() {
    const nodeSelect = document.getElementById('neighborChartNode');
    const periodButtons = document.querySelectorAll('.neighbor-period');

    if (nodeSelect) {
        nodeSelect.addEventListener('change', () => {
            loadNeighborChart();
        });
    }

    periodButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            periodButtons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            selectedNeighborChartPeriod = btn.dataset.period;
            loadNeighborChart();
        });
    });

    // Load available nodes
    loadNeighborChartNodes();
}

// Load nodes with telemetry data into the chart selector (if needed)
async function loadTelemetryChartNodes() {
    try {
        const data = await api('GET', '/telemetry/nodes');
        // Nodes are already managed by the telemetry node selector
        // This function can be used to refresh available nodes for charting
        console.log('Telemetry nodes:', data.nodes);
    } catch (e) {
        console.error('Failed to load telemetry chart nodes:', e);
    }
}

// Load nodes with neighbor data into the selector
async function loadNeighborChartNodes() {
    const select = document.getElementById('neighborChartNode');
    if (!select) return;

    const currentValue = select.value;

    // Get nodes from in-memory neighbor data
    const inMemoryNodes = new Set(Object.keys(neighborInfoData).map(n => parseInt(n)));

    // Also fetch nodes with historical data from database
    let dbNodes = [];
    try {
        const data = await api('GET', '/neighbors/nodes');
        if (data.nodes) {
            dbNodes = data.nodes;
        }
    } catch (e) {
        console.warn('Failed to load neighbor nodes from database:', e);
    }

    // Merge both sources
    const allNodes = new Set([...inMemoryNodes, ...dbNodes]);
    const nodeList = Array.from(allNodes).sort((a, b) => {
        const nodeA = nodes[a];
        const nodeB = nodes[b];
        const nameA = nodeA?.shortName || nodeA?.longName || '';
        const nameB = nodeB?.shortName || nodeB?.longName || '';
        return nameA.localeCompare(nameB);
    });

    // Keep the default option
    select.innerHTML = '<option value="">Select a node...</option>';

    if (nodeList.length === 0) {
        const option = document.createElement('option');
        option.disabled = true;
        option.textContent = 'No neighbor data available';
        select.appendChild(option);
        return;
    }

    nodeList.forEach(nodeNum => {
        const node = nodes[nodeNum];
        const displayName = getNodeDisplayName(nodeNum, node);
        const hasLiveData = inMemoryNodes.has(nodeNum);
        const option = document.createElement('option');
        option.value = nodeNum;
        option.textContent = `${displayName}${hasLiveData ? '' : ' [historical]'}`;
        select.appendChild(option);
    });

    // Restore selection if still valid
    if (currentValue && allNodes.has(parseInt(currentValue))) {
        select.value = currentValue;
    }
}

// Helper to get node display name
function getNodeDisplayName(nodeNum, node) {
    const nodeId = `!${(parseInt(nodeNum) >>> 0).toString(16)}`;
    if (node) {
        if (node.longName) return `${node.longName} (${nodeId})`;
        if (node.shortName) return `${node.shortName} (${nodeId})`;
    }
    return nodeId;
}

// Load and render telemetry chart
async function loadTelemetryChart() {
    const nodeNum = selectedTelemetryNode;
    const metric = document.getElementById('telemetryChartMetric')?.value || 'battery_level';
    const period = selectedTelemetryChartPeriod;

    const chartEmpty = document.getElementById('telemetryChartEmpty');
    const chartContainer = document.querySelector('#telemetryChartsSection .chart-container');

    if (!nodeNum) {
        if (chartEmpty) chartEmpty.style.display = 'flex';
        if (chartContainer) chartContainer.style.display = 'none';
        return;
    }

    try {
        const data = await api('GET', `/telemetry/${nodeNum}/aggregated?metric=${metric}&period=${period}`);

        if (!data.data || data.data.length === 0) {
            if (chartEmpty) {
                chartEmpty.innerHTML = '<p>No historical data available for this metric</p>';
                chartEmpty.style.display = 'flex';
            }
            if (chartContainer) chartContainer.style.display = 'none';
            return;
        }

        if (chartEmpty) chartEmpty.style.display = 'none';
        if (chartContainer) chartContainer.style.display = 'block';

        renderTelemetryChart(data.data, metric, period);
    } catch (e) {
        console.error('Failed to load telemetry chart:', e);
        if (chartEmpty) {
            chartEmpty.innerHTML = '<p>Failed to load chart data</p>';
            chartEmpty.style.display = 'flex';
        }
    }
}

// Render telemetry chart using Chart.js
function renderTelemetryChart(data, metric, period) {
    const canvas = document.getElementById('telemetryChart');
    if (!canvas) return;

    // Destroy existing chart
    if (telemetryChart) {
        telemetryChart.destroy();
    }

    const labels = data.map(point => formatChartLabel(point.timestamp, period));
    const values = data.map(point => point.avg);
    const minValues = data.map(point => point.min);
    const maxValues = data.map(point => point.max);

    const metricLabels = {
        battery_level: 'Battery Level (%)',
        voltage: 'Voltage (V)',
        channel_utilization: 'Channel Utilization (%)',
        air_util_tx: 'Air Time TX (%)',
        temperature: 'Temperature (°C)',
        relative_humidity: 'Humidity (%)',
        barometric_pressure: 'Pressure (hPa)'
    };

    const ctx = canvas.getContext('2d');
    telemetryChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Average',
                    data: values,
                    borderColor: '#306A42',
                    backgroundColor: 'rgba(48, 106, 66, 0.15)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 3,
                    pointHoverRadius: 6,
                    borderWidth: 2.5
                },
                {
                    label: 'Min',
                    data: minValues,
                    borderColor: '#2196F3',
                    backgroundColor: 'rgba(33, 150, 243, 0.1)',
                    borderDash: [6, 3],
                    fill: false,
                    tension: 0.3,
                    pointRadius: 2,
                    pointHoverRadius: 4,
                    borderWidth: 1.5
                },
                {
                    label: 'Max',
                    data: maxValues,
                    borderColor: '#FF5722',
                    backgroundColor: 'rgba(255, 87, 34, 0.1)',
                    borderDash: [2, 2],
                    fill: false,
                    tension: 0.3,
                    pointRadius: 2,
                    pointHoverRadius: 4,
                    borderWidth: 1.5
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: metricLabels[metric] || metric
                },
                legend: {
                    position: 'bottom'
                }
            },
            scales: {
                x: {
                    display: true,
                    title: {
                        display: true,
                        text: 'Time'
                    }
                },
                y: {
                    display: true,
                    title: {
                        display: true,
                        text: metricLabels[metric] || metric
                    }
                }
            }
        }
    });
}

// Load and render neighbor chart
async function loadNeighborChart() {
    const nodeNum = document.getElementById('neighborChartNode')?.value;
    const period = selectedNeighborChartPeriod;

    const chartEmpty = document.getElementById('neighborChartEmpty');
    const chartContainer = document.querySelector('.neighbor-chart-section .chart-container');

    if (!nodeNum) {
        if (chartEmpty) chartEmpty.style.display = 'flex';
        if (chartContainer) chartContainer.style.display = 'none';
        return;
    }

    try {
        const data = await api('GET', `/neighbors/${nodeNum}/chart?period=${period}`);

        if (!data.data || data.data.length === 0) {
            if (chartEmpty) {
                chartEmpty.innerHTML = '<p>No historical data available for this node</p>';
                chartEmpty.style.display = 'flex';
            }
            if (chartContainer) chartContainer.style.display = 'none';
            return;
        }

        if (chartEmpty) chartEmpty.style.display = 'none';
        if (chartContainer) chartContainer.style.display = 'block';

        renderNeighborChart(data.data, period);
    } catch (e) {
        console.error('Failed to load neighbor chart:', e);
        if (chartEmpty) {
            chartEmpty.innerHTML = '<p>Failed to load chart data</p>';
            chartEmpty.style.display = 'flex';
        }
    }
}

// Render neighbor chart using Chart.js
function renderNeighborChart(data, period) {
    const canvas = document.getElementById('neighborChart');
    if (!canvas) return;

    // Destroy existing chart
    if (neighborChart) {
        neighborChart.destroy();
    }

    const labels = data.map(point => formatChartLabel(point.timestamp, period));
    const avgCounts = data.map(point => point.avgCount);
    const maxCounts = data.map(point => point.maxCount);
    const avgSnr = data.map(point => point.avgSnr);

    const ctx = canvas.getContext('2d');
    neighborChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Avg Neighbors',
                    data: avgCounts,
                    backgroundColor: 'rgba(48, 106, 66, 0.8)',
                    borderColor: '#306A42',
                    borderWidth: 1,
                    borderRadius: 4,
                    yAxisID: 'y',
                    order: 2
                },
                {
                    label: 'Max Neighbors',
                    data: maxCounts,
                    backgroundColor: 'rgba(33, 150, 243, 0.5)',
                    borderColor: '#2196F3',
                    borderWidth: 1,
                    borderRadius: 4,
                    yAxisID: 'y',
                    order: 3
                },
                {
                    label: 'Avg SNR (dB)',
                    data: avgSnr,
                    type: 'line',
                    borderColor: '#FF5722',
                    backgroundColor: 'rgba(255, 87, 34, 0.1)',
                    fill: false,
                    tension: 0.3,
                    pointRadius: 4,
                    pointHoverRadius: 6,
                    borderWidth: 2.5,
                    yAxisID: 'y1',
                    order: 1
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: 'Neighbor Count & Signal Quality Over Time'
                },
                legend: {
                    position: 'bottom'
                }
            },
            scales: {
                x: {
                    display: true,
                    title: {
                        display: true,
                        text: 'Time'
                    }
                },
                y: {
                    type: 'linear',
                    display: true,
                    position: 'left',
                    title: {
                        display: true,
                        text: 'Neighbor Count'
                    },
                    min: 0
                },
                y1: {
                    type: 'linear',
                    display: true,
                    position: 'right',
                    title: {
                        display: true,
                        text: 'SNR (dB)'
                    },
                    grid: {
                        drawOnChartArea: false
                    }
                }
            }
        }
    });
}

// Format chart label based on period
function formatChartLabel(timestamp, period) {
    const date = new Date(timestamp * 1000);
    switch (period) {
        case 'day':
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        case 'month':
            return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        case 'year':
            return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        default:
            return date.toLocaleString();
    }
}

// Update telemetry chart when node is selected
const originalRenderTelemetryDashboard = typeof renderTelemetryDashboard !== 'undefined' ? renderTelemetryDashboard : null;
if (originalRenderTelemetryDashboard) {
    renderTelemetryDashboard = function(nodeNum) {
        originalRenderTelemetryDashboard(nodeNum);
        // Load chart when dashboard is rendered
        loadTelemetryChart();
    };
}

// Refresh neighbor nodes when new neighbor info is received
const originalHandleNeighborInfoUpdated = handleNeighborInfoUpdated;
handleNeighborInfoUpdated = function(data) {
    originalHandleNeighborInfoUpdated(data);
    // Refresh the node selector in case this is a new node
    loadNeighborChartNodes();
};
