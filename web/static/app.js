/**
 * Gthulhu - eBPF Scheduler Control Interface
 * Main Application JavaScript
 */

// ===========================================
// Configuration & State
// ===========================================

const state = {
    jwtToken: localStorage.getItem('jwtToken'),
    isAuthenticated: false,
    apiBaseUrl: localStorage.getItem('apiBaseUrl') || '',
    healthHistory: [],
    healthInterval: null,
    strategyCounter: 0,
    currentUser: null
};

state.isAuthenticated = !!state.jwtToken;

// ===========================================
// API Configuration
// ===========================================

function getApiUrl(endpoint) {
    // Always use relative URLs when apiBaseUrl is empty (same-origin requests via proxy)
    if (!state.apiBaseUrl || state.apiBaseUrl === '') {
        return endpoint;
    }
    const base = state.apiBaseUrl.replace(/\/$/, '');
    return base + endpoint;
}

function showConfigModal() {
    const modal = document.getElementById('configModal');
    const input = document.getElementById('apiBaseUrl');
    input.value = state.apiBaseUrl;
    modal.classList.add('active');
}

function hideConfigModal() {
    document.getElementById('configModal').classList.remove('active');
}

function saveApiConfig() {
    const input = document.getElementById('apiBaseUrl');
    state.apiBaseUrl = input.value.trim();
    localStorage.setItem('apiBaseUrl', state.apiBaseUrl);
    hideConfigModal();
    showToast('success', 'Configuration saved successfully');
}

// ===========================================
// Authentication
// ===========================================

function showLoginModal() {
    document.getElementById('loginModal').classList.add('active');
    document.getElementById('email').focus();
}

function hideLoginModal() {
    const modal = document.getElementById('loginModal');
    modal.classList.remove('active');
    document.getElementById('loginForm').reset();
    hideLoginError();
}

function showLoginError(message) {
    const errorDiv = document.getElementById('loginError');
    errorDiv.textContent = message;
    errorDiv.classList.add('show');
}

function hideLoginError() {
    const errorDiv = document.getElementById('loginError');
    errorDiv.textContent = '';
    errorDiv.classList.remove('show');
}

async function handleLogin(event) {
    event.preventDefault();
    
    const submitBtn = document.getElementById('loginSubmitBtn');
    const email = document.getElementById('email').value.trim();
    const password = document.getElementById('password').value;
    
    if (!email || !password) {
        showLoginError('Please enter both email and password');
        return;
    }
    
    submitBtn.classList.add('loading');
    submitBtn.disabled = true;
    hideLoginError();
    
    try {
        const response = await fetch(getApiUrl('/api/v1/auth/login'), {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ username: email, password })
        });
        
        const data = await response.json();
        
        if (response.ok && data.success && data.data && data.data.token) {
            state.jwtToken = data.data.token;
            state.isAuthenticated = true;
            localStorage.setItem('jwtToken', state.jwtToken);
            
            updateAuthUI();
            hideLoginModal();
            showToast('success', 'Authentication successful!');
            
            await getUserProfile();
            checkHealth();
        } else {
            throw new Error(data.error || data.message || 'Authentication failed');
        }
    } catch (error) {
        console.error('Login error:', error);
        showLoginError(error.message || 'Failed to authenticate. Please check your credentials.');
    } finally {
        submitBtn.classList.remove('loading');
        submitBtn.disabled = false;
    }
}

function logout() {
    state.jwtToken = null;
    state.isAuthenticated = false;
    state.currentUser = null;
    localStorage.removeItem('jwtToken');
    
    if (state.healthInterval) {
        clearInterval(state.healthInterval);
        state.healthInterval = null;
        document.getElementById('healthAutoRefresh').checked = false;
    }
    
    document.getElementById('userUsername').textContent = '--';
    document.getElementById('userEmail').textContent = '--';
    document.getElementById('userRole').textContent = '--';
    document.getElementById('userDetails').querySelector('.code-block').textContent = 'Authenticate to view profile...';
    
    updateAuthUI();
    showToast('info', 'You have been logged out');
}

function updateAuthUI() {
    const connectionStatus = document.getElementById('connectionStatus');
    const authBtn = document.getElementById('authBtn');
    const statusText = connectionStatus.querySelector('.status-text');
    
    if (state.isAuthenticated) {
        connectionStatus.classList.add('connected');
        statusText.textContent = 'Connected';
        authBtn.innerHTML = '<span class="btn-icon">⚡</span><span>Disconnect</span>';
        authBtn.classList.add('logout');
        authBtn.onclick = logout;
    } else {
        connectionStatus.classList.remove('connected');
        statusText.textContent = 'Disconnected';
        authBtn.innerHTML = '<span class="btn-icon">⚡</span><span>Connect</span>';
        authBtn.classList.remove('logout');
        authBtn.onclick = showLoginModal;
    }
    
    document.querySelectorAll('.auth-required').forEach(el => {
        el.disabled = !state.isAuthenticated;
    });
}

// ===========================================
// API Requests
// ===========================================

async function makeAuthenticatedRequest(endpoint, options = {}) {
    if (!state.isAuthenticated) {
        throw new Error('Authentication required');
    }
    
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + state.jwtToken,
        ...options.headers
    };
    
    const response = await fetch(getApiUrl(endpoint), {
        ...options,
        headers
    });
    
    if (response.status === 401) {
        logout();
        showToast('error', 'Session expired. Please login again.');
        throw new Error('Session expired');
    }
    
    return response;
}

// ===========================================
// Health Check
// ===========================================

async function checkHealth() {
    const healthStatus = document.getElementById('healthStatus');
    const healthRing = document.getElementById('healthRing');
    const healthDetails = document.getElementById('healthDetails');
    
    healthStatus.textContent = '...';
    healthStatus.className = 'health-status';
    
    try {
        const response = await fetch(getApiUrl('/health'));
        const data = await response.json();
        const isHealthy = response.ok && data.status === 'healthy';
        
        state.healthHistory.push({
            timestamp: new Date().toISOString(),
            healthy: isHealthy,
            data: data
        });
        
        if (state.healthHistory.length > 10) {
            state.healthHistory.shift();
        }
        
        healthStatus.textContent = isHealthy ? 'OK' : 'FAIL';
        healthStatus.className = 'health-status ' + (isHealthy ? 'healthy' : 'unhealthy');
        healthRing.className = 'ring-progress ' + (isHealthy ? 'healthy' : 'unhealthy');
        
        healthDetails.querySelector('.code-block').textContent = JSON.stringify(data, null, 2);
        healthDetails.querySelector('.code-block').className = 'code-block ' + (isHealthy ? 'success' : 'error');
        
        updateHealthGrid();
        
    } catch (error) {
        console.error('Health check error:', error);
        
        state.healthHistory.push({
            timestamp: new Date().toISOString(),
            healthy: false,
            error: error.message
        });
        
        if (state.healthHistory.length > 10) {
            state.healthHistory.shift();
        }
        
        healthStatus.textContent = 'ERR';
        healthStatus.className = 'health-status unhealthy';
        healthRing.className = 'ring-progress unhealthy';
        
        healthDetails.querySelector('.code-block').textContent = 'Error: ' + error.message + '\n\nTip: Configure the API Base URL if running from a different origin.';
        healthDetails.querySelector('.code-block').className = 'code-block error';
        
        updateHealthGrid();
    }
}

function updateHealthGrid() {
    const grid = document.getElementById('healthGrid');
    grid.innerHTML = '';
    
    const emptySlots = 10 - state.healthHistory.length;
    for (let i = 0; i < emptySlots; i++) {
        const dot = document.createElement('div');
        dot.className = 'history-dot';
        dot.textContent = '-';
        dot.title = 'No data';
        grid.appendChild(dot);
    }
    
    state.healthHistory.forEach(function(result) {
        const dot = document.createElement('div');
        dot.className = 'history-dot ' + (result.healthy ? 'healthy' : 'unhealthy');
        dot.textContent = result.healthy ? '✓' : '✗';
        dot.title = new Date(result.timestamp).toLocaleTimeString() + ': ' + (result.healthy ? 'Healthy' : 'Unhealthy');
        grid.appendChild(dot);
    });
}

function toggleHealthAutoRefresh() {
    const checkbox = document.getElementById('healthAutoRefresh');
    
    if (checkbox.checked) {
        state.healthInterval = setInterval(checkHealth, 5000);
        checkHealth();
        showToast('info', 'Auto-refresh enabled (5s interval)');
    } else {
        if (state.healthInterval) {
            clearInterval(state.healthInterval);
            state.healthInterval = null;
        }
        showToast('info', 'Auto-refresh disabled');
    }
}

// ===========================================
// Version
// ===========================================

async function getVersion() {
    try {
        const response = await fetch(getApiUrl('/version'));
        const data = await response.json();
        showToast('info', 'Version: ' + (data.version || JSON.stringify(data)));
    } catch (error) {
        showToast('error', 'Failed to get version: ' + error.message);
    }
}

// ===========================================
// User Profile
// ===========================================

async function getUserProfile() {
    var userDetails = document.getElementById('userDetails');
    
    try {
        var response = await makeAuthenticatedRequest('/api/v1/users/self');
        var data = await response.json();
        
        if (data.success && data.data) {
            var user = data.data;
            state.currentUser = user;
            
            // Update enhanced profile display
            var displayName = user.username || user.email || 'User';
            var avatarInitial = displayName.charAt(0).toUpperCase();
            
            document.getElementById('userAvatar').textContent = avatarInitial;
            document.getElementById('userDisplayName').textContent = displayName;
            document.getElementById('userEmailDisplay').textContent = user.email || '--';
            document.getElementById('userRoleDisplay').textContent = (user.roles && user.roles[0]) || 'User';
            document.getElementById('userStatusDisplay').textContent = 'Active';
            
            // Update status indicator
            var statusIndicator = document.getElementById('userStatusIndicator');
            if (statusIndicator) {
                statusIndicator.classList.add('online');
            }
            
            // Update raw details
            userDetails.textContent = JSON.stringify(data, null, 2);
            userDetails.className = 'code-block success';
        } else {
            throw new Error(data.error || data.message || 'Failed to get user profile');
        }
    } catch (error) {
        userDetails.textContent = 'Error: ' + error.message;
        userDetails.className = 'code-block error';
    }
}

// ===========================================
// Schedule Intents
// ===========================================

async function getIntents() {
    const resultDiv = document.getElementById('intentsResult');
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/intents/self');
        const data = await response.json();
        
        if (data.success) {
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
            
            const intents = data.data && data.data.intents;
            if (intents && intents.length > 0) {
                showToast('success', 'Loaded ' + intents.length + ' intent(s)');
            } else {
                showToast('info', 'No intents found');
            }
        } else {
            resultDiv.textContent = 'Error: ' + (data.error || data.message);
            resultDiv.className = 'code-block error';
        }
    } catch (error) {
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
    }
}

function showDeleteIntentsModal() {
    document.getElementById('deleteIntentsModal').classList.add('active');
    document.getElementById('intentIds').focus();
}

function hideDeleteIntentsModal() {
    document.getElementById('deleteIntentsModal').classList.remove('active');
    document.getElementById('intentIds').value = '';
}

async function deleteIntents() {
    const intentIdsInput = document.getElementById('intentIds').value.trim();
    
    if (!intentIdsInput) {
        showToast('error', 'Please enter intent IDs');
        return;
    }
    
    const intentIds = intentIdsInput.split(',').map(function(id) { return id.trim(); }).filter(function(id) { return id; });
    
    if (intentIds.length === 0) {
        showToast('error', 'No valid intent IDs provided');
        return;
    }
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/intents', {
            method: 'DELETE',
            body: JSON.stringify({ intentIds: intentIds })
        });
        
        const data = await response.json();
        
        if (data.success) {
            showToast('success', 'Deleted ' + intentIds.length + ' intent(s)');
            hideDeleteIntentsModal();
            await getIntents();
        } else {
            showToast('error', data.error || data.message || 'Failed to delete intents');
        }
    } catch (error) {
        showToast('error', 'Error: ' + error.message);
    }
}

// ===========================================
// Scheduling Strategies
// ===========================================

async function getStrategies() {
    const resultDiv = document.getElementById('strategiesResult');
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/strategies/self');
        const data = await response.json();
        
        if (data.success) {
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
            
            const strategies = data.data && data.data.strategies;
            if (strategies && strategies.length > 0) {
                showToast('success', 'Loaded ' + strategies.length + ' strategy(ies)');
            } else {
                showToast('info', 'No strategies found');
            }
        } else {
            resultDiv.textContent = 'Error: ' + (data.error || data.message);
            resultDiv.className = 'code-block error';
        }
    } catch (error) {
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
    }
}

function addStrategy() {
    const container = document.getElementById('strategiesContainer');
    const actionsDiv = document.getElementById('strategiesActions');
    state.strategyCounter++;
    const strategyId = 'strategy-' + state.strategyCounter;
    
    const strategyDiv = document.createElement('div');
    strategyDiv.className = 'strategy-item';
    strategyDiv.id = strategyId;
    
    strategyDiv.innerHTML = '<div class="strategy-header">' +
        '<h4>Strategy #' + state.strategyCounter + '</h4>' +
        '<button type="button" class="remove-strategy-btn" onclick="removeStrategy(\'' + strategyId + '\')">' +
        '✕ Remove</button></div>' +
        '<div class="strategy-form">' +
        '<div><label>Strategy Namespace</label>' +
        '<input type="text" name="strategyNamespace" placeholder="e.g., default, trading, ml"></div>' +
        '<div><label>Priority (0-100)</label>' +
        '<input type="number" name="priority" value="50" min="0" max="100" placeholder="50"></div>' +
        '<div><label>Execution Time (ns)</label>' +
        '<input type="number" name="executionTime" value="20000000" placeholder="20000000"></div>' +
        '<div><label>Command Regex (optional)</label>' +
        '<input type="text" name="commandRegex" placeholder="e.g., nr-gnb|ping"></div>' +
        '<div class="full-width"><label>K8s Namespaces (comma separated)</label>' +
        '<input type="text" name="k8sNamespace" placeholder="default, kube-system"></div>' +
        '<div class="full-width selectors-container"><label>Label Selectors</label>' +
        '<div class="selectors-list" id="selectors-' + strategyId + '">' +
        '<div class="selector-row">' +
        '<input type="text" name="selectorKey" placeholder="Key">' +
        '<input type="text" name="selectorValue" placeholder="Value">' +
        '<button type="button" onclick="removeSelector(this)">✕</button>' +
        '</div></div>' +
        '<button type="button" class="add-selector-btn" onclick="addSelectorToStrategy(\'' + strategyId + '\')">' +
        '+ Add Selector</button></div></div>';
    
    container.appendChild(strategyDiv);
    actionsDiv.style.display = 'flex';
}

function removeStrategy(strategyId) {
    const strategyItem = document.getElementById(strategyId);
    if (strategyItem) {
        strategyItem.remove();
    }
    
    const container = document.getElementById('strategiesContainer');
    const actionsDiv = document.getElementById('strategiesActions');
    
    if (container.children.length === 0) {
        actionsDiv.style.display = 'none';
    }
}

function clearAllStrategies() {
    const container = document.getElementById('strategiesContainer');
    const actionsDiv = document.getElementById('strategiesActions');
    
    container.innerHTML = '';
    actionsDiv.style.display = 'none';
    state.strategyCounter = 0;
    
    showToast('info', 'Form cleared');
}

function addSelectorToStrategy(strategyId) {
    const selectorsContainer = document.getElementById('selectors-' + strategyId);
    
    const selectorDiv = document.createElement('div');
    selectorDiv.className = 'selector-row';
    selectorDiv.innerHTML = '<input type="text" name="selectorKey" placeholder="Key">' +
        '<input type="text" name="selectorValue" placeholder="Value">' +
        '<button type="button" onclick="removeSelector(this)">✕</button>';
    
    selectorsContainer.appendChild(selectorDiv);
}

function removeSelector(button) {
    const selectorDiv = button.parentElement;
    const parentContainer = selectorDiv.parentElement;
    selectorDiv.remove();
    
    if (parentContainer.children.length === 0) {
        const strategyId = parentContainer.id.replace('selectors-', '');
        addSelectorToStrategy(strategyId);
    }
}

async function saveAllStrategies() {
    const resultDiv = document.getElementById('strategiesResult');
    const strategyItems = document.querySelectorAll('.strategy-item');
    
    if (strategyItems.length === 0) {
        showToast('error', 'No strategies to save');
        return;
    }
    
    for (let i = 0; i < strategyItems.length; i++) {
        const item = strategyItems[i];
        const strategy = {};
        
        const strategyNamespace = item.querySelector('input[name="strategyNamespace"]');
        if (strategyNamespace && strategyNamespace.value.trim()) {
            strategy.strategyNamespace = strategyNamespace.value.trim();
        }
        
        const priority = item.querySelector('input[name="priority"]');
        if (priority && priority.value) {
            strategy.priority = parseInt(priority.value);
        }
        
        const executionTime = item.querySelector('input[name="executionTime"]');
        if (executionTime && executionTime.value) {
            strategy.executionTime = parseInt(executionTime.value);
        }
        
        const commandRegex = item.querySelector('input[name="commandRegex"]');
        if (commandRegex && commandRegex.value.trim()) {
            strategy.commandRegex = commandRegex.value.trim();
        }
        
        const k8sNamespaceInput = item.querySelector('input[name="k8sNamespace"]');
        if (k8sNamespaceInput && k8sNamespaceInput.value.trim()) {
            strategy.k8sNamespace = k8sNamespaceInput.value.trim().split(',').map(function(ns) { return ns.trim(); }).filter(function(ns) { return ns; });
        }
        
        const labelSelectors = [];
        const selectorItems = item.querySelectorAll('.selector-row');
        
        for (let j = 0; j < selectorItems.length; j++) {
            const selectorItem = selectorItems[j];
            const keyInput = selectorItem.querySelector('input[name="selectorKey"]');
            const valueInput = selectorItem.querySelector('input[name="selectorValue"]');
            const key = keyInput && keyInput.value.trim();
            const value = valueInput && valueInput.value.trim();
            if (key && value) {
                labelSelectors.push({ key: key, value: value });
            }
        }
        
        if (labelSelectors.length > 0) {
            strategy.labelSelectors = labelSelectors;
        }
        
        try {
            const response = await makeAuthenticatedRequest('/api/v1/strategies', {
                method: 'POST',
                body: JSON.stringify(strategy)
            });
            
            const data = await response.json();
            
            if (data.success) {
                resultDiv.textContent = JSON.stringify(data, null, 2);
                resultDiv.className = 'code-block success';
                showToast('success', 'Strategy created successfully');
            } else {
                resultDiv.textContent = 'Error: ' + (data.error || data.message);
                resultDiv.className = 'code-block error';
                showToast('error', data.error || data.message || 'Failed to create strategy');
            }
        } catch (error) {
            resultDiv.textContent = 'Error: ' + error.message;
            resultDiv.className = 'code-block error';
            showToast('error', error.message);
        }
    }
}

function showDeleteStrategyModal() {
    document.getElementById('deleteStrategyModal').classList.add('active');
    document.getElementById('deleteStrategyId').focus();
}

function hideDeleteStrategyModal() {
    document.getElementById('deleteStrategyModal').classList.remove('active');
    document.getElementById('deleteStrategyId').value = '';
}

async function deleteStrategy() {
    const strategyId = document.getElementById('deleteStrategyId').value.trim();
    
    if (!strategyId) {
        showToast('error', 'Please enter a strategy ID');
        return;
    }
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/strategies', {
            method: 'DELETE',
            body: JSON.stringify({ strategyId: strategyId })
        });
        
        const data = await response.json();
        
        if (data.success) {
            showToast('success', 'Strategy deleted successfully');
            hideDeleteStrategyModal();
            await getStrategies();
        } else {
            showToast('error', data.error || data.message || 'Failed to delete strategy');
        }
    } catch (error) {
        showToast('error', 'Error: ' + error.message);
    }
}

// ===========================================
// Users Management
// ===========================================

async function getUsers() {
    const resultDiv = document.getElementById('usersResult');
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/users');
        const data = await response.json();
        
        if (data.success) {
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
        } else {
            resultDiv.textContent = 'Error: ' + (data.error || data.message);
            resultDiv.className = 'code-block error';
        }
    } catch (error) {
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
    }
}

// ===========================================
// Roles & Permissions
// ===========================================

async function getRoles() {
    const resultDiv = document.getElementById('rolesResult');
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/roles');
        const data = await response.json();
        
        if (data.success) {
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
        } else {
            resultDiv.textContent = 'Error: ' + (data.error || data.message);
            resultDiv.className = 'code-block error';
        }
    } catch (error) {
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
    }
}

async function getPermissions() {
    const resultDiv = document.getElementById('rolesResult');
    
    try {
        const response = await makeAuthenticatedRequest('/api/v1/permissions');
        const data = await response.json();
        
        if (data.success) {
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
        } else {
            resultDiv.textContent = 'Error: ' + (data.error || data.message);
            resultDiv.className = 'code-block error';
        }
    } catch (error) {
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
    }
}

// ===========================================
// Pod-PID Mapping
// ===========================================

var podPidsInterval = null;

async function getPodPids() {
    var accordion = document.getElementById('podsAccordion');
    var resultDiv = document.getElementById('podsResult');
    var nodeNameDisplay = document.getElementById('nodeNameDisplay');
    var podsCount = document.getElementById('podsCount');
    var processesCount = document.getElementById('processesCount');
    var lastUpdated = document.getElementById('lastUpdated');
    
    try {
        var response = await makeAuthenticatedRequest('/api/v1/pods/pids');
        var data = await response.json();
        
        if (data.success && data.data) {
            var pods = data.data.pods || [];
            var totalProcesses = 0;
            var nodeName = data.data.node_name || 'Unknown';
            
            // Build accordion items
            var accordionHtml = '';
            
            if (pods.length === 0) {
                accordionHtml = '<div class="pods-empty-state">No pods found on this node</div>';
            } else {
                pods.forEach(function(pod, podIndex) {
                    var processes = pod.processes || [];
                    totalProcesses += processes.length;
                    
                    var podUid = pod.pod_uid || '--';
                    var podId = pod.pod_id || '--';
                    var processCount = processes.length;
                    
                    accordionHtml += '<div class="pod-accordion-item" data-pod-uid="' + escapeHtml(podUid) + '">' +
                        '<div class="pod-accordion-header" onclick="togglePod(this)">' +
                        '<div class="pod-accordion-info">' +
                        '<span class="pod-accordion-toggle">▶</span>' +
                        '<div class="pod-accordion-title">' +
                        '<span class="pod-uid-full" title="Pod UID">' + escapeHtml(podUid) + '</span>' +
                        '<span class="pod-id-badge">' + escapeHtml(podId) + '</span>' +
                        '</div>' +
                        '</div>' +
                        '<div class="pod-accordion-meta">' +
                        '<span class="process-count-badge">' + processCount + ' process' + (processCount !== 1 ? 'es' : '') + '</span>' +
                        '</div>' +
                        '</div>' +
                        '<div class="pod-accordion-content">';
                    
                    if (processes.length === 0) {
                        accordionHtml += '<div class="pods-empty-state">No processes in this pod</div>';
                    } else {
                        accordionHtml += '<table class="pod-processes-table">' +
                            '<thead><tr>' +
                            '<th>PID</th>' +
                            '<th>Command</th>' +
                            '<th>PPID</th>' +
                            '<th>Container ID</th>' +
                            '</tr></thead><tbody>';
                        
                        processes.forEach(function(proc) {
                            accordionHtml += '<tr>' +
                                '<td class="pid-cell">' + (proc.pid || '--') + '</td>' +
                                '<td class="command-cell" title="' + escapeHtml(proc.command || '') + '">' + escapeHtml(proc.command || '--') + '</td>' +
                                '<td>' + (proc.ppid || '--') + '</td>' +
                                '<td class="container-id-cell" title="' + escapeHtml(proc.container_id || '') + '">' + truncateText(proc.container_id || '--', 16) + '</td>' +
                                '</tr>';
                        });
                        
                        accordionHtml += '</tbody></table>';
                    }
                    
                    accordionHtml += '</div></div>';
                });
            }
            
            accordion.innerHTML = accordionHtml;
            
            // Update summary
            nodeNameDisplay.textContent = nodeName;
            podsCount.textContent = pods.length;
            processesCount.textContent = totalProcesses;
            lastUpdated.textContent = data.data.timestamp ? formatTimestamp(data.data.timestamp) : 'Now';
            
            // Update raw JSON
            resultDiv.textContent = JSON.stringify(data, null, 2);
            resultDiv.className = 'code-block success';
            
            showToast('success', 'Loaded ' + pods.length + ' pod(s) with ' + totalProcesses + ' process(es)');
        } else {
            throw new Error(data.error || data.message || 'Failed to get pod-PID mappings');
        }
    } catch (error) {
        console.error('Pod-PID mapping error:', error);
        accordion.innerHTML = '<div class="pods-empty-state error">Error: ' + escapeHtml(error.message) + '</div>';
        resultDiv.textContent = 'Error: ' + error.message;
        resultDiv.className = 'code-block error';
        nodeNameDisplay.textContent = '--';
        podsCount.textContent = '--';
        processesCount.textContent = '--';
        lastUpdated.textContent = '--';
    }
}

function togglePod(headerElement) {
    var accordionItem = headerElement.parentElement;
    accordionItem.classList.toggle('expanded');
}

function toggleAllPods(expand) {
    var items = document.querySelectorAll('.pod-accordion-item');
    items.forEach(function(item) {
        if (expand) {
            item.classList.add('expanded');
        } else {
            item.classList.remove('expanded');
        }
    });
    showToast('info', expand ? 'Expanded all pods' : 'Collapsed all pods');
}

function togglePodPidsAutoRefresh() {
    var checkbox = document.getElementById('podPidsAutoRefresh');
    
    if (checkbox.checked) {
        podPidsInterval = setInterval(getPodPids, 5000);
        getPodPids();
        showToast('info', 'Pod-PID auto-refresh enabled (5s interval)');
    } else {
        if (podPidsInterval) {
            clearInterval(podPidsInterval);
            podPidsInterval = null;
        }
        showToast('info', 'Pod-PID auto-refresh disabled');
    }
}

function truncateText(text, maxLen) {
    if (!text) return '--';
    if (text.length <= maxLen) return text;
    return text.substring(0, maxLen - 3) + '...';
}

function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatTimestamp(isoString) {
    try {
        var date = new Date(isoString);
        return date.toLocaleTimeString();
    } catch (e) {
        return isoString;
    }
}

// ===========================================
// Refresh All Data
// ===========================================

async function refreshAllData() {
    showToast('info', 'Refreshing all data...');
    
    await checkHealth();
    
    if (state.isAuthenticated) {
        await getUserProfile();
        await getStrategies();
        await getIntents();
        await getPodPids();
    }
    
    showToast('success', 'Data refreshed');
}

// ===========================================
// Toast Notifications
// ===========================================

function showToast(type, message) {
    const container = document.getElementById('toastContainer');
    
    const toast = document.createElement('div');
    toast.className = 'toast ' + type;
    
    const icons = {
        success: '✓',
        error: '✕',
        info: 'ℹ',
        warning: '⚠'
    };
    
    toast.innerHTML = '<span class="toast-icon">' + (icons[type] || 'ℹ') + '</span>' +
        '<span class="toast-message">' + message + '</span>' +
        '<button class="toast-close" onclick="this.parentElement.remove()">×</button>';
    
    container.appendChild(toast);
    
    setTimeout(function() {
        if (toast.parentElement) {
            toast.style.animation = 'toastOut 0.3s ease forwards';
            setTimeout(function() { toast.remove(); }, 300);
        }
    }, 4000);
}

// Add CSS for toast out animation
var style = document.createElement('style');
style.textContent = '@keyframes toastOut { to { opacity: 0; transform: translateX(100%); } }';
document.head.appendChild(style);

// ===========================================
// Event Listeners & Initialization
// ===========================================

document.addEventListener('DOMContentLoaded', function() {
    updateAuthUI();
    updateHealthGrid();
    
    // Close modals on outside click
    document.querySelectorAll('.modal-overlay').forEach(function(modal) {
        modal.addEventListener('click', function(e) {
            if (e.target === this) {
                this.classList.remove('active');
            }
        });
    });
    
    // Keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-overlay.active').forEach(function(modal) {
                modal.classList.remove('active');
            });
        }
        
        if (e.ctrlKey && e.key === 'Enter') {
            var loginModal = document.getElementById('loginModal');
            if (loginModal.classList.contains('active')) {
                document.getElementById('loginForm').dispatchEvent(new Event('submit'));
            }
        }
    });
    
    // Initial health check
    setTimeout(checkHealth, 500);
    
    // Load user profile if already authenticated
    if (state.isAuthenticated) {
        getUserProfile();
    }
});

// Handle token from URL (for OAuth flows)
(function() {
    var urlParams = new URLSearchParams(window.location.search);
    var token = urlParams.get('token');
    if (token) {
        state.jwtToken = token;
        state.isAuthenticated = true;
        localStorage.setItem('jwtToken', token);
        window.history.replaceState({}, document.title, window.location.pathname);
        updateAuthUI();
    }
})();
