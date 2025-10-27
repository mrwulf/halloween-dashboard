# Trigger Configuration Details

The `config.json` file defines all the interactive triggers available in your maze. Each trigger is an object in the `triggers` array with the following common fields:

-   **`id`** (string, required): A unique identifier for the trigger. Used internally by the system.
-   **`name`** (string, required): The display name for the button on the dashboard.
-   **`description`** (string, required): A short explanation of what the trigger does.
-   **`type`** (string, required): Specifies the type of action this trigger performs.
-   **`secret_key`** (string, required for `arduino` type): A secret key used to authenticate with the target device.

Here are the supported `type` values and their specific configuration fields:

#### `arduino` Trigger

This type sends an HTTP GET request to an Arduino or similar micro-controller.

-   **`arduino_ip`** (string, required): The IP address or hostname of the Arduino device.
-   **`secret_key`** (string, required): The secret key expected by the Arduino endpoint.

Example:
```json
{
  "id": "witch_cackle",
  "name": "Witch's Cackle",
  "description": "A terrifying laugh echoes from the darkness.",
  "type": "arduino",
  "arduino_ip": "192.168.1.10",
  "secret_key": "your_arduino_secret"
}
```

#### `govee_lightning` Trigger

This type simulates a lightning storm effect on a Govee light.

-   **`govee_device_ip`** (string, required): The IP address of the Govee device on your local network.
-   **`govee_model`** (string, required): The model number of the Govee device (e.g., "H6076", "H619E").

Example:
```json
{
  "id": "govee_storm_corner",
  "name": "Govee Corner Lightning Storm",
  "description": "Unleash a 10-second lightning storm on the Govee light.",
  "type": "govee_lightning",
  "govee_device_ip": "10.0.20.161",
  "govee_model": "H6076"
}
```

#### `govee_status` Trigger

This type queries the current status of a Govee light and logs it to the server console. Useful for debugging.

-   **`govee_device_ip`** (string, required): The IP address of the Govee device.
-   **`govee_model`** (string, required): The model number of the Govee device.

Example:
```json
{
  "id": "govee_status_check",
  "name": "Govee Status Check",
  "description": "Queries the Govee light and logs its current status to the console.",
  "type": "govee_status",
  "govee_device_ip": "10.0.20.125",
  "govee_model": "H619E"
}
```

#### `govee_set_state` Trigger

This type sets a Govee light to a specific power, brightness, color, or color temperature.

-   **`govee_device_ip`** (string, required): The IP address of the Govee device.
-   **`govee_model`** (string, required): The model number of the Govee device.
-   **`govee_color`** (object, optional): An RGB color object `{ "r": 255, "g": 0, "b": 0 }`. If set, `govee_color_temp` will be ignored.
-   **`govee_color_temp`** (integer, optional): A color temperature in Kelvin (e.g., 2700 for warm white, 6500 for cool white). Only used if `govee_color` is not set.
-   **`govee_brightness`** (integer, optional): Brightness percentage (1-100).

Example:
```json
{
  "id": "set_mood_light_strip",
  "name": "Set The Strip to Purple",
  "description": "Sets the corner light to a static purple color.",
  "type": "govee_set_state",
  "govee_device_ip": "10.0.20.125",
  "govee_model": "H619E",
  "govee_color": { "r": 226, "g": 0, "b": 226 },
  "govee_brightness": 50
}
```