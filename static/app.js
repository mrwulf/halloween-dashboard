
const BUILD_ID_KEY = 'haunted-dashboard-build-id';
console.log("Haunted Maze Control panel script loaded.");

const triggersContainer = document.getElementById('triggers-container');
const tokenCountSpan = document.getElementById('token-count');
const adminIndicator = document.getElementById('admin-indicator');
const statsLink = document.getElementById('stats-link');
const loginLink = document.getElementById('login-link');
const logoutLink = document.getElementById('logout-link');
const halloweenFactWrapper = document.getElementById('halloween-fact-wrapper');
const adminQrSection = document.getElementById('admin-qr-section');

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
            generateQrCodes(); // Generate QR codes for admin
            removeHalloweenFact(); // Admins don't see the fact card
        } else {
            tokenCountSpan.textContent = user.tokens_remaining;
            adminIndicator.style.display = 'none';
            statsLink.style.display = 'none';
            loginLink.style.display = 'inline';
            logoutLink.style.display = 'none';
            if (adminQrSection) adminQrSection.style.display = 'none'; // Hide QR codes for non-admins
            loadAndDisplayHalloweenFact(); // Only show facts for non-admins
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

// Function to load and display the Halloween fact
async function loadAndDisplayHalloweenFact() {
    if (!halloweenFactWrapper) {
        console.error("Halloween fact wrapper not found!");
        return;
    }

    try {
        const response = await fetch('/api/halloween-fact');
        if (!response.ok) return;

        const data = await response.json();
        if (data.fact) {
            const existingCard = document.getElementById('fact-card');
            if (existingCard) existingCard.remove(); // Remove old one if it exists
            const card = document.createElement('div');
            card.className = 'fact-card';
            card.id = 'fact-card';

            const title = document.createElement('h3');
            title.textContent = 'A Spooky Fact';

            const factText = document.createElement('p');
            factText.textContent = data.fact;

            card.appendChild(title);
            card.appendChild(factText);

            halloweenFactWrapper.appendChild(card); // Add it to the dedicated wrapper
        }
    } catch (error) {
        console.error("Failed to load Halloween fact:", error);
    }
}

function removeHalloweenFact() {
    const factCard = document.getElementById('fact-card');
    if (factCard) factCard.remove();
}

async function generateQrCodes() {
    if (!adminQrSection) return;
    adminQrSection.style.display = 'block'; // Show the section

    const baseUrl = window.location.origin;

    // 1. Dashboard URL - fetch public access key to add it if it exists
    try {
        const response = await fetch('/api/admin/public-access-key');
        const data = await response.json();
        const dashboardUrl = new URL(baseUrl);
        if (data.public_access_key) {
            dashboardUrl.searchParams.set('access_key', data.public_access_key);
        }
        const finalUrl = dashboardUrl.toString();
        new QRious({
            element: document.getElementById('qr-dashboard'),
            value: finalUrl,
            size: 200,
        });
        document.getElementById('qr-dashboard').dataset.value = finalUrl;
    } catch (error) {
        console.error("Failed to generate dashboard QR code with access key:", error);
    }

    // 2. Recharge URL
    new QRious({
        element: document.getElementById('qr-recharge'),
        value: `${baseUrl}/api/recharge`,
        size: 200,
    });
    document.getElementById('qr-recharge').dataset.value = `${baseUrl}/api/recharge`;

    // 3. Admin URL (requires fetching the secret)
    try {
        const response = await fetch('/api/admin/secret');
        if (!response.ok) throw new Error('Failed to fetch admin secret');

        const data = await response.json();
        if (data.admin_secret_key) {
            const adminUrl = new URL(baseUrl);
            adminUrl.searchParams.set('admin_key', data.admin_secret_key);

            new QRious({
                element: document.getElementById('qr-admin'),
                value: adminUrl.toString(),
                size: 200,
            });
            document.getElementById('qr-admin').dataset.value = adminUrl.toString();
        }
    } catch (error) {
        console.error("Failed to generate admin QR code:", error);
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

// Event listener for QR code clicks to expand them
adminQrSection.addEventListener('click', (event) => {
    const canvas = event.target.closest('canvas');
    if (!canvas || !canvas.dataset.value) return;

    const qrBox = canvas.parentElement;
    const title = qrBox.querySelector('h3').textContent;
    const value = canvas.dataset.value;

    // Create modal
    const overlay = document.createElement('div');
    overlay.className = 'qr-modal-overlay';
    
    const content = document.createElement('div');
    content.className = 'qr-modal-content';

    const h3 = document.createElement('h3');
    h3.textContent = title;

    const newCanvas = document.createElement('canvas');

    content.appendChild(h3);
    content.appendChild(newCanvas);
    overlay.appendChild(content);
    document.body.appendChild(overlay);

    // Render large QR code
    new QRious({
        element: newCanvas,
        value: value,
        size: Math.min(window.innerWidth, window.innerHeight) * 0.7, // Make it large but fit the screen
    });

    // Click overlay to close
    overlay.addEventListener('click', () => overlay.remove());
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

async function checkBackendVersion() {
    try {
        const response = await fetch('/api/build-id');
        if (!response.ok) return;

        const data = await response.json();
        const currentBuildId = data.build_id;
        const storedBuildId = sessionStorage.getItem(BUILD_ID_KEY);

        if (storedBuildId && storedBuildId !== currentBuildId) {
            console.log('Backend has been updated. Forcing a hard reload to get the latest assets.');
            window.location.reload(true); // true forces a hard reload from the server
        }

        sessionStorage.setItem(BUILD_ID_KEY, currentBuildId);
    } catch (error) {
        console.error("Could not check backend version:", error);
    }
}

// Initial load
function init() {
    checkBackendVersion(); // Check for updates first
    loadTriggers();
    updateUserStatus(); // This now controls whether the fact is loaded
}

init();
