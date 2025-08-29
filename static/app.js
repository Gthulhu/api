// Global variables
let jwtToken = localStorage.getItem('jwtToken');
let isAuthenticated = !!jwtToken;

// Auto-refresh intervals
let healthInterval = null;
let metricsInterval = null;
let healthHistory = []; // Store last 10 health check results

// DOM elements
const authStatus = document.getElementById('authStatus');
const authText = document.getElementById('authText');
const authBtn = document.getElementById('authBtn');
const authModal = document.getElementById('authModal');
const authForm = document.getElementById('authForm');
const strategiesForm = document.getElementById('strategiesForm');

// Initialize the app
document.addEventListener('DOMContentLoaded', function() {
    updateAuthStatus();
    setupEventListeners();
    updateAuthRequiredButtons();
});

// Update authentication status display
function updateAuthStatus() {
    if (isAuthenticated) {
        authText.textContent = 'Authenticated';
        authBtn.textContent = 'Clear Token';
        authBtn.onclick = clearToken;
    } else {
        authText.textContent = 'Not Authenticated';
        authBtn.textContent = 'Get Token';
        authBtn.onclick = showAuthModal;
    }
    updateAuthRequiredButtons();
}

// Update buttons that require authentication
function updateAuthRequiredButtons() {
    const authRequiredElements = document.querySelectorAll('.auth-required');
    authRequiredElements.forEach(element => {
        element.disabled = !isAuthenticated;
    });
    
    // Stop metrics auto-refresh if user is not authenticated
    if (!isAuthenticated && metricsInterval) {
        clearInterval(metricsInterval);
        metricsInterval = null;
        const btn = document.getElementById('metricsAutoBtn');
        const intervalInput = document.getElementById('metricsInterval');
        if (btn) {
            btn.textContent = 'Start Auto-refresh';
        }
        if (intervalInput) {
            intervalInput.disabled = false;
        }
    }
}

// Show authentication modal
function showAuthModal() {
    authModal.style.display = 'block';
}

// Hide authentication modal
function hideAuthModal() {
    authModal.style.display = 'none';
}

// Clear JWT token
function clearToken() {
    jwtToken = null;
    isAuthenticated = false;
    localStorage.removeItem('jwtToken');
    updateAuthStatus();
    showResult('authResult', 'Authentication token cleared', 'success');
}

// Setup event listeners
function setupEventListeners() {
    // Close modal when clicking outside
    window.onclick = function(event) {
        if (event.target === authModal) {
            hideAuthModal();
        }
    }

    // Auth form submission
    authForm.addEventListener('submit', async function(e) {
        e.preventDefault();
        await getJWTToken();
    });

    // Strategies form submission
    strategiesForm.addEventListener('submit', async function(e) {
        e.preventDefault();
        await saveStrategies();
    });
}

// Get JWT Token
async function getJWTToken() {
    const publicKey = document.getElementById('publicKey').value.trim();
    
    if (!publicKey) {
        alert('Please enter public key');
        return;
    }

    try {
        const response = await fetch('/api/v1/auth/token', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                public_key: publicKey
            })
        });

        const data = await response.json();
        
        if (data.success && data.token) {
            jwtToken = data.token;
            isAuthenticated = true;
            localStorage.setItem('jwtToken', jwtToken);
            updateAuthStatus();
            hideAuthModal();
            showResult('authResult', 'Authentication successful!', 'success');
        } else {
            showResult('authResult', 'Authentication failed: ' + (data.error || data.message), 'error');
        }
    } catch (error) {
        showResult('authResult', 'Request failed: ' + error.message, 'error');
    }
}

// API request helper with authentication
async function makeAuthenticatedRequest(url, options = {}) {
    if (!isAuthenticated) {
        throw new Error('Authentication required');
    }

    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${jwtToken}`,
        ...options.headers
    };

    return fetch(url, {
        ...options,
        headers
    });
}

// Check health endpoint
async function checkHealth() {
    try {
        const response = await fetch('/health');
        const data = await response.json();
        const isHealthy = response.ok && data.status == "healthy";
        
        // Add to health history
        healthHistory.push({
            timestamp: new Date().toISOString(),
            healthy: isHealthy,
            data: data
        });
        
        // Keep only last 10 results
        if (healthHistory.length > 10) {
            healthHistory.shift();
        }
        
        // Update health grid
        updateHealthGrid();
        
        showResult('healthResult', JSON.stringify(data, null, 2), isHealthy ? 'success' : 'error');
    } catch (error) {
        // Add error to health history
        healthHistory.push({
            timestamp: new Date().toISOString(),
            healthy: false,
            error: error.message
        });
        
        // Keep only last 10 results
        if (healthHistory.length > 10) {
            healthHistory.shift();
        }
        
        // Update health grid
        updateHealthGrid();
        
        showResult('healthResult', 'Request failed: ' + error.message, 'error');
    }
}

// Get Pod-PID mappings
async function getPodPids() {
    try {
        const response = await makeAuthenticatedRequest('/api/v1/pods/pids');
        const data = await response.json();
        
        if (data.success) {
            showResult('podPidsResult', JSON.stringify(data, null, 2), 'success');
        } else {
            showResult('podPidsResult', 'Failed: ' + (data.error || data.message), 'error');
        }
    } catch (error) {
        showResult('podPidsResult', 'Request failed: ' + error.message, 'error');
    }
}

// Get scheduling strategies
async function getStrategies() {
    try {
        const response = await makeAuthenticatedRequest('/api/v1/scheduling/strategies');
        const data = await response.json();
        
        if (data.success) {
            showResult('strategiesResult', JSON.stringify(data, null, 2), 'success');
        } else {
            showResult('strategiesResult', 'Failed: ' + (data.error || data.message), 'error');
        }
    } catch (error) {
        showResult('strategiesResult', 'Request failed: ' + error.message, 'error');
    }
}

// Save scheduling strategies
async function saveStrategies() {
    try {
        const formData = new FormData(strategiesForm);
        
        // Collect selectors
        const selectors = [];
        const selectorKeys = document.querySelectorAll('input[name="selectorKey"]');
        const selectorValues = document.querySelectorAll('input[name="selectorValue"]');
        
        for (let i = 0; i < selectorKeys.length; i++) {
            const key = selectorKeys[i].value.trim();
            const value = selectorValues[i].value.trim();
            if (key && value) {
                selectors.push({ key, value });
            }
        }

        // Build strategy object
        const strategy = {
            priority: formData.get('priority') === 'on',
            execution_time: parseInt(formData.get('executionTime')),
            selectors: selectors
        };

        const pid = formData.get('pid');
        if (pid) {
            strategy.pid = parseInt(pid);
        }

        const commandRegex = formData.get('commandRegex');
        if (commandRegex) {
            strategy.command_regex = commandRegex;
        }

        const requestBody = {
            strategies: [strategy]
        };

        const response = await makeAuthenticatedRequest('/api/v1/scheduling/strategies', {
            method: 'POST',
            body: JSON.stringify(requestBody)
        });

        const data = await response.json();
        
        if (data.success) {
            showResult('strategiesResult', 'Strategy saved successfully: ' + JSON.stringify(data, null, 2), 'success');
            strategiesForm.reset();
        } else {
            showResult('strategiesResult', 'Save failed: ' + (data.error || data.message), 'error');
        }
    } catch (error) {
        showResult('strategiesResult', 'Request failed: ' + error.message, 'error');
    }
}

// Get current metrics (replaces submitMetrics)
async function getMetrics() {
    try {
        const response = await makeAuthenticatedRequest('/api/v1/metrics', {
            method: 'GET'
        });
        
        const data = await response.json();
        
        if (data.success && data.data) {
            // Format the metrics data nicely
            const metrics = data.data;
            const formattedMetrics = {
                "Last Update": data.metrics_timestamp,
                "UserSched Last Run": metrics.usersched_last_run_at,
                "Queued Tasks": metrics.nr_queued,
                "Scheduled Tasks": metrics.nr_scheduled,
                "Running Tasks": metrics.nr_running,
                "Online CPUs": metrics.nr_online_cpus,
                "User Dispatches": metrics.nr_user_dispatches,
                "Kernel Dispatches": metrics.nr_kernel_dispatches,
                "Cancel Dispatches": metrics.nr_cancel_dispatches,
                "Bounce Dispatches": metrics.nr_bounce_dispatches,
                "Failed Dispatches": metrics.nr_failed_dispatches,
                "Scheduler Congested": metrics.nr_sched_congested
            };
            
            showResult('metricsResult', JSON.stringify(formattedMetrics, null, 2), 'success');
        } else {
            showResult('metricsResult', data.message || 'No metrics data available', 'info');
        }
    } catch (error) {
        showResult('metricsResult', 'Request failed: ' + error.message, 'error');
    }
}

// Add selector input
function addSelector() {
    const selectorsDiv = document.getElementById('selectors');
    const selectorDiv = document.createElement('div');
    selectorDiv.className = 'selector';
    selectorDiv.innerHTML = `
        <input type="text" name="selectorKey" placeholder="key">
        <input type="text" name="selectorValue" placeholder="value">
        <button type="button" onclick="removeSelector(this)">Remove</button>
    `;
    selectorsDiv.appendChild(selectorDiv);
}

// Remove selector input
function removeSelector(button) {
    const selectorDiv = button.parentElement;
    selectorDiv.remove();
}

// Show result in specified element
function showResult(elementId, message, type = 'success') {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = message;
        element.className = `result ${type}`;
    }
}

// Fill sample public key for testing
function fillSampleKey() {
    const sampleKey = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAny28YMC2/+yYj3T29lz6
0uryNz8gNVrqD7lTJuHQ3DMTE6ADqnERy8VgHve0tWzhJc5ZBZ1Hduvj+z/kNqbc
U81YGhmfOrQ3iFNYBlSAseIHdAw39HGyC6OKzTXI4HRpc8CwcF6hKExkyWlkALr5
i+IQDfimvarjjZ6Nm368L0Rthv3KOkI5CqRZ6bsVwwBug7GcdkvFs3LiRSKlMBpH
2tCkZ5ZZE8VyuK7VnlwV7n6EHzN5BqaHq8HVLw2KzvibSi+/5wIZV2Yx33tViLbh
OsZqLt6qQCGGgKzNX4TGwRLGAiVV1NCpgQhimZ4YP2thqSsqbaISOuvFlYq+QGP1
bcvcHB7UhT1ZnHSDYcbT2qiD3VoqytXVKLB1X5XCD99YLSP9B32f1lvZD4MhDtE4
IhAuqn15MGB5ct4yj/uMldFScs9KhqnWcwS4K6Qx3IfdB+ZxT5hEOWJLEcGqe/CS
XITNG7oS9mrSAJJvHSLz++4R/Sh1MnT2YWjyDk6qeeqAwut0w5iDKWt7qsGEcHFP
IVVlos+xLfrPDtgHQk8upjslUcMyMDTf21Y3RdJ3k1gTR9KHEwzKeiNlLjen9ekF
WupF8jik1aYRWL6h54ZyGxwKEyMYi9o18G2pXPzvVaPYtU+TGXdO4QwiES72TNCD
bNaGj75Gj0sN+LfjjQ4A898CAwEAAQ==
-----END PUBLIC KEY-----`;
    document.getElementById('publicKey').value = sampleKey;
}

// Update health status grid
function updateHealthGrid() {
    const healthGrid = document.getElementById('healthGrid');
    if (!healthGrid) return;
    
    healthGrid.innerHTML = '';
    
    // Fill empty slots if we have less than 10 results
    const totalSlots = 10;
    const emptySlots = totalSlots - healthHistory.length;
    
    // Add empty slots first
    for (let i = 0; i < emptySlots; i++) {
        const box = document.createElement('div');
        box.className = 'status-box';
        box.style.backgroundColor = '#f0f0f0';
        box.style.borderColor = '#ddd';
        box.title = 'No data';
        healthGrid.appendChild(box);
    }
    
    // Add actual health results
    healthHistory.forEach((result, index) => {
        const box = document.createElement('div');
        box.className = `status-box ${result.healthy ? 'healthy' : 'unhealthy'}`;
        box.textContent = result.healthy ? '✓' : '✗';
        box.title = `${new Date(result.timestamp).toLocaleTimeString()}: ${result.healthy ? 'Healthy' : 'Unhealthy'}`;
        healthGrid.appendChild(box);
    });
}

// Toggle health auto-refresh
function toggleHealthAutoRefresh() {
    const btn = document.getElementById('healthAutoBtn');
    const intervalInput = document.getElementById('healthInterval');
    
    if (healthInterval) {
        // Stop auto-refresh
        clearInterval(healthInterval);
        healthInterval = null;
        btn.textContent = 'Start Auto-refresh';
        intervalInput.disabled = false;
    } else {
        // Start auto-refresh
        const interval = parseInt(intervalInput.value) * 1000;
        if (interval < 1000) {
            alert('Interval must be at least 1 second');
            return;
        }
        
        healthInterval = setInterval(checkHealth, interval);
        btn.textContent = 'Stop Auto-refresh';
        intervalInput.disabled = true;
        
        // Do an immediate check
        checkHealth();
    }
}

// Toggle metrics auto-refresh
function toggleMetricsAutoRefresh() {
    const btn = document.getElementById('metricsAutoBtn');
    const intervalInput = document.getElementById('metricsInterval');
    
    if (metricsInterval) {
        // Stop auto-refresh
        clearInterval(metricsInterval);
        metricsInterval = null;
        btn.textContent = 'Start Auto-refresh';
        intervalInput.disabled = false;
    } else {
        // Start auto-refresh
        const interval = parseInt(intervalInput.value) * 1000;
        if (interval < 1000) {
            alert('Interval must be at least 1 second');
            return;
        }
        
        if (!isAuthenticated) {
            alert('Authentication required for metrics auto-refresh');
            return;
        }
        
        metricsInterval = setInterval(getMetrics, interval);
        btn.textContent = 'Stop Auto-refresh';
        intervalInput.disabled = true;
        
        // Do an immediate check
        getMetrics();
    }
}
