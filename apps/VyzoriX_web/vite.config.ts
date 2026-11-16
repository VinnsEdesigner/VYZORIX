import { fixApolloSsrPlugin } from './fix-apollo-ssr-plugin';
import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import react from "@vitejs/plugin-react";
import { defineViteConfig } from "@vyzorix/config/vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineViteConfig({
  plugins: [
    fixApolloSsrPlugin(),
    tanstackStart({ server: { entry: "src/server.ts" } }),
    tailwindcss(),
    react(),
    tsconfigPaths(),
  ] as never,
  vite: {
    // The local store runs SQLite in a Web Worker over OPFS. The worker is
    // created with `new Worker(new URL('./db.worker.ts', import.meta.url))`,
    // which Vite only resolves when it knows the worker format; without this the
    // worker is not emitted and the store silently falls back.
    worker: {
      format: "es",
      plugins: () => [tsconfigPaths()],
    },
    // sqlite3.wasm ships beside the sqlite3 JS. Treat it as an asset so Vite
    // emits it with a hashed URL that survives a non-root base path.
    assetsInclude: ["**/*.wasm"],
    // The wasm module is loaded by the worker at runtime; pre-bundling it on the
    // main thread is wasted work and can break the worker's own import.
    optimizeDeps: {
      exclude: ["@sqlite.org/sqlite-wasm"],
    },
    build: {
      // A worker with a top-level await needs a modern target, and sqlite-wasm
      // uses BigInt internally.
      target: "es2022",
    },
    // The worker must not be bundled into the server build: it touches Worker,
    // navigator.storage and OPFS, none of which exist during SSR.
    ssr: {
      external: ["@sqlite.org/sqlite-wasm"],
    },
  },
  proxy: {
    "/v1": { target: "http://localhost:3000", changeOrigin: true },
    "/api": { target: "http://localhost:3000", changeOrigin: true },
    "/health": { target: "http://localhost:3000", changeOrigin: true },
    "/healthz": { target: "http://localhost:3000", changeOrigin: true },
    "/bin": { target: "http://localhost:3000", changeOrigin: true },
  },
});