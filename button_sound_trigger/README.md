Project: Interactive Haunted House Sound & Motion Prop

This document outlines the development process, hardware decisions, and code for an interactive prop built around an ESP32 microcontroller. The primary goal is to create a versatile and expandable system for haunted house scares.

Core Functionality

The device is a sound player that can be triggered by multiple inputs: a physical button, a PIR motion sensor, and a password-protected web request over Wi-Fi. It includes logic to prevent re-triggering while a sound is playing or during a configurable "cooldown" period.

Hardware Bill of Materials

### Core Components (v1)
*   **Microcontroller:** ESP32 Development Board (ESP32-WROOM-32 with CP2102 USB chip).
*   **Sound Module:** DFPlayer Mini MP3 Player Module.
*   **Storage:** MicroSD Card (FAT32 formatted).
*   **Physical Trigger:** Momentary Pushbutton.
*   **Motion Trigger:** HC-SR312 PIR (Infrared) Human Sensor.
*   **Wiring:** Solderless Breadboard and Jumper Wires.
*   **Resistors:**
    *   1x 1kΩ Resistor: **CRITICAL**. For logic level shifting between the 5V DFPlayer and 3.3V ESP32.
    *   1x 10kΩ Resistor: **CRITICAL**. As a pull-up resistor for the DFPlayer's BUSY pin.

### "Loud" Expansion Components
*   **Power Supply:** 12V or 24V DC Power Brick (e.g., 24V 6A).
*   **Power Converter:** Buck Converter (e.g., LM2596, MP1584EN, XL4015) to step down the 12V/24V to a safe 5V for the ESP32.
*   **Audio Amplifier:** External Mono Amplifier Module (e.g., TPA3116D2 for high power).
*   **Speaker:** A high-power 4-ohm or 8-ohm speaker (e.g., car audio or bookshelf speaker).

### "Drop & Reel" Motion Expansion Components
*   **Motor:** High-Torque 12V DC Geared Motor (e.g., Car Wiper Motor, **NOT** a worm gear motor).
*   **Motor Driver:** High-Current H-Bridge Motor Driver (e.g., BTS7960 43A). The L298N is insufficient for heavy props.
*   **Sensor:** Limit Switch (for detecting the "up" position).
*   **Mechanical:** Spool/winch to attach to the motor shaft.

Key Design & Wiring Decisions

### 1. Logic Level Shifting (5V vs 3.3V)
*   **Problem:** The ESP32 operates at 3.3V logic, while the DFPlayer Mini operates at 5V. Connecting the DFPlayer's 5V TX pin directly to the ESP32's 3.3V RX pin would destroy the ESP32.
*   **Decision:** A 1kΩ resistor is placed in-line on the DFPlayer TX -> ESP32 RX connection. This resistor limits the current to a safe level, protecting the ESP32 pin while still allowing the data signal to be read correctly. The ESP32 TX -> DFPlayer RX direction is safe and requires no resistor.

### 2. DFPlayer BUSY Pin Signal
*   **Problem:** The BUSY pin on the DFPlayer is "open-drain." It can pull the signal LOW when playing, but it cannot actively drive it HIGH when idle, causing it to "float" and give unreliable readings.
*   **Decision:** A 10kΩ pull-up resistor is connected between the BUSY pin and the 3.3V rail. This ensures the pin is held reliably HIGH when no sound is playing.

### 3. ESP32 GPIO Pin Selection
*   **Problem:** When the BUSY pin was connected to GPIO 21, the pull-up resistor interfered with the ESP32's ability to enter upload mode.
*   **Decision:** The BUSY pin was moved to a "safe" general-purpose pin, GPIO 25, which resolved the upload conflict.

### 4. Powering the System
*   **Problem:** A high-power amplifier (12V/24V) and the ESP32 (5V) need to be powered from a single wall adapter for a clean installation. The ESP32's onboard VIN regulator is a linear regulator that would overheat and fail if fed 12V or 24V.
*   **Decision:** A buck converter is used to efficiently step down the 12V/24V from the main power brick to a stable 5V to power the ESP32 and its peripherals. This is both safe and energy-efficient. A common ground connection between all components is essential to prevent audio noise.

### 5. "Drop & Reel" Mechanism
*   **Problem:** A simple geared motor in reverse is too slow for a surprising "drop" effect.
*   **Decision:** The drop is powered by gravity. The motor driver's Enable (EN) pin is used to disengage ("coast") the motor, allowing the prop to fall freely. The geared motor's torque is then used to reel the prop back up against gravity. A limit switch is used to detect when the prop has returned to the top, allowing the Arduino to stop the motor accurately. A high-current motor driver is required for props weighing more than a few pounds.

## Code Evolution

The project evolved through several versions, adding features and refactoring for clarity.

*   **v1:** Basic button trigger for a single sound.
*   **v2:** Added a PIR motion sensor, a cooldown period, and a 50% trigger chance for the PIR. Introduced the BUSY pin to check the player's status.
*   **v3:** Added a Wi-Fi connection and a WebServer to allow triggering via a simple GET request.
*   **v4:** Added a `secretKey` URL parameter (`?key=...`) to the web trigger for basic security.
*   **v5:** Refactored the code to eliminate duplication. Created a central `getDeviceStatus()` function to check for `READY`, `BUSY_PLAYING`, or `BUSY_COOLDOWN` states.
*   **v6:** Final refactor. Pulled the PIR trigger chance into a constant (`pirTriggerChance`) and replaced numerous `Serial.print` lines with `snprintf` for cleaner, more readable logging. Added logic to play a random MP3 file.
*   **v7:** Fixed compilation error: Replaced `server.has("key")` with `server.hasArg("key")` for correct `WebServer` argument checking.
*   **v8:** Added "SD Card Preparation" section to `README.md` and enhanced DFPlayer Mini initialization and error logging in `setup()`.
*   **v9:** Added "DFPlayer Mini Troubleshooting" section to `README.md` to guide diagnosis of file recognition issues.
*   **v10:** Increased DFPlayer command timeout to 2000ms to improve reliability of commands that require a response, like `readFileCounts()`.
*   **v11:** Implemented a `MANUAL_FILE_COUNT` workaround. If `readFileCounts()` fails, the code now uses a hard-coded value to allow the project to function despite one-way communication issues.
*   **v12:** Added a handler for the default page with a button leading to the trigger.
*   **v13:** Added a 'ready' indicator. The onboard LED (usually GPIO 2) will now light up once the `setup()` function is complete and the device is fully operational.

## SD Card Preparation

The DFPlayer Mini is very particular about how MP3 files are named and organized on the SD card. To ensure your files are recognized:

1.  **Format:** The SD card must be formatted as FAT16 or FAT32.
2.  **Folder Name:** Create a folder named `mp3` (all lowercase) in the root directory of the SD card.
3.  **File Naming:** MP3 files inside the `mp3` folder (or in the root) must be named using a four-digit sequence, starting from `0001.mp3`. For example:
    *   `/mp3/0001.mp3`
    *   `/mp3/0002.mp3`
    *   ...
    *   `/mp3/0255.mp3`

    Files named `1.mp3`, `01.mp3`, or `001.mp3` (without the leading zero to make it four digits) will generally **not** be recognized by the DFPlayer Mini.

## DFPlayer Mini Troubleshooting

A common and frustrating issue is when `myDFPlayer.play()` commands work, but functions that require a response, like `myDFPlayer.readFileCounts()`, return `-1` (timeout). This indicates a one-way communication failure from the DFPlayer's TX pin back to the ESP32's RX pin.

**Workaround:** The code now includes a `MANUAL_FILE_COUNT` constant. If `readFileCounts()` fails, the system will print a warning and fall back to using this manually set number. This allows the prop to remain fully functional for playing sounds. Simply update this constant in the code whenever you add or remove sound files from the SD card.

**Root Cause Diagnosis:** To definitively test the ESP32's serial port, you can perform a "loopback test": disconnect the DFPlayer from GPIO 16/17 and connect a jumper wire directly between GPIO 16 and 17. A simple test sketch can then write data to the TX pin and verify it is received on the RX pin, confirming the ESP32's hardware is working correctly and isolating the problem to the DFPlayer or its connection.

## Rules for AI Assistant

1.  Any code change needs to update the `README.md` so it can stay up to date and relevant.



The project evolved through several versions, adding features and refactoring for clarity.

*   **v1:** Basic button trigger for a single sound.
*   **v2:** Added a PIR motion sensor, a cooldown period, and a 50% trigger chance for the PIR. Introduced the BUSY pin to check the player's status.
*   **v3:** Added a Wi-Fi connection and a WebServer to allow triggering via a simple GET request.
*   **v4:** Added a `secretKey` URL parameter (`?key=...`) to the web trigger for basic security.
*   **v5:** Refactored the code to eliminate duplication. Created a central `getDeviceStatus()` function to check for `READY`, `BUSY_PLAYING`, or `BUSY_COOLDOWN` states.
*   **v6:** Final refactor. Pulled the PIR trigger chance into a constant (`pirTriggerChance`) and replaced numerous `Serial.print` lines with `snprintf` for cleaner, more readable logging. Added logic to play a random MP3 file.
*   **v7:** Fixed compilation error: Replaced `server.has("key")` with `server.hasArg("key")` for correct `WebServer` argument checking.

## Rules for AI Assistant

1.  Any code change needs to update the `README.md` so it can stay up to date and relevant.