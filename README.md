# Mindray Simulator 

This Go program simulates a Mindray bed monitoring system that sends vital signs, waveforms, and alarms to a BedMasterEx server. It can be configured to run on multiple beds within a unit.

## Features

- **Vital Signs and Waveforms:** Simulates the hl7 transmission of vital signs and waveform data from each bed.
- **Alarms:** Optionally sends hl7 alarms based on pre-defined templates.

## Usage

1. **Clone the Repository:** Clone the repository containing the source code.

2. **Set Configuration:**
    - Optionally, set environment variables to customize the configuration:
        - `IP`: IP address of the central server (default: `127.0.0.1`).
        - `PORT`: Port number of the central server (default: `9899`).
        - `BED_COUNT`: Number of beds to simulate (default: `20`).
        - `SEND_ALARMS`: Enable/disable sending alarms (`true` or `false`, default: `true`).
        - `UNIT_NAME`: Name of the unit (default: `LAB`).

3. **Run the Program:** Execute the Go program by running the .exe or by running the Docker image.

    - **Running via Docker:**
        - Build the Docker image using the provided Dockerfile.
        - Run the image to create a Docker container.
        - Pass environment variable to the CLI command or set them via the Docker Desktop GUI.

4. **View Output:**
    - The program will output CLI logs indicating the status of connections and data transmission.
    - If alarms are enabled, simulated alarm messages will also be sent periodically.

## Customization

- **Custom Templates:** You can customize the templates for vital signs, waveforms, and alarms by modifying the template files located in the `Templates` directory.
- **Adding Alarms:** Add new alarms to the `Alarms/AlarmDict.csv` file in the format `AlarmText,AlarmCode`.
    - The AlarmDict csv contains all Mindray Alarms in BedUtils as of May 1, 2024.
