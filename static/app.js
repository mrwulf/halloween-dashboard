
console.log("Haunted Maze Control panel script loaded.");

const triggersContainer = document.getElementById('triggers-container');
const tokenCountSpan = document.getElementById('token-count');
const adminIndicator = document.getElementById('admin-indicator');
const statsLink = document.getElementById('stats-link');
const loginLink = document.getElementById('login-link');
const logoutLink = document.getElementById('logout-link');

// Function to get and display the user's current token count
async function updateUserStatus() {
    if (!tokenCountSpan || !adminIndicator || !statsLink || !loginLink || !logoutLink) return;

    try {
        const response = await fetch('/api/user/status');
        if (!response.ok) {
            tokenCountSpan.textContent = '?';
            return;
        }
        const user = await response.json();

        if (user.is_admin) {
            tokenCountSpan.textContent = 'Unlimited';
            adminIndicator.textContent = 'Admin';
            adminIndicator.style.display = 'inline-block';
            statsLink.style.display = 'inline';
            loginLink.style.display = 'none';
            logoutLink.style.display = 'inline';
        } else {
            tokenCountSpan.textContent = user.tokens_remaining;
            adminIndicator.style.display = 'none';
            statsLink.style.display = 'none';
            loginLink.style.display = 'inline';
            logoutLink.style.display = 'none';
        }

    } catch (error) {
        console.error("Failed to fetch user status:", error);
        tokenCountSpan.textContent = '?';
    }
}

// Function to activate a trigger
async function activateTrigger(triggerId, button) {
    console.log(`Activating trigger: ${triggerId}`);
    const originalButtonText = button.textContent;
    button.disabled = true;
    button.textContent = 'ACTIVATING...';

    // Optimistic UI update for token count
    let tokenUpdated = false;
    const currentTokens = parseInt(tokenCountSpan.textContent, 10);
    if (!isNaN(currentTokens) && currentTokens > 0) {
        // Only update if it's a number (i.e., not an admin)
        tokenCountSpan.textContent = currentTokens - 1;
        tokenUpdated = true;
    }

    try {
        const response = await fetch(`/api/activate/${triggerId}`, {
            method: 'POST',
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Server error: ${response.status} - ${errorText}`);
        }

        button.textContent = 'ACTIVATED!';

    } catch (error) {
        console.error("Failed to activate trigger:", error);
        button.textContent = 'FAILED!';
        // On failure, the backend will refund the token.
        // We call updateUserStatus() to get the corrected count from the server.
        // This handles both trigger failures and out-of-token errors.
        updateUserStatus();
    } finally {
        // Reset the button after a short delay
        setTimeout(() => {
            button.textContent = originalButtonText;
            button.disabled = false;
        }, 2000);
    }
}

// Main function to load triggers
async function loadTriggers() {
    if (!triggersContainer) {
        console.error("Trigger container not found!");
        return;
    }

    try {
        const response = await fetch('/api/triggers');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const triggers = await response.json();

        triggersContainer.innerHTML = ''; // Clear existing content

        triggers.forEach(trigger => {
            const card = document.createElement('div');
            card.className = 'trigger-card';

            const name = document.createElement('h2');
            name.textContent = trigger.name;

            const description = document.createElement('p');
            description.textContent = trigger.description;

            const button = document.createElement('button');
            button.className = 'trigger-button';
            button.textContent = `Activate`;
            button.dataset.triggerId = trigger.id;

            card.appendChild(name);
            card.appendChild(description);
            card.appendChild(button);

            triggersContainer.appendChild(card);
        });

    } catch (error) {
        console.error("Failed to load triggers:", error);
        triggersContainer.innerHTML = '<p>Could not load triggers. Please try again later.</p>';
    }
}

// Use event delegation for button clicks
triggersContainer.addEventListener('click', (event) => {
    const button = event.target.closest('.trigger-button');
    if (button) {
        const triggerId = button.dataset.triggerId;
        activateTrigger(triggerId, button);
    }
});

// Event listener for logout
logoutLink.addEventListener('click', async (event) => {
    event.preventDefault();
    console.log("Logging out...");

    try {
        const response = await fetch('/api/admin/logout', { method: 'POST' });
        if (!response.ok) {
            throw new Error('Logout failed.');
        }
        // On successful logout, the cookie is cleared by the server. Reload the page to get a new non-admin session.
        window.location.reload();
    } catch (error) {
        console.error(error);
    }
});
// Initial load
function init() {
    loadTriggers();
    updateUserStatus();
}

init();
