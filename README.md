# Project: Haunted Maze Control Dashboard

This document outlines the development plan for a web-based control panel for an interactive haunted Halloween maze. The goal is to create a simple, aesthetically pleasing, and responsive web application that allows guests to trigger lights, sounds, and other effects within the maze.

## Project Phases

### Phase 1: Core Dashboard

The initial phase focuses on creating the core user-facing dashboard.

-   **Responsive UI:** The layout must be mobile-first, but also functional on tablets and laptops.
-   **Theme:** A dark, high-contrast Halloween theme suitable for use in low-light conditions.
-   **Trigger Buttons:** The main interface will consist of buttons. Each button will correspond to a specific trigger in the maze (e.g., "Cackle Witch," "Lightning Strike").
-   **Dynamic Configuration:** The buttons (triggers) should be easily configurable, likely from a simple JSON or YAML file, so that more can be added without changing the code. Each trigger should have a name and a short description.
-   **API Calls:** Clicking a button will make a simple web API call to a predefined endpoint on the corresponding Arduino controlling the effect.
-   **Livestream Link:** A prominent link or embedded view for a YouTube livestream of the maze interior.


### Phase 2: Containerization & Deployment

This phase focuses on packaging and deploying the application.

-   **Minimal Container:** The web application will be packaged into a minimal, efficient Docker container.
-   **Publish to Registry:** The container image will be published to a container registry (e.g., GitHub Container Registry).
-   **Kubernetes Ready:** The deliverable will include basic Kubernetes manifest examples (`Deployment.yaml`, `Service.yaml`) to facilitate deployment into an existing cluster.


### Phase 3: Token & Statistics System

This phase adds gamification and analytics.

-   **Token System:**
    -   New users receive a limited number of tokens (e.g., 10) upon their first visit, identified by a browser cookie.
    -   Each trigger action costs one token.
    -   A special link (`/api/recharge`) allows users to reset their tokens. This can be linked from a QR code placed in the physical maze.
-   **Admin Mode:**
    -   An admin-specific access point or mode that bypasses the token limit for unlimited triggers.
    -   To become an admin, navigate to `/static/login.html` (via the link in the footer) and enter the `SUPER_SECRET` key. This will issue a new admin-level cookie to your browser.
-   **Statistics:**
    -   The system will collect anonymous usage data.
    -   A new admin-only statistics page is available at `/static/stats.html`, with a link appearing in the header for logged-in admins.
    -   Track metrics such as:
        -   Unique user sessions.
        -   Tokens issued/recharged.
        -   Count for each trigger action.
    -   Admin actions are tracked separately from public user actions on the statistics page.

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
