console.log("Admin login script loaded.");

const loginForm = document.getElementById('login-form');
const errorMessageEl = document.getElementById('error-message');

loginForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    errorMessageEl.textContent = '';

    const key = document.getElementById('admin-key').value;

    try {
        const response = await fetch('/api/admin/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ admin_key: key }),
        });

        if (!response.ok) {
            throw new Error('Invalid secret key.');
        }

        // On success, the backend sets a new cookie. Redirect to the main page.
        window.location.href = '/';

    } catch (error) {
        errorMessageEl.textContent = error.message;
    }
});