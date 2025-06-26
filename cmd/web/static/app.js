// Global state
let currentLogs = [];
let selectedLogId = null;

// DOM elements
const uploadForm = document.getElementById('upload-form');
const uploadBtn = document.getElementById('upload-btn');
const uploadStatus = document.getElementById('upload-status');
const mainInterface = document.getElementById('main-interface');
const logsTbody = document.getElementById('logs-tbody');
const logCount = document.getElementById('log-count');
const sourceCode = document.getElementById('source-code');
const sourceSelector = document.getElementById('source-selector');
const sourceInfo = document.getElementById('source-info');

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    setupEventListeners();
});

function setupEventListeners() {
    // File upload form
    uploadForm.addEventListener('submit', handleFileUpload);
    
    // Source selector change
    sourceSelector.addEventListener('change', handleSourceSelection);
}

async function handleFileUpload(event) {
    event.preventDefault();
    
    const fileInput = document.getElementById('logfile');
    const file = fileInput.files[0];
    
    if (!file) {
        showStatus('Please select a file', 'error');
        return;
    }
    
    // Disable upload button and show loading
    uploadBtn.disabled = true;
    uploadBtn.textContent = 'Processing...';
    showStatus('Uploading and processing log file...', 'info');
    
    // Create abort controller for timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
        controller.abort();
    }, 60000); // 60 second timeout
    
    try {
        const formData = new FormData();
        formData.append('logfile', file);
        
        const response = await fetch('/api/upload', {
            method: 'POST',
            body: formData,
            signal: controller.signal
        });
        
        // Clear timeout since request completed
        clearTimeout(timeoutId);
        
        // Check if response is ok first
        if (!response.ok) {
            let errorMessage;
            try {
                const errorText = await response.text();
                errorMessage = `Server error (${response.status}): ${errorText}`;
            } catch {
                errorMessage = `Server error (${response.status}): Unable to read error details`;
            }
            throw new Error(errorMessage);
        }
        
        // Parse JSON response
        let result;
        try {
            result = await response.json();
        } catch (jsonError) {
            throw new Error('Invalid response from server: ' + jsonError.message);
        }
        
        // Check application-level success
        if (result.success) {
            showStatus(`Successfully processed ${result.count} logs`, 'success');
            await loadLogs();
            showMainInterface();
        } else {
            throw new Error(result.message || 'Upload failed for unknown reason');
        }
        
    } catch (error) {
        // Clear timeout in case of error
        clearTimeout(timeoutId);
        
        // Provide specific error messages based on error type
        if (error.name === 'AbortError') {
            showStatus('Upload timed out after 60 seconds. Please try again with a smaller file or check your connection.', 'error');
        } else if (error.message.includes('Server error')) {
            showStatus(error.message, 'error');
        } else if (error.message.includes('Failed to fetch')) {
            showStatus('Network error: Unable to connect to server. Please check if the server is running.', 'error');
        } else {
            showStatus('Upload failed: ' + error.message, 'error');
        }
    } finally {
        // Re-enable upload button
        uploadBtn.disabled = false;
        uploadBtn.textContent = 'Upload & Process';
    }
}

async function loadLogs() {
    try {
        const response = await fetch('/api/logs');
        if (response.ok) {
            currentLogs = await response.json();
            renderLogsTable();
        } else {
            showStatus('Error loading logs', 'error');
        }
    } catch (error) {
        showStatus('Error loading logs: ' + error.message, 'error');
    }
}

function renderLogsTable() {
    logsTbody.innerHTML = '';
    logCount.textContent = `${currentLogs.length} logs`;
    
    currentLogs.forEach((log, index) => {
        const row = document.createElement('tr');
        row.dataset.logId = log.id;
        
        row.innerHTML = `
            <td>${log.time}</td>
            <td>${escapeHtml(log.service || '')}</td>
            <td class="message-cell">${escapeHtml(log.message)}</td>
            <td class="sources-count">${log.sources}</td>
            <td>
                <button class="delete-btn" onclick="deleteLog(${log.id})">Delete</button>
            </td>
        `;
        
        // Add click handler for row selection
        row.addEventListener('click', () => selectLog(log.id, row));
        
        logsTbody.appendChild(row);
    });
}

async function selectLog(logId, rowElement) {
    // Update selected row styling
    document.querySelectorAll('#logs-tbody tr').forEach(tr => tr.classList.remove('selected'));
    rowElement.classList.add('selected');
    
    selectedLogId = logId;
    
    // Load source code for this log
    await loadSourceCode(logId);
}

async function loadSourceCode(logId, sourceIdx = 0) {
    try {
        let url = `/api/logs/${logId}/source`;
        if (sourceIdx > 0) {
            url += `?source=${sourceIdx}`;
        }
        
        const response = await fetch(url);
        if (response.ok) {
            const sourceData = await response.json();
            renderSourceCode(sourceData);
        } else {
            sourceCode.innerHTML = '<code>Error loading source code</code>';
            sourceInfo.textContent = '';
        }
    } catch (error) {
        sourceCode.innerHTML = '<code>Error loading source code: ' + error.message + '</code>';
        sourceInfo.textContent = '';
    }
}

function renderSourceCode(sourceData) {
    if (!sourceData.content || sourceData.content === "No source code available") {
        sourceCode.innerHTML = '<code>No source code available</code>';
        sourceInfo.textContent = '';
        sourceSelector.classList.add('hidden');
        return;
    }
    
    // Update source info
    sourceInfo.textContent = `${sourceData.path}:${sourceData.line}`;
    
    // Handle multiple sources
    if (sourceData.sources && sourceData.sources.length > 1) {
        populateSourceSelector(sourceData.sources, sourceData.selectedIdx || 0);
        sourceSelector.classList.remove('hidden');
    } else {
        sourceSelector.classList.add('hidden');
    }
    
    // Render source code with line highlighting
    const lines = sourceData.content.split('\n');
    const targetLine = sourceData.line;
    
    let highlightedContent = '';
    lines.forEach((line, index) => {
        const lineNumber = index + 1;
        const isTargetLine = lineNumber === targetLine;
        
        if (isTargetLine) {
            highlightedContent += `<span class="highlight-line">${escapeHtml(line)}</span>\n`;
        } else {
            highlightedContent += escapeHtml(line) + '\n';
        }
    });
    
    sourceCode.innerHTML = `<code>${highlightedContent}</code>`;
}

function populateSourceSelector(sources, selectedIdx) {
    sourceSelector.innerHTML = '';
    
    sources.forEach((source, index) => {
        const option = document.createElement('option');
        option.value = index;
        option.textContent = `${source.path}:${source.line}`;
        option.selected = index === selectedIdx;
        sourceSelector.appendChild(option);
    });
}

async function handleSourceSelection() {
    if (selectedLogId !== null) {
        const sourceIdx = parseInt(sourceSelector.value);
        await loadSourceCode(selectedLogId, sourceIdx);
    }
}

async function deleteLog(logId) {
    if (!confirm('Are you sure you want to delete this log entry?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/logs/${logId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            // Reload logs to reflect the deletion
            await loadLogs();
            
            // Clear source code if the deleted log was selected
            if (selectedLogId === logId) {
                selectedLogId = null;
                sourceCode.innerHTML = '<code>Select a log entry to view source code</code>';
                sourceInfo.textContent = '';
                sourceSelector.classList.add('hidden');
            }
            
            showStatus('Log deleted successfully', 'success');
        } else {
            showStatus('Error deleting log', 'error');
        }
    } catch (error) {
        showStatus('Error deleting log: ' + error.message, 'error');
    }
}

function showMainInterface() {
    mainInterface.classList.remove('hidden');
}

function showStatus(message, type) {
    uploadStatus.textContent = message;
    uploadStatus.className = `status-message ${type}`;
    
    // Auto-hide success messages after 3 seconds
    if (type === 'success') {
        setTimeout(() => {
            uploadStatus.textContent = '';
            uploadStatus.className = 'status-message';
        }, 3000);
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Utility function to refresh logs (can be called from console for debugging)
window.refreshLogs = loadLogs;
