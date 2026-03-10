import { defineConfig } from "vite";
import { resolve } from "node:path";

export default defineConfig({
  root: resolve(process.cwd(), "desktop"),
  build: {
    outDir: resolve(process.cwd(), "desktop-dist"),
    emptyOutDir: true,
  },
  server: {
    port: 1420,
    strictPort: true,
    host: "127.0.0.1",
  },
});
