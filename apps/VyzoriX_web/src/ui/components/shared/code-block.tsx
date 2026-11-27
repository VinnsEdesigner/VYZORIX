import { useEffect, useState } from 'react';
import { css, cx } from '@emotion/css';
import { useStyles2, useTheme2, type VyzorixTheme } from '@vyzorix/ui';

const LANG_BY_EXTENSION: Record<string, string | null> = {

  toml: 'toml',
  v: 'toml',
  json: 'json',
  jsonc: 'jsonc',
  json5: 'json5',
  yaml: 'yaml',
  yml: 'yaml',
  ini: 'ini',
  cfg: 'ini',
  conf: 'ini',
  properties: 'ini',
  env: 'dotenv',

  md: 'markdown',
  markdown: 'markdown',
  mdx: 'mdx',
  rst: null,
  txt: null,

  sh: 'shellscript',
  bash: 'shellscript',
  zsh: 'shellscript',
  ksh: 'shellscript',
  fish: null,
  ps1: 'powershell',
  psm1: 'powershell',
  bat: 'bat',
  cmd: 'bat',
  dockerfile: 'dockerfile',
  containerfile: 'dockerfile',
  mk: 'makefile',

  go: 'go',
  ts: 'typescript',
  tsx: 'tsx',
  mts: 'typescript',
  cts: 'typescript',
  js: 'javascript',
  jsx: 'jsx',
  mjs: 'javascript',
  cjs: 'javascript',
  css: 'css',
  scss: 'scss',
  sass: 'sass',
  less: 'less',
  html: 'html',
  htm: 'html',
  vue: 'vue',
  svelte: 'svelte',
  astro: 'astro',

  sql: 'sql',
  graphql: 'graphql',
  gql: 'graphql',
  proto: 'proto',
  xml: 'xml',
  svg: 'xml',
  csv: null,

  tf: 'terraform',
  tfvars: 'terraform',
  hcl: 'hcl',
  nginx: 'nginx',

  py: 'python',
  rb: 'ruby',
  php: 'php',
  java: 'java',
  kt: 'kotlin',
  kts: 'kotlin',
  scala: 'scala',
  groovy: 'groovy',
  clj: 'clojure',
  rs: 'rust',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  cc: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  m: 'objective-c',
  swift: 'swift',
  dart: 'dart',
  lua: 'lua',
  pl: 'perl',
  r: 'r',
  ex: 'elixir',
  exs: 'elixir',
  erl: 'erlang',
  hs: 'haskell',

  diff: 'diff',
  patch: 'diff',
};

const LANG_BY_FILENAME: Record<string, string | null> = {
  dockerfile: 'dockerfile',
  containerfile: 'dockerfile',
  makefile: 'makefile',
  gnumakefile: 'makefile',
  'cmakelists.txt': 'makefile',
  '.env': 'dotenv',
  '.gitignore': null,
  '.dockerignore': null,
  '.npmignore': null,
  'go.mod': 'go',
  'go.sum': null,
  '.editorconfig': 'ini',
  '.npmrc': 'ini',
  '.prettierrc': 'json',
  '.eslintrc': 'json',
};

export function languageForPath(path: string): string {
  const name = (path.split('/').pop() ?? '').toLowerCase();
  if (!name) return 'text';

  if (name in LANG_BY_FILENAME) return LANG_BY_FILENAME[name] ?? 'text';

  const dot = name.lastIndexOf('.');

  if (dot <= 0) return 'text';

  return LANG_BY_EXTENSION[name.slice(dot + 1)] ?? 'text';
}

interface CodeBlockProps {
  code: string;

  language: string;
  className?: string;
}

export function CodeBlock({ code, language, className }: CodeBlockProps) {
  const styles = useStyles2(getStyles);
  const theme = useTheme2();
  const [html, setHtml] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  const shikiTheme = theme.isDark ? 'github-dark' : 'github-light';
  const lang = language.includes('.') ? languageForPath(language) : language;

  useEffect(() => {
    let cancelled = false;
    setFailed(false);

    void (async () => {
      try {
        const { codeToHtml } = await import('shiki');
        const rendered = await codeToHtml(code, { lang, theme: shikiTheme });

        if (!cancelled) setHtml(rendered);
      } catch {

        if (!cancelled) setFailed(true);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [code, lang, shikiTheme]);

  if (failed || html === null) {
    return <pre className={cx(styles.plain, className)}>{code}</pre>;
  }

  return <div className={cx(styles.highlighted, className)} dangerouslySetInnerHTML={{ __html: html }} />;
}

const getStyles = (theme: VyzorixTheme) => ({
  plain: css({
    margin: 0,
    padding: theme.spacing(2),
    overflow: 'auto',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.7,
    color: theme.colors.text.primary,
    whiteSpace: 'pre',
  }),
  highlighted: css({
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    overflow: 'auto',
    '& pre': {
      margin: 0,
      padding: theme.spacing(2),
      background: 'transparent !important',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: theme.typography.sm.fontSize,
      lineHeight: 1.7,
    },
  }),
});
