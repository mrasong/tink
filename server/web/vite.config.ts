import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";
import fs from "node:fs";
import { execSync } from "node:child_process";

function resolveVersionAndBuild() {
  let version = process.env.VITE_APP_VERSION || "";
  if (!version) {
    try {
      const rootVersionFile = fileURLToPath(
        new URL("../../VERSION", import.meta.url),
      );
      if (fs.existsSync(rootVersionFile)) {
        version = fs.readFileSync(rootVersionFile, "utf-8").trim();
      }
    } catch {
      version = "dev";
    }
  }

  let build = process.env.VITE_APP_BUILD || "";
  if (!build) {
    try {
      build = execSync("git rev-parse --short=7 HEAD", {
        stdio: ["ignore", "pipe", "ignore"],
      })
        .toString()
        .trim();
    } catch {
      build = "unknown";
    }
  }

  return { version: version || "0.0.0", build: build || "dev" };
}

const { version: appVersion, build: appBuild } = resolveVersionAndBuild();

// https://vite.dev/config/
export default defineConfig({
  base: "./",
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
    __APP_BUILD__: JSON.stringify(appBuild),
  },
  plugins: [svelte()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://localhost:5021",
    },
  },
});
