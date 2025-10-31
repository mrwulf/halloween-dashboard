
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
const adminFactWrapper = document.getElementById('admin-fact-wrapper');
const contactBoxWrapper = document.getElementById('contact-box-wrapper');

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
            loadAndDisplayHalloweenFact(true); // Load fact for admin
        } else {
            tokenCountSpan.textContent = user.tokens_remaining;
            adminIndicator.style.display = 'none';
            statsLink.style.display = 'none';
            loginLink.style.display = 'inline';
            logoutLink.style.display = 'none';
            if (adminQrSection) adminQrSection.style.display = 'none'; // Hide QR codes for non-admins
            loadAndDisplayHalloweenFact(false); // Load fact for non-admins
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
            // If the server responds with 403, it means the user is out of tokens.
            // The response body is the "out of tokens" page. Redirect the browser to it.
            if (response.status === 403) {
                window.location.href = "/out-of-tokens.html";
                return;
            }
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

        const publicTriggers = triggers.filter(t => !t.is_admin_only);
        const adminTriggers = triggers.filter(t => t.is_admin_only);

        const renderTrigger = (trigger) => {
            const card = document.createElement('div');
            card.className = 'trigger-card';
            if (trigger.is_admin_only) {
                card.classList.add('admin-trigger');
            }

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
        };

        publicTriggers.forEach(renderTrigger);

        if (adminTriggers.length > 0) {
            const separator = document.createElement('hr');
            separator.className = 'admin-separator';
            triggersContainer.appendChild(separator);
            adminTriggers.forEach(renderTrigger);
        }

    } catch (error) {
        console.error("Failed to load triggers:", error);
        triggersContainer.innerHTML = '<p>Could not load triggers. Please try again later.</p>';
    }
}

// Function to load and display the Halloween fact
async function loadAndDisplayHalloweenFact(isAdmin) {
    if (!halloweenFactWrapper) {
        console.error("Halloween fact wrapper not found!");
        return;
    }

    try {
        const response = await fetch('/api/halloween-fact');
        if (!response.ok) return;

        const data = await response.json();
        const wrapper = isAdmin ? adminFactWrapper : halloweenFactWrapper;
        if (!wrapper) return;

        // Clear previous content from the wrapper
        wrapper.innerHTML = '';

        // 1. Create and add the livestream box if the URL exists
        if (data.livestream_url) {
            const livestreamCard = document.createElement('div');
            livestreamCard.className = 'box'; // Use the generic .box style

            const link = document.createElement('a');
            link.href = data.livestream_url;
            link.textContent = 'Watch the Livestream!';
            link.target = '_blank';
            link.className = 'livestream-link'; // This makes it look like a button

            livestreamCard.appendChild(link);
            wrapper.appendChild(livestreamCard);
        }

        // 2. Create and add the fact box if a fact exists
        if (data.fact) {
            const card = document.createElement('div');
            card.className = 'fact-card';
            card.id = 'fact-card';

            const title = document.createElement('h3');
            title.textContent = 'A Spooky Fact';
            const factText = document.createElement('p');
            factText.textContent = data.fact;

            card.appendChild(title);
            card.appendChild(factText);

            wrapper.appendChild(card);

            // Add contact box if email is provided
            if (data.contact_email) {
                const contactCard = document.createElement('div');
                contactCard.className = 'box contact-box'; // Use .box for styling
                contactCard.id = 'contact-card';

                const contactMsg = document.createElement('p');
                contactMsg.innerHTML = `Have feedback or cool pictures? Send them to <a href="mailto:${data.contact_email}" style="color: #ffb74d;">${data.contact_email}</a>!`;
                
                contactCard.appendChild(contactMsg);
                contactBoxWrapper.appendChild(contactCard); // Add it to the dedicated wrapper at the bottom
            }
        }
    } catch (error) {
        console.error("Failed to load Halloween fact:", error);
    }
}

function removeHalloweenFact() {
    const factCard = document.getElementById('fact-card');
    if (factCard) factCard.remove();
    // Clear the admin wrapper as well
    if (adminFactWrapper) adminFactWrapper.innerHTML = '';
    const contactCard = document.getElementById('contact-card');
    if (contactCard) contactCard.remove(); // Also remove contact card when switching to admin
}

async function generateQrCodes() {
    if (!adminQrSection) return;
    adminQrSection.style.display = 'block'; // Show the section

    try {
        // Fetch all necessary secrets and data in parallel for efficiency
        const [accessKeyRes, secretKeyRes, factRes] = await Promise.all([
            fetch('/api/admin/public-access-key'),
            fetch('/api/admin/secret'),
            fetch('/api/halloween-fact')
        ]);

        if (!accessKeyRes.ok || !secretKeyRes.ok || !factRes.ok) {
            throw new Error("Failed to fetch all data for QR codes");
        }

        const accessKeyData = await accessKeyRes.json();
        const secretKeyData = await secretKeyRes.json();
        const factData = await factRes.json();

        const publicAccessKey = accessKeyData.public_access_key;
        const adminSecretKey = secretKeyData.admin_secret_key;
        const livestreamUrl = factData.livestream_url;
        const baseUrl = window.location.origin;

        // 1. Dashboard URL (with public access key)
        const dashboardUrl = new URL(baseUrl);
        if (publicAccessKey) {
            dashboardUrl.searchParams.set('access_key', publicAccessKey);
        }
        const finalUrl = dashboardUrl.toString();
        new QRious({ element: document.getElementById('qr-dashboard'), value: finalUrl, size: 200 });
        document.getElementById('qr-dashboard').dataset.value = finalUrl;

        // 2. Recharge URL
        const rechargeUrl = `${baseUrl}/api/recharge`;
        new QRious({ element: document.getElementById('qr-recharge'), value: rechargeUrl, size: 200 });
        document.getElementById('qr-recharge').dataset.value = rechargeUrl;

        // 3. Admin Access URL (with BOTH keys)
        if (adminSecretKey) {
            const adminUrl = new URL(baseUrl);
            adminUrl.searchParams.set('admin_key', adminSecretKey);
            if (publicAccessKey) {
                adminUrl.searchParams.set('access_key', publicAccessKey);
            }
            const finalAdminUrl = adminUrl.toString();
            new QRious({ element: document.getElementById('qr-admin'), value: finalAdminUrl, size: 200 });
            document.getElementById('qr-admin').dataset.value = finalAdminUrl;
        }

        // 4. Livestream URL (if it exists)
        if (livestreamUrl) {
            new QRious({ element: document.getElementById('qr-livestream'), value: livestreamUrl, size: 200 });
            document.getElementById('qr-livestream').dataset.value = livestreamUrl;
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
