console.log("Stats page script loaded.");

const totalUsersEl = document.getElementById('total-users');
const totalRechargesEl = document.getElementById('total-recharges');
const triggersTableBodyEl = document.querySelector('#triggers-table tbody');
const statsContainer = document.getElementById('stats-container');
const triggerStatsTableBodyEl = document.querySelector('#trigger-stats-table tbody');

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
        renderTriggerChart(stats.trigger_activations);
        renderActivationsChart(stats.activations_last_hour);

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

function renderTriggerChart(triggers) {
    const ctx = document.getElementById('triggerChart').getContext('2d');
    new Chart(ctx, {
        type: 'bar',
        data: {
            labels: triggers.map(t => t.trigger_name),
            datasets: [{
                label: 'Public',
                data: triggers.map(t => t.public_count),
                backgroundColor: '#bb86fc',
            }, {
                label: 'Admin',
                data: triggers.map(t => t.admin_count),
                backgroundColor: '#ffb74d',
            }]
        },
        options: {
            responsive: true,
            scales: {
                x: {
                    stacked: true,
                },
                y: {
                    stacked: true,
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1 // Ensure integer scale
                    }
                }
            }
        }
    });
}

function renderActivationsChart(activations) {
    const ctx = document.getElementById('activationsLastHourChart').getContext('2d');
    const lastActivation = activations[activations.length - 1];

    new Chart(ctx, {
        type: 'bar',
        data: {
            datasets: [{
                label: 'Public',
                data: activations.map(a => ({ x: a.minute, y: a.public_count })),
                backgroundColor: '#bb86fc',
            }, {
                label: 'Admin',
                data: activations.map(a => ({ x: a.minute, y: a.admin_count })),
                backgroundColor: '#ffb74d',
            }]
        },
        options: {
            responsive: true,
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: 'minute',
                        tooltipFormat: 'h:mm a',
                        displayFormats: {
                            minute: 'h:mm a'
                        }
                    },
                    stacked: true,
                    max: lastActivation.minute, // This fixes the axis extending too far
                    reverse: true, // This puts the most recent time on the left
                },
                y: {
                    stacked: true,
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1 // This fixes the vertical scale
                    }
                }
            }
        }
    });
}

loadStats();