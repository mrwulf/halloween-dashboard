/*
 * Haunted House Sound Trigger (v6)
 *
 * - Pulls PIR trigger odds into a constant.
 * - Uses snprintf() to consolidate and clean up Serial.println calls.
 * - Plays a random MP3 file.
 * - Uses GPIO 25 for BUSY pin.
 */

#include "Arduino.h"
#include "DFRobotDFPlayerMini.h"
#include <WiFi.h>
#include <WebServer.h>

// --- YOUR WI-FI SETTINGS ---
const char* ssid = "not-my-iot";
const char* password = "secretiot";

// --- Web Security ---
const char* secretKey = "pumpkin"; // Change this to your secret password

// --- Pin Definitions ---
#define RXD2 16
#define TXD2 17
#define BUTTON_PIN 23
#define PIR_PIN 22
#define BUSY_PIN 25 // Using the safe GPIO 25

// --- Settings ---
const long cooldownPeriod = 10000; // 10 seconds

// --- Trigger Chance Constant ---
// Set the percentage chance (0-100) that the PIR sensor will trigger.
const int pirTriggerChance = 50; // 50% chance

// --- DFPlayer Workaround ---
// If myDFPlayer.readFileCounts() fails (returns -1), the code will use this value instead.
// Set this to the actual number of MP3 files on your SD card.
// This allows the prop to function even with a one-way communication issue.
const int MANUAL_FILE_COUNT = 1;

// --- Status Codes ---
#define STATUS_READY 0
#define STATUS_BUSY_PLAYING 1
#define STATUS_BUSY_COOLDOWN 2

// --- Global Objects ---
DFRobotDFPlayerMini myDFPlayer;
WebServer server(80);

// --- Global Variables ---
unsigned long lastTriggerTime = 0;
bool buttonWasUp = true;
bool motionWasClear = true;
int totalFiles = 0; // To store the total number of MP3s

// --- Buffer for snprintf ---
char logBuffer[256];

// --- Forward Declarations ---
void handleWebTrigger();
int getDeviceStatus();

void setup() {
  Serial.begin(115200);
  Serial2.begin(9600, SERIAL_8N1, RXD2, TXD2);

  Serial.println();
  Serial.println("DFPlayer Initializing...");

  if (!myDFPlayer.begin(Serial2, /*isACK = */true, /*doReset = */true)) {
    snprintf(logBuffer, sizeof(logBuffer),
      "Unable to begin:\n"
      "1. Please recheck wiring.\n"
      "2. Please insert the SD card.");
    Serial.println(logBuffer);
    while (true); // Halt
  }
  Serial.println("DFPlayer Mini is online.");

  // Show Any DFPlayer Errors
  while (myDFPlayer.available()) {
    uint8_t type = myDFPlayer.readType();
    uint16_t param = myDFPlayer.read();
    if (type == DFPlayerError) {
      snprintf(logBuffer, sizeof(logBuffer), "DFPlayer Event (during init): Type 0x%02X, Param 0x%04X", type, param);
      Serial.println(logBuffer);

      snprintf(logBuffer, sizeof(logBuffer), "DFPlayer Error during init: 0x%04X", param);
      Serial.println(logBuffer);
    }
  }

  Serial.println("DFPlayer Mini is available.");

  // Count files on the SD Card
  totalFiles = myDFPlayer.readFileCounts();
  if (totalFiles <= 0) { // Handles errors (-1) and no files (0)
    snprintf(logBuffer, sizeof(logBuffer), "WARNING: myDFPlayer.readFileCounts() failed (returned %d).", totalFiles);
    Serial.println(logBuffer);
    snprintf(logBuffer, sizeof(logBuffer), "FALLING BACK to manual file count of %d.", MANUAL_FILE_COUNT);
    Serial.println(logBuffer);
    totalFiles = MANUAL_FILE_COUNT;
  } else {
    snprintf(logBuffer, sizeof(logBuffer), "Found %d total MP3 files on SD card.", totalFiles);
    Serial.println(logBuffer);
  }

  pinMode(BUTTON_PIN, INPUT_PULLUP);
  pinMode(PIR_PIN, INPUT);
  pinMode(BUSY_PIN, INPUT); // BUSY pin with 10k pull-up resistor to 3.3V

  randomSeed(analogRead(0));
  myDFPlayer.volume(25);

  Serial.println("Pins and sound configured.");

  // --- Wi-Fi Connection ---
  Serial.println();
  snprintf(logBuffer, sizeof(logBuffer), "Connecting to %s", ssid);
  Serial.println(logBuffer);

  WiFi.begin(ssid, password);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("\nWiFi connected!");

  snprintf(logBuffer, sizeof(logBuffer), "IP Address: %s", WiFi.localIP().toString().c_str());
  Serial.println(logBuffer);

  // --- Web Server Setup ---
  server.on("/trigger", handleWebTrigger);
  server.begin();
  Serial.println("HTTP server started.");

  snprintf(logBuffer, sizeof(logBuffer), "To trigger, visit: http://%s/trigger?key=%s",
           WiFi.localIP().toString().c_str(), secretKey);
  Serial.println(logBuffer);

  Serial.println("---------------------------------");
  Serial.println("Ready!");
}

/**
 * @brief This helper function contains all the logic for triggering a random sound.
 */
void triggerSound() {
  if (totalFiles == 0) {
    Serial.println("TRIGGER FAILED: No files on SD card!");
    return;
  }

  // Pick a random track number from 1 to totalFiles
  int randomTrack = random(1, totalFiles + 1);

  snprintf(logBuffer, sizeof(logBuffer), "TRIGGER! Playing random track %d of %d...", randomTrack, totalFiles);
  Serial.println(logBuffer);

  myDFPlayer.play(randomTrack);
  lastTriggerTime = millis(); // Set the trigger time to "now"
}

/**
 * @brief Central function to check the device's state.
 * @return int - The status code (STATUS_READY, STATUS_BUSY_PLAYING, etc.)
 */
int getDeviceStatus() {
  // 1. Check if sound is playing
  if (digitalRead(BUSY_PIN) == LOW) {
    return STATUS_BUSY_PLAYING;
  }

  // 2. Check if we are in the cooldown period
  // Use >= to ensure cooldown truly finishes before becoming ready
  if (millis() - lastTriggerTime < cooldownPeriod) {
     return STATUS_BUSY_COOLDOWN;
  }

  // 3. If neither of the above, we are ready!
  return STATUS_READY;
}

/**
 * @brief Web handler now uses getDeviceStatus().
 */
void handleWebTrigger() {
  Serial.println("Web request received...");

  if (!server.hasArg("key")) {
    Serial.println("...Request failed: Missing key.");
    server.send(401, "text/plain", "Unauthorized: Missing 'key' parameter.");
    return;
  }

  if (server.arg("key") != secretKey) {
    Serial.println("...Request failed: Invalid key.");
    server.send(403, "text/plain", "Forbidden: Invalid key.");
    return;
  }

  Serial.println("...Key is valid! Checking device status.");
  int currentStatus = getDeviceStatus();

  switch (currentStatus) {
    case STATUS_READY:
      server.send(200, "text/plain", "OK: Sound Triggered!");
      triggerSound();
      break;
    case STATUS_BUSY_PLAYING:
      server.send(503, "text/plain", "BUSY: Sound is already playing.");
      break;
    case STATUS_BUSY_COOLDOWN:
      server.send(429, "text/plain", "BUSY: In cooldown period.");
      break;
  }
}

/**
 * @brief Main loop now uses getDeviceStatus().
 */
void loop() {
  server.handleClient(); // Keep the web server alive

  int currentStatus = getDeviceStatus();

  if (currentStatus != STATUS_READY) {
    // Not ready, so just reset the "was" flags
    buttonWasUp = (digitalRead(BUTTON_PIN) == HIGH);
    motionWasClear = (digitalRead(PIR_PIN) == LOW);
    return;
  }

  // --- If we get here, we are READY. Check for triggers ---

  // 1. CHECK THE BUTTON (100% Trigger)
  bool buttonIsPressed = (digitalRead(BUTTON_PIN) == LOW);
  if (buttonIsPressed && buttonWasUp) {
    triggerSound();
    buttonWasUp = false; // Prevent re-trigger until release
    return; // A trigger has happened
  }
  buttonWasUp = !buttonIsPressed;

  // 2. CHECK THE PIR SENSOR
  bool motionDetected = (digitalRead(PIR_PIN) == HIGH);
  if (motionDetected && motionWasClear) {
    snprintf(logBuffer, sizeof(logBuffer), "New motion detected! Rolling for a %d%% chance...", pirTriggerChance);
    Serial.println(logBuffer);

    // Use the pirTriggerChance constant
    if (random(0, 100) < pirTriggerChance) {
      Serial.println("...Success! Triggering sound.");
      triggerSound();
    } else {
      Serial.println("...Roll failed. Ignoring this motion event.");
    }
    motionWasClear = false; // Prevent re-trigger until no motion
    return; // A motion event was processed
  }
  motionWasClear = !motionDetected;
}