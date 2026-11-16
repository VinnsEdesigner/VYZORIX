import { type UserConfig } from 'vite';

export interface VyzorixViteConfig {
  /** Map of request path prefix to proxy target. */
  proxy?: Record<string, string | { target: string; changeOrigin?: boolean }>;
  /** Extra Vite config (plugins are passed separately). */
  vite?: Omit<UserConfig, 'plugins'>;
  /** Vite plugins, supplied by the consuming app. */
  plugins?: UserConfig['plugins'];
}

export declare function defineViteConfig(config?: VyzorixViteConfig): UserConfig;
