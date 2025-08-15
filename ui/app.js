class CaptureGatewayUI {
    constructor() {
        this.currentPage = 0;
        this.pageSize = 50;
        this.currentFilters = {};
        this.currentRecord = null;
        this.streamEventSource = null;
        
        this.initializeElements();
        this.bindEvents();
        this.loadTheme();
        this.loadRequests();
    }

    initializeElements() {
        // Filter elements
        this.searchInput = document.getElementById('search');
        this.providerFilter = document.getElementById('provider-filter');
        this.modelFilter = document.getElementById('model-filter');
        this.statusFilter = document.getElementById('status-filter');
        this.clearFiltersBtn = document.getElementById('clear-filters');

        // Table elements
        this.requestsTable = document.getElementById('requests-table');
        this.requestsTbody = document.getElementById('requests-tbody');
        this.totalCount = document.getElementById('total-count');
        this.loading = document.getElementById('loading');

        // Pagination elements
        this.prevPageBtn = document.getElementById('prev-page');
        this.nextPageBtn = document.getElementById('next-page');
        this.pageInfo = document.getElementById('page-info');

        // Modal elements
        this.modal = document.getElementById('detail-modal');
        this.closeModalBtn = document.getElementById('close-modal');
        this.closeModalFooterBtn = document.getElementById('close-modal-btn');
        this.deleteRequestBtn = document.getElementById('delete-request');

        // Tab elements
        this.tabBtns = document.querySelectorAll('.tab-btn');
        this.tabContents = document.querySelectorAll('.tab-content');

        // Other elements
        this.themeToggle = document.getElementById('theme-toggle');
        this.exportBtn = document.getElementById('export-btn');
    }

    bindEvents() {
        // Filter events
        this.searchInput.addEventListener('input', this.debounce(() => this.applyFilters(), 300));
        this.providerFilter.addEventListener('change', () => this.applyFilters());
        this.modelFilter.addEventListener('input', this.debounce(() => this.applyFilters(), 300));
        this.statusFilter.addEventListener('change', () => this.applyFilters());
        this.clearFiltersBtn.addEventListener('click', () => this.clearFilters());

        // Pagination events
        this.prevPageBtn.addEventListener('click', () => this.previousPage());
        this.nextPageBtn.addEventListener('click', () => this.nextPage());

        // Modal events
        this.closeModalBtn.addEventListener('click', () => this.closeModal());
        this.closeModalFooterBtn.addEventListener('click', () => this.closeModal());
        this.deleteRequestBtn.addEventListener('click', () => this.deleteRequest());
        this.modal.addEventListener('click', (e) => {
            if (e.target === this.modal) this.closeModal();
        });

        // Tab events
        this.tabBtns.forEach(btn => {
            btn.addEventListener('click', () => this.switchTab(btn.dataset.tab));
        });

        // Copy button events
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('copy-btn')) {
                this.copyToClipboard(e.target);
            }
        });

        // Stream events
        document.getElementById('play-stream').addEventListener('click', () => this.playStream());
        document.getElementById('stop-stream').addEventListener('click', () => this.stopStream());

        // Other events
        this.themeToggle.addEventListener('click', () => this.toggleTheme());
        this.exportBtn.addEventListener('click', () => this.exportData());

        // Keyboard events
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && !this.modal.classList.contains('hidden')) {
                this.closeModal();
            }
        });
    }

    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    async loadRequests() {
        this.showLoading(true);
        
        try {
            const params = new URLSearchParams({
                offset: this.currentPage * this.pageSize,
                limit: this.pageSize,
                sort: '-ts',
                ...this.currentFilters
            });

            const response = await fetch(`/api/requests?${params}`);
            if (!response.ok) throw new Error('Failed to load requests');
            
            const data = await response.json();
            this.renderRequests(data.records);
            this.updatePagination(data.total, data.offset, data.limit);
            this.updateStats(data.total);
        } catch (error) {
            console.error('Error loading requests:', error);
            this.showError('Failed to load requests');
        } finally {
            this.showLoading(false);
        }
    }

    renderRequests(requests) {
        this.requestsTbody.innerHTML = '';
        
        if (requests.length === 0) {
            const row = document.createElement('tr');
            row.innerHTML = '<td colspan="9" class="text-center">No requests found</td>';
            this.requestsTbody.appendChild(row);
            return;
        }

        requests.forEach(request => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${this.formatDate(request.ts)}</td>
                <td>${request.provider}</td>
                <td>${request.method}</td>
                <td title="${request.url}">${this.truncate(request.url, 40)}</td>
                <td><span class="status-${request.status}">${request.status}</span></td>
                <td>${request.duration_ms}ms</td>
                <td>${request.model_hint || '-'}</td>
                <td>${request.stream ? '<span class="stream-badge">Stream</span>' : '-'}</td>
                <td>
                    <button class="btn btn-primary view-btn" onclick="ui.viewRequest('${request.id}')">
                        View
                    </button>
                </td>
            `;
            this.requestsTbody.appendChild(row);
        });
    }

    async viewRequest(id) {
        try {
            const response = await fetch(`/api/requests/${id}`);
            if (!response.ok) throw new Error('Failed to load request details');
            
            this.currentRecord = await response.json();
            this.showRequestDetails(this.currentRecord);
        } catch (error) {
            console.error('Error loading request details:', error);
            this.showError('Failed to load request details');
        }
    }

    showRequestDetails(record) {
        // Populate overview tab
        document.getElementById('detail-id').textContent = record.id;
        document.getElementById('detail-provider').textContent = record.provider;
        document.getElementById('detail-method').textContent = record.method;
        document.getElementById('detail-url').textContent = record.url;
        document.getElementById('detail-status').textContent = record.status;
        document.getElementById('detail-duration').textContent = `${record.duration_ms}ms`;
        document.getElementById('detail-model').textContent = record.model_hint || '-';
        document.getElementById('detail-req-size').textContent = this.formatBytes(record.size_req_bytes);
        document.getElementById('detail-res-size').textContent = this.formatBytes(record.size_res_bytes);

        // Populate request tab
        document.getElementById('request-body').textContent = this.formatJSON(record.request_body);

        // Populate response tab
        document.getElementById('response-body').textContent = this.formatJSON(record.response_body);

        // Show/hide stream tab
        const streamTab = document.getElementById('stream-tab');
        if (record.stream && record.response_chunks && record.response_chunks.length > 0) {
            streamTab.style.display = 'block';
        } else {
            streamTab.style.display = 'none';
        }

        // Switch to overview tab
        this.switchTab('overview');
        
        // Show modal
        this.modal.classList.remove('hidden');
    }

    switchTab(tabName) {
        // Update tab buttons
        this.tabBtns.forEach(btn => {
            btn.classList.toggle('active', btn.dataset.tab === tabName);
        });

        // Update tab contents
        this.tabContents.forEach(content => {
            const isActive = content.id === `${tabName}-tab` || content.id === `${tabName}-tab-content`;
            content.classList.toggle('active', isActive);
        });
    }

    async deleteRequest() {
        if (!this.currentRecord) return;
        
        if (!confirm('Are you sure you want to delete this request?')) return;

        try {
            const response = await fetch(`/api/requests/${this.currentRecord.id}`, {
                method: 'DELETE'
            });
            
            if (!response.ok) throw new Error('Failed to delete request');
            
            this.closeModal();
            this.loadRequests(); // Reload the list
        } catch (error) {
            console.error('Error deleting request:', error);
            this.showError('Failed to delete request');
        }
    }

    playStream() {
        if (!this.currentRecord || !this.currentRecord.stream) return;

        const playBtn = document.getElementById('play-stream');
        const stopBtn = document.getElementById('stop-stream');
        const streamContent = document.getElementById('stream-content');

        playBtn.disabled = true;
        stopBtn.disabled = false;
        streamContent.textContent = '';

        this.streamEventSource = new EventSource(`/api/requests/${this.currentRecord.id}/chunks`);
        
        this.streamEventSource.onmessage = (event) => {
            streamContent.textContent += event.data;
            streamContent.scrollTop = streamContent.scrollHeight;
        };

        this.streamEventSource.onerror = () => {
            this.stopStream();
        };

        this.streamEventSource.onopen = () => {
            console.log('Stream playback started');
        };
    }

    stopStream() {
        if (this.streamEventSource) {
            this.streamEventSource.close();
            this.streamEventSource = null;
        }

        const playBtn = document.getElementById('play-stream');
        const stopBtn = document.getElementById('stop-stream');
        
        playBtn.disabled = false;
        stopBtn.disabled = true;
    }

    closeModal() {
        this.modal.classList.add('hidden');
        this.currentRecord = null;
        this.stopStream();
    }

    applyFilters() {
        this.currentFilters = {};
        
        if (this.searchInput.value.trim()) {
            this.currentFilters.q = this.searchInput.value.trim();
        }
        
        if (this.providerFilter.value) {
            this.currentFilters.provider = this.providerFilter.value;
        }
        
        if (this.modelFilter.value.trim()) {
            this.currentFilters.modelLike = this.modelFilter.value.trim();
        }
        
        if (this.statusFilter.value) {
            this.currentFilters.status = this.statusFilter.value;
        }

        this.currentPage = 0;
        this.loadRequests();
    }

    clearFilters() {
        this.searchInput.value = '';
        this.providerFilter.value = '';
        this.modelFilter.value = '';
        this.statusFilter.value = '';
        this.currentFilters = {};
        this.currentPage = 0;
        this.loadRequests();
    }

    previousPage() {
        if (this.currentPage > 0) {
            this.currentPage--;
            this.loadRequests();
        }
    }

    nextPage() {
        this.currentPage++;
        this.loadRequests();
    }

    updatePagination(total, offset, limit) {
        const currentPage = Math.floor(offset / limit) + 1;
        const totalPages = Math.ceil(total / limit);
        
        this.pageInfo.textContent = `Page ${currentPage} of ${totalPages}`;
        this.prevPageBtn.disabled = currentPage <= 1;
        this.nextPageBtn.disabled = currentPage >= totalPages;
    }

    updateStats(total) {
        this.totalCount.textContent = `${total} request${total !== 1 ? 's' : ''}`;
    }

    async exportData() {
        try {
            const params = new URLSearchParams(this.currentFilters);
            const response = await fetch(`/api/export.ndjson?${params}`);
            
            if (!response.ok) throw new Error('Failed to export data');
            
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'capture-export.ndjson';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
        } catch (error) {
            console.error('Error exporting data:', error);
            this.showError('Failed to export data');
        }
    }

    copyToClipboard(button) {
        const targetId = button.dataset.copy;
        const targetElement = document.getElementById(targetId);
        
        if (!targetElement) return;
        
        const text = targetElement.textContent;
        navigator.clipboard.writeText(text).then(() => {
            const originalText = button.textContent;
            button.textContent = '✓';
            setTimeout(() => {
                button.textContent = originalText;
            }, 1000);
        }).catch(err => {
            console.error('Failed to copy text:', err);
        });
    }

    toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        
        this.themeToggle.textContent = newTheme === 'dark' ? '☀️' : '🌙';
    }

    loadTheme() {
        const savedTheme = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', savedTheme);
        this.themeToggle.textContent = savedTheme === 'dark' ? '☀️' : '🌙';
    }

    showLoading(show) {
        this.loading.classList.toggle('hidden', !show);
    }

    showError(message) {
        // Simple error display - could be enhanced with a proper notification system
        alert(message);
    }

    formatDate(dateString) {
        const date = new Date(dateString);
        return date.toLocaleString();
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    formatJSON(jsonString) {
        if (!jsonString) return '';
        
        try {
            const parsed = JSON.parse(jsonString);
            return JSON.stringify(parsed, null, 2);
        } catch (e) {
            return jsonString; // Return as-is if not valid JSON
        }
    }

    truncate(str, length) {
        if (str.length <= length) return str;
        return str.substring(0, length) + '...';
    }
}

// Initialize the UI when the page loads
let ui;
document.addEventListener('DOMContentLoaded', () => {
    ui = new CaptureGatewayUI();
});
