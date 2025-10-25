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

**Important:** `cd` into the `dashboard` directory before running the following commands.

1.  **Build the development container:**
    ```sh
    docker build -t spooky-dashboard:latest -f Dockerfile.dev .
    ```

2.  **Run the development container:**
    ```sh
    docker run --rm -it -p 8080:8080 -v "$(pwd):/app:z" spooky-dashboard:latest
    ```

Once running, the web application will be available at [http://localhost:8080](http://localhost:8080).

## Technical Details

### API Contract (Arduino)

The web application will communicate with the Arduinos via simple HTTP GET requests. The backend will call an endpoint like:

`http://<arduino-ip-address>/trigger?key=<secret-key>`

The backend is responsible for looking up the correct IP address and secret key for the trigger that was activated.

### Configuration

A `config.json` or `config.yaml` file will define the available triggers:

```json
{
  "triggers": [
    {
      "id": "witch_cackle",
      "name": "Witch's Cackle",
      "description": "A terrifying laugh echoes from the darkness.",
      "arduino_ip": "192.168.1.10"
    },
    {
      "id": "lightning_strike",
      "name": "Lightning Strike",
      "description": "A bright flash of light followed by a clap of thunder.",
      "arduino_ip": "192.168.1.11"
    }
  ]
}
```
