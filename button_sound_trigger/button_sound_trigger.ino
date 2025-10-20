/*
 * ESP32 Sound Button Project
 * * This sketch plays a sound from a DFPlayer Mini when a button
 * is pressed. It uses the ESP32's second hardware serial port (Serial2).
 */

#include "Arduino.h"
#include "DFRobotDFPlayerMini.h"

// --- Pin Definitions ---

// Define the pins for Hardware Serial 2 (Serial2)
// ESP32 RX2 pin is GPIO 16
// ESP32 TX2 pin is GPIO 17
#define RXD2 16
#define TXD2 17

// Define the pin the button is connected to
#define BUTTON_PIN 23

// --- Global Objects ---
DFRobotDFPlayerMini myDFPlayer;

void setup() {
  // Start the main Serial port for debugging (so you can see messages)
  Serial.begin(115200);

  // Start Serial2 and connect it to the DFPlayer
  // Note: We use Serial2, as Serial1 is often used by the flash chip.
  Serial2.begin(9600, SERIAL_8N1, RXD2, TXD2);

  Serial.println();
  Serial.println("Initializing DFPlayer ... (May take 3-5 seconds)");

  // Initialize the DFPlayer
  if (!myDFPlayer.begin(Serial2)) {
    Serial.println("Unable to begin:");
    Serial.println("1. Please recheck wiring.");
    Serial.println("2. Please insert the SD card.");
    while (true); // Stay here forever if it fails
  }
  Serial.println("DFPlayer Mini is online.");

  // Set the DFPlayer volume (0~30)
  myDFPlayer.volume(25);  // Set volume to 25 (out of 30)

  // Set the button pin as an input with an internal PULL-UP resistor.
  // This means the pin is HIGH by default, and LOW when pressed.
  pinMode(BUTTON_PIN, INPUT_PULLUP);

  Serial.println("Ready to play. Press the button!");
}

void loop() {
  // Check if the button is pressed
  // digitalRead() will return LOW when the button is pressed
  // because INPUT_PULLUP connects it to GND.
  
  if (digitalRead(BUTTON_PIN) == LOW) {
    Serial.println("Button pressed! Playing sound...");
    
    // Tell the DFPlayer to play the first track (0001.mp3)
    myDFPlayer.play(1);
    
    // "Debounce" delay: Wait for a moment to prevent
    // one long press from triggering the sound hundreds of times.
    delay(500); // Wait half a second
  }
}