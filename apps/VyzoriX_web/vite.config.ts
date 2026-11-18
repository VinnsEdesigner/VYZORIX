import { fixApolloSsrPlugin } from './fix-apollo-ssr-plugin';
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import react from "@vitejs/plugin-react";
import { defineViteConfig } from "../../tooling/vite/config.mjs";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineViteConfig({
  plugins: [
    fixApolloSsrPlugin(),
    tanstackStart({ server: { entry: "src/server.ts" } }),
    react(),
    tsconfigPaths(),
  ] as never,
  vite: {
    server: {
      allowedHosts: ['.prod-runtime.all-hands.dev', 'localhost'],
    },
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
      rollupOptions: {
        output: {
          // Split third-party code into vendor chunks so the app entry stays
          // small and each vendor caches independently across deploys. The entry
          // otherwise bundles every dependency (~800 kB), and a one-line app
          // change re-hashes and re-downloads the lot.
          //
          // A function (not an object): it only re-groups modules already in the
          // graph. The object form force-resolves each named package's entry, and
          // `@tanstack/react-start`'s entry reaches React Native (Flow-typed and
          // unparseable by Rollup) — which fails the build.
          manualChunks(id) {
            if (!id.includes('node_modules')) return undefined;
            if (/node_modules\/(react|react-dom|scheduler)\//.test(id)) return 'react';
            if (id.includes('@apollo') || /node_modules\/graphql(-ws)?\//.test(id)) return 'graphql';
            if (id.includes('lucide-react')) return 'icons';
            return undefined;
          },
        },
      },
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