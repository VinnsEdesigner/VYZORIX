// Vite config helper for the Vyzorix apps.
//
// Wraps `defineConfig` with the two things every app needs: a proxy map to the
// Go API, and sourcemaps on. Plugins are passed through untouched.
import { defineConfig } from 'vite';

/**
 * @param {object} config
 * @param {Record<string, string | { target: string, changeOrigin?: boolean }>} [config.proxy]
 * @param {import('vite').UserConfig} [config.vite]  extra Vite config (no `plugins`)
 * @param {import('vite').UserConfig['plugins']} [config.plugins]
 * @returns {import('vite').UserConfig}
 */
export function defineViteConfig(config = {}) {
  const { proxy = {}, vite: viteConfig = {}, plugins } = config;

  const proxyConfig = {};
  for (const [path, target] of Object.entries(proxy)) {
    proxyConfig[path] =
      typeof target === 'string'
        ? { target, changeOrigin: true }
        : { ...target, changeOrigin: target.changeOrigin ?? true };
  }

  return defineConfig({
    ...viteConfig,
    plugins,
    server: {
      proxy: proxyConfig,
      ...viteConfig.server,
    },
    build: {
      sourcemap: true,
      ...viteConfig.build,
    },
  });
}
