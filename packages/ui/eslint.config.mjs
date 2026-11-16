import js from '@eslint/js';
import tseslint from 'typescript-eslint';

/**
 * Lint config for the shared UI package.
 *
 * This package had no ESLint config, so its sources were never linted — the web
 * app's config only covers `apps/VyzoriX_web/src`. The rules mirror the web
 * app's, minus the architecture plugin (that governs the app's layers, not this
 * package).
 */
export default tseslint.config(
  { ignores: ['node_modules', 'dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['src/**/*.ts', 'src/**/*.tsx'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
    },
    rules: {
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }],
      '@typescript-eslint/no-explicit-any': 'warn',
      'no-unused-vars': 'off',
      // A tagged template used purely for its side effect (Emotion's
      // `injectGlobal`) is not a dead expression; allow the form. The
      // typescript-eslint variant is the one enabled by the recommended set.
      '@typescript-eslint/no-unused-expressions': ['error', { allowTaggedTemplates: true }],
    },
  },
);
