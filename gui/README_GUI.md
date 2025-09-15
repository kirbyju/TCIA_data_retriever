# GUI — NBIA Data Retriever

This README explains how to run the desktop GUI for the NBIA Data Retriever.

Prerequisite: Build the base command-line application first.

1. Build the base CLI application

   Follow the "Building" section in the top-level `README.md` in the project root to build the `nbia-data-retriever-cli` binary.

2. Start the GUI in development mode

   From the `gui` directory run:

   ```bash
   wails dev
   ```

   This will start the frontend dev server and the Go backend. The GUI will use the dev frontend at `http://localhost:4200` by default.

Notes:
- For a packaged build, run `wails build` inside `gui/` and run the generated binary under `gui/build/bin/`.

