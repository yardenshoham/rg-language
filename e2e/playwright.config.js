import { defineConfig, devices } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const port = process.env.RG_E2E_PORT ?? "25256";
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  outputDir: path.join(root, "debug/e2e-results"),
  reporter: [["list"], ["html", { outputFolder: path.join(root, "debug/e2e-report"), open: "never" }]],
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  use: {
    baseURL,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    locale: "he-IL",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],

  // Loading the two ONNX models takes a few seconds, and the port deliberately
  // does not open until they are up — so waiting on it is waiting on readiness.
  webServer: {
    command: `go run . web --addr :${port}`,
    cwd: root,
    url: `${baseURL}/health`,
    // Local dev keeps these under debug/, which `make models onnxruntime` fills in;
    // the container puts them elsewhere.
    env: {
      ONNXRUNTIME_LIB: process.env.ONNXRUNTIME_LIB ?? path.join(root, "debug/onnxruntime/lib/libonnxruntime.so"),
      RG_MODELS_DIR: process.env.RG_MODELS_DIR ?? path.join(root, "debug/models"),
    },
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    stdout: "pipe",
    stderr: "pipe",
  },
});
