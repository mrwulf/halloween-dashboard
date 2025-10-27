# Project: Haunted Maze Control Dashboard

This project provides a web-based control panel for an interactive haunted Halloween maze. It features a responsive UI, dynamic trigger configuration, a token system for public users, and admin-only statistics.

## Technical Stack

- **Backend:** Go (`net/http` standard library)
- **Frontend:** Vanilla HTML, CSS, and JavaScript
- **Persistence (Phase 3):** SQLite
- **Development:** Container-based using Docker to ensure a consistent environment.

## Development Environment

To avoid installing tooling on the host machine, all development, building, and testing is performed inside a Docker container defined by `Dockerfile.dev`.

The development server uses `air` for live reloading. Any changes made to Go, HTML, CSS, or JS files will automatically trigger a rebuild and restart of the application. The build process also automatically runs `go mod tidy` to keep Go module dependencies synchronized, so you don't need to manage the `go.sum` file manually.

### Getting Started

This project uses go-task as a command runner to simplify development.

**Important:** `cd` into the `dashboard` directory before running any tasks.

1.  **Build the development container:** (This is done automatically by the `run` task if needed)
    ```sh
    task build
    ```

2.  **Run the development container:** This command will build the container if it doesn't exist, then run it with live-reloading.
    ```sh
    task run
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

**IMPORTANT:** This project uses two types of secrets that should not be shared publicly.

1.  **Admin Secret Key:** The key required to access the admin login page (`/static/login.html`). This is managed via the `ADMIN_SECRET_KEY` environment variable. The `Taskfile.yml` sets a default value for development, but you should use a strong, unique value for any real deployment.

2.  **Device Secret Keys:** The keys used by the backend to authenticate with Arduinos and other devices. These are stored in `config/config.json`.

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

## Project History

*   **v1:** Initial setup with core dashboard, token system, and statistics page.
*   **v2:** Added support for backend-handled Govee light triggers.
*   **v3:** Implemented state-aware Govee triggers that restore the light's original state.
*   **v4:** Added a Taskfile to simplify the build/run process and optimized startup time by caching the Go build.
*   **v5:** Corrected Govee LAN protocol to use JSON and added model-specific (H6076) logic.
*   **v6:** Fixed various build errors related to imports and syntax.
*   **v7:** Simplified Govee lightning trigger to use a direct scene command instead of manual simulation.
*   **v8:** Fixed a build error by removing the unused 'math/rand' import.
*   **v9:** Refactored Govee integration to use the `govee-led-we-go` library, replacing manual protocol implementation.
*   **v10:** Fixed a build error by adding `git` to the development Dockerfile.
*   **v11:** Replaced a deleted Govee library with an active one (`govee-lan-api`) to fix build failures.
*   **v12:** Replaced a non-existent Govee library with a working one (`govee-lan`) to fix build failures.
*   **v13:** Removed all external Govee libraries and reverted to a manual JSON-based implementation to fix build and functionality issues.
*   **v14:** Implemented a state-aware lightning effect simulation after discovering scene commands are unsupported via LAN.
*   **v15:** Added a generic `govee_set_state` trigger to capture and reuse scene states discovered via the status check.
*   **v16:** Improved `govee_set_state` trigger by adding a reset sequence to reliably override active scenes.
*   **v17:** Corrected the `govee_set_state` trigger to remove the power-cycle reset, allowing it to properly re-activate scenes by setting their effective color.
*   **v18:** Enhanced the `govee_set_state` trigger to support activating scenes by a numeric ID.
*   **v19:** Implemented dynamic configuration reloading to apply `config.json` changes without restarting the server.
*   **v20:** Fixed a bug where the wrong Govee light was triggered due to duplicate trigger IDs in `config.json`.
*   **v21:** Refactored Govee triggers to be model-aware, allowing for different commands for different device models (e.g., H619E vs. H6076).
*   **v22:** Reverted all Govee lightning triggers to use the manual simulation due to unreliable scene command support across models.
*   **v23:** Updated the Govee lightning simulation to use the `colorwc` command for better compatibility with segmented light strips like the H619E.
*   **v24:** Corrected the Govee state restoration logic to also use the `colorwc` command, fixing an issue where lights would not revert to their original state.
*   **v25:** Added a "reset" command to the Govee simulation to prime segmented light strips like the H619E before the effect.
*   **v26:** Implemented a more robust power-cycle reset for Govee triggers to reliably exit scene modes on segmented strips like the H619E.
*   **v27:** Added error handling to Govee simulation and restoration commands to improve reliability.
*   **v28:** Improved Govee simulation reliability by adding delays between commands to prevent overwhelming the device.
*   **v29:** Simplified the Govee lightning simulation to only flicker brightness, improving compatibility with sensitive devices like the H619E.
*   **v30:** Implemented a segment-aware lightning simulation for better compatibility with light strips like the H619E.
*   **v31:** Removed the power-cycle reset from the Govee simulation, as it was preventing segment-based commands from working on the H619E.
*   **v32:** Re-introduced model-aware logic to use different lightning simulations for different Govee devices (H6076 vs. H619E).
*   **v33:** Corrected the lightning simulation for the H6076 bulb and added an example for the existing `govee_set_state` trigger to set a static color.
*   **v34:** Made the `govee_set_state` trigger model-aware to use the correct color command for different devices (e.g., `colorwc` for H619E).
*   **v35:** Improved `govee_set_state` reliability by adding delays and a specific command order (on, then brightness, then color).
*   **v36:** Corrected the `govee_set_state` command order to `on -> color -> brightness` for better device compatibility.
*   **v37:** Replaced the manual Govee implementation with a self-contained, community-vetted version to improve reliability.
*   **v38:** Fixed build errors caused by an incomplete refactoring of the Govee implementation.
*   **v39:** Corrected a function call to resolve the final build error from the Govee refactoring.
*   **v40:** Fixed a build error by correcting a struct field name (`IsOn` -> `On`) in a log message.
*   **v41:** Fixed a Govee JSON unmarshal error and corrected the command used for setting static colors.
