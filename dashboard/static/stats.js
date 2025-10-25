console.log("Stats page script loaded.");

const totalUsersEl = document.getElementById('total-users');
const totalRechargesEl = document.getElementById('total-recharges');
const triggersTableBodyEl = document.querySelector('#triggers-table tbody');
const statsContainer = document.getElementById('stats-container');
const triggerChartEl = document.getElementById('trigger-chart');
const triggerStatsTableBodyEl = document.querySelector('#trigger-stats-table tbody');
const timeChartEl = document.getElementById('time-chart');

async function loadStats() {
    try {
        const response = await fetch('/api/stats');

        if (response.status === 403) {
            statsContainer.innerHTML = `
                <h2 class="error">Access Denied</h2>
                <p class="error">You must be an admin to view this page. Please <a href="/login.html">log in</a>.</p>
            `;
            return;
        }

        if (!response.ok) {
            throw new Error(`Server error: ${response.status}`);
        }

        const stats = await response.json();

        totalUsersEl.textContent = stats.total_users;
        totalRechargesEl.textContent = stats.total_recharges;

        renderUserTable(stats.user_stats);
        renderTriggerTable(stats.trigger_activations);
        renderBarChart(triggerChartEl, stats.trigger_activations, 'trigger_name', ['public_count', 'admin_count']);
        renderBarChart(timeChartEl, stats.activations_last_hour, 'minute', ['public_count', 'admin_count']);

    } catch (error) {
        console.error("Failed to load stats:", error);
        statsContainer.innerHTML = `<p class="error">Could not load statistics. Please try again later.</p>`;
    }
}

function renderUserTable(users) {
    triggersTableBodyEl.innerHTML = ''; // Clear previous data
    if (users && users.length > 0) {
        users.forEach(user => {
            const row = `
                <tr>
                    <td>${user.id.substring(0, 8)}...</td>
                    <td>${user.is_admin ? 'Admin' : 'Public'}</td>
                    <td>${user.tokens_used}</td>
                    <td>${new Date(user.created_at).toLocaleString()}</td>
                </tr>`;
            triggersTableBodyEl.innerHTML += row;
        });
    } else {
        triggersTableBodyEl.innerHTML = '<tr><td colspan="4">No user data yet.</td></tr>';
    }
}

function renderTriggerTable(triggers) {
    triggerStatsTableBodyEl.innerHTML = ''; // Clear previous data
    if (triggers && triggers.length > 0) {
        triggers.sort((a, b) => (b.public_count + b.admin_count) - (a.public_count + a.admin_count)); // Sort by total successes

        triggers.forEach(trigger => {
            const totalSuccess = trigger.public_count + trigger.admin_count;
            const row = `
                <tr>
                    <td>${trigger.trigger_name || trigger.trigger_id}</td>
                    <td>${trigger.public_count}</td>
                    <td>${trigger.admin_count}</td>
                    <td class="${trigger.failure_count > 0 ? 'error' : ''}">${trigger.failure_count}</td>
                    <td>${totalSuccess}</td>
                </tr>`;
            triggerStatsTableBodyEl.innerHTML += row;
        });
    } else {
        triggerStatsTableBodyEl.innerHTML = '<tr><td colspan="4">No trigger activation data yet.</td></tr>';
    }
}

function renderBarChart(container, data, labelKey, valueKeys) {
    container.innerHTML = '';
    if (!data || data.length === 0) {
        container.innerHTML = '<p style="text-align: center; width: 100%;">No data available for this chart.</p>';
        return;
    }

    const maxVal = data.reduce((max, item) => {
        const total = valueKeys.reduce((sum, key) => sum + (item[key] || 0), 0);
        return Math.max(max, total);
    }, 0);

    if (maxVal === 0) {
        container.innerHTML = '<p style="text-align: center; width: 100%;">No activity to display.</p>';
        return;
    }

    data.forEach(item => {
        const group = document.createElement('div');
        group.className = 'bar-group';

        const publicVal = item[valueKeys[0]] || 0;
        const adminVal = item[valueKeys[1]] || 0;

        const publicBar = document.createElement('div');
        publicBar.className = 'bar public';
        publicBar.style.height = `${(publicVal / maxVal) * 100}%`;
        publicBar.title = `Public: ${publicVal}`;

        const adminBar = document.createElement('div');
        adminBar.className = 'bar admin';
        adminBar.style.height = `${(adminVal / maxVal) * 100}%`;
        adminBar.title = `Admin: ${adminVal}`;

        const label = document.createElement('div');
        label.className = 'bar-label';
        let labelText = item[labelKey] || 'N/A';
        if (labelKey === 'minute') {
            labelText = new Date(labelText).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        }
        label.textContent = labelText;

        group.appendChild(publicBar);
        group.appendChild(adminBar);
        group.appendChild(label);
        container.appendChild(group);
    });
}

loadStats();