# Project: Haunted Maze Control Dashboard

This project provides a web-based control panel for an interactive haunted Halloween maze. It features a responsive UI, dynamic trigger configuration, a token system for public users, and admin-only statistics.

## Technical Stack

- **Backend:** Go (`net/http` standard library)
- **Frontend:** Vanilla HTML, CSS, and JavaScript
- **Persistence (Phase 3):** SQLite
- **Development:** Container-based using Docker to ensure a consistent environment.

## Development Environment

To avoid installing tooling on the host machine, all development is performed inside a Docker container. The project uses a single, multi-stage `Dockerfile` for both development and production builds.

The development server uses `air` for live reloading. Any changes made to Go, HTML, CSS, or JS files will automatically trigger a rebuild and restart of the application.

### Getting Started

This project uses go-task as a command runner to simplify development.

1.  **Run the development container:** This command will build the development container if it doesn't exist (or if source files have changed), then run it with live-reloading.
    ```sh
    task run
    ```

2.  **Update Go Modules:** If you add or remove a dependency in the Go code, run the `tidy` task to update your `go.mod` and `go.sum` files.
    ```sh
    task tidy
    ```

Once running, the web application will be available at http://localhost:8080.

## Deployment

The application is designed to run as a stateless container. To run the container in a production environment (e.g., using Docker, Podman, or Kubernetes), you must provide configuration, secrets, and a persistent volume for the database.

### Environment Variables

The following environment variables can be used to configure the application:

-   **`ADMIN_SECRET_KEY`** (required): This is the secret key required to log in as an admin. It should be a long, random, and unique string.
-   **`PUBLIC_ACCESS_KEY`** (optional): If set, this key is required as a URL parameter (`?access_key=...`) to view the public dashboard. If not set, the dashboard is open to everyone.
-   **`CONTACT_EMAIL`** (optional): If set, this email address will be displayed on the public dashboard and on the "out of tokens" page, inviting users to send feedback.

### Volumes

You must mount the following paths into the container:
-   **/config/config.json** (read-only): This is the main configuration file containing trigger definitions and device secrets. You should create this file based on `config/config.json.example` and mount it into the container.
-   **/data/** (read-write): This directory stores the SQLite database (`dashboard.db`). Mounting this as a volume ensures that your data persists across container restarts.

### Example `docker run`

Here is an example `docker run` command that illustrates how to set the environment variable and mount the necessary volumes. This assumes your `config.json` is in `/path/to/your/config` and you want to store the database in `/path/to/your/data`.

```sh
# Create a directory for your persistent data
mkdir -p /path/to/your/app-data

# Run the container
docker run -d \
  --name haunted-maze-dashboard \
  -p 8080:8080 \
  -v /path/to/your/config/config.json:/config/config.json:ro \
  -v /path/to/your/app-data:/data \
  -e ADMIN_SECRET_KEY="your-super-strong-secret-key" \
  -e PUBLIC_ACCESS_KEY="your-public-access-key" \
  -e CONTACT_EMAIL="your-email@example.com" \
  ghcr.io/your-username/halloween-dashboard:latest
```

### Security

**IMPORTANT:** This project uses multiple types of secrets that should not be shared publicly.

1.  **Admin Secret Key:** The key required to log in as an admin via the login page or the `?admin_key=` URL parameter. This is managed via the `ADMIN_SECRET_KEY` environment variable.

2.  **Public Access Key:** If the `PUBLIC_ACCESS_KEY` environment variable is set, the entire dashboard is protected. Users must provide this key via a URL parameter (`?access_key=...`) to gain access. Users without the key will be shown a public-facing "locked" page with a Halloween countdown.

3.  **Device Secret Keys:** The keys used by the backend to authenticate with Arduino devices. These are stored in `config/config.json`.

To prevent secrets from being committed to Git, this project includes:

-   A `.gitignore` file to ignore `config/config.json`.
-   An example configuration file at `config/config.json.example`.

To set up your local configuration, copy the example file: `cp config/config.json.example config/config.json` and then edit `config/config.json` with your device IPs and secret keys.

## Technical Details

### API Contract (Arduino)

The web application will communicate with the Arduinos via simple HTTP GET requests. The backend will call an endpoint like:

`http://<arduino-ip-address>/trigger?key=<secret-key>`

The backend is responsible for looking up the correct IP address and secret key for the trigger that was activated.

### Configuration

A `config.json` file defines the available triggers. This file is ignored by Git to protect secrets. To get started, copy `config/config.json.example` to `config/config.json` and customize it for your devices.

For detailed information on all available trigger types and their parameters, please see the [Trigger Configuration Details](TRIGGER_DOCS.md).

```json
{
  "triggers": [
    // ... trigger definitions go here ...
  ]
}
```

### Govee documentation for lan access
https://app-h5.govee.com/user-manual/wlan-guide
