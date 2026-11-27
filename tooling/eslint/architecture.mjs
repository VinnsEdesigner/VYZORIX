/**
 * Vyzorix layered-architecture ESLint rules.
 *
 * Dependency flow (inward only):
 *
 *   UI (components, pages, layouts)
 *     -> Hooks (queries, mutations, state)
 *       -> API_Client (domain) | Lib (utilities)
 *
 * Enforced:
 *   - UI may only import from Hooks
 *   - Hooks may import from API_Client (domain, vyzorServer), Hooks, Lib
 *   - API_Client is pure TypeScript (no React, no UI)
 *
 * Plain JS so both apps can import it without a build step. The rules are
 * framework-agnostic; each app mounts them as the `vyzo` plugin.
 */

const LAYERS = {
  ui: {
    name: 'UI Layer',
    canNotImportFrom: ['domain', 'vyzorServer'],
  },
  hooks: {
    name: 'Hooks Layer',
    canNotImportFrom: ['ui'],
  },
  domain: {
    name: 'Domain Layer (API_Client)',
    canNotImportFrom: ['hooks', 'ui', 'vyzorServer', 'react'],
  },
  vyzorServer: {
    name: 'VyzoServer Layer (API_Client)',
    canNotImportFrom: ['hooks', 'ui'],
  },
};

function getFileLayer(filePath) {
  const p = filePath.replace(/\\/g, '/');
  if (p.includes('/src/ui/')) return 'ui';
  if (p.includes('/src/hooks/')) return 'hooks';
  if (p.includes('/packages/API_Client/src/domain/')) return 'domain';
  if (p.includes('/packages/API_Client/src/vyzorServer/')) return 'vyzorServer';
  return null;
}

function getImportLayer(importPath) {
  const p = importPath.replace(/\\/g, '/').replace(/^@\//, '');
  if (p.startsWith('components/') || p.startsWith('ui/') || p.startsWith('pages/')) return 'ui';
  if (p.startsWith('hooks/')) return 'hooks';
  // The bare package specifier is the SDK itself. It must be listed explicitly: matching
  // only `@vyzorix/api-client/domain/...` let a plain `@vyzorix/api-client` import through
  // from a component, which is the same violation spelled differently.
  if (p === '@vyzorix/api-client' || p.startsWith('@vyzorix/api-client/')) return 'domain';
  if (p.startsWith('@vyzorix/api-client/vyzorServer/') || p.startsWith('vyzorServer/')) return 'vyzorServer';
  return null;
}

/** Enforce that imports flow inward through the layers. */
export const layerImportsRule = {
  meta: {
    type: 'problem',
    docs: { description: 'Enforce layered architecture - dependencies must flow inward only' },
    schema: [],
    messages: { forbidden: '{{ fromLayer }} cannot import from {{ toLayer }}. {{ rule }}' },
  },
  create(context) {
    const filename = context.filename ?? context.getFilename();

    return {
      ImportDeclaration(node) {
        const source = node.source.value;
        if (typeof source !== 'string') return;
        if (!source.startsWith('@/') && !source.startsWith('@vyzorix/')) return;

        const fromLayer = getFileLayer(filename);
        if (!fromLayer) return;

        const toLayer = getImportLayer(source);
        if (!toLayer) return;
        if (fromLayer === toLayer) return;

        const fromInfo = LAYERS[fromLayer];
        const toInfo = LAYERS[toLayer];
        if (!fromInfo.canNotImportFrom.includes(toLayer)) return;

        let rule = '';
        if (fromLayer === 'ui' && (toLayer === 'domain' || toLayer === 'vyzorServer')) {
          rule = 'UI must use hooks from @/hooks instead.';
        } else if (fromLayer === 'hooks' && toLayer === 'ui') {
          rule = 'Hooks must be pure logic without UI dependencies.';
        } else if (fromLayer === 'domain' || fromLayer === 'vyzorServer') {
          rule = 'API_Client must be pure TypeScript - no React or UI imports.';
        }

        context.report({
          node,
          messageId: 'forbidden',
          data: { fromLayer: fromInfo.name, toLayer: toInfo.name, rule },
        });
      },
    };
  },
};

/** API_Client cannot import React or UI-related code. */
export const noReactInApiClientRule = {
  meta: {
    type: 'problem',
    docs: { description: 'API_Client package cannot import React or UI-related code' },
    schema: [],
    messages: { forbidden: 'API_Client cannot import {{ package }}. Must be pure TypeScript.' },
  },
  create(context) {
    const filename = context.filename ?? context.getFilename();
    const fileLayer = getFileLayer(filename);
    if (fileLayer !== 'domain' && fileLayer !== 'vyzorServer') return {};

    const forbiddenImports = [
      { pattern: /^react$/, name: 'React' },
      { pattern: /^react-dom/, name: 'react-dom' },
      { pattern: /^react-native$/, name: 'react-native' },
      { pattern: /^@\/components/, name: '@/components' },
      { pattern: /^@\/hooks/, name: '@/hooks' },
      { pattern: /^@\/ui/, name: '@/ui' },
      { pattern: /^@vyzorix\/ui/, name: '@vyzorix/ui' },
    ];

    return {
      ImportDeclaration(node) {
        const source = node.source.value;
        if (typeof source !== 'string') return;
        for (const { pattern, name } of forbiddenImports) {
          if (pattern.test(source)) {
            context.report({ node, messageId: 'forbidden', data: { package: name } });
          }
        }
      },
    };
  },
};

/** The plugin object, mounted as `vyzo` in each app's eslint config. */
export const vyzo = {
  rules: {
    'layer-imports': layerImportsRule,
    'no-react-in-api-client': noReactInApiClientRule,
  },
};

export default vyzo;
