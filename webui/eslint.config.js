import js from '@eslint/js'
import prettier from 'eslint-config-prettier'
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript'
import { importX } from 'eslint-plugin-import-x'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'
import tseslint from 'typescript-eslint'

// Layering, lowest to highest: types, services, hooks/components, features, pages, app.
// Each zone lists what the layer must not import.
const forbid = (target, froms) =>
  froms.map((from) => ({
    target: `./src/${target}`,
    from: `./src/${from}`,
    message: `${target} must not import from ${from}. See README "Layering".`,
  }))

const apiClientMessage =
  'Call local backend APIs through the OpenAPI-typed client in "@/services/apiClient" (generated via "make generate"). Do not call fetch or EventSource directly.'

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'src/types/api.gen.ts'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommendedTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      jsxA11y.flatConfigs.recommended,
      importX.flatConfigs.typescript,
    ],
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    settings: {
      'import-x/resolver-next': [createTypeScriptImportResolver()],
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      'import-x/no-cycle': 'error',
      'import-x/no-restricted-paths': [
        'error',
        {
          zones: [
            ...forbid('types', ['services', 'hooks', 'components', 'features', 'pages', 'app']),
            ...forbid('services', ['hooks', 'components', 'features', 'pages', 'app']),
            ...forbid('hooks', ['components', 'features', 'pages', 'app']),
            ...forbid('components', ['features', 'pages', 'app']),
            ...forbid('features', ['pages', 'app']),
            ...forbid('pages', ['app']),
          ],
        },
      ],
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              regex: '^@/features/[^/]+/.+',
              message: 'Import a feature through its index.ts: "@/features/<name>".',
            },
          ],
        },
      ],
    },
  },
  {
    // Local backend calls must go through the generated OpenAPI client.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/services/apiClient.ts', 'src/**/*.test.{ts,tsx}', 'src/test/**'],
    rules: {
      'no-restricted-globals': [
        'error',
        { name: 'fetch', message: apiClientMessage },
        { name: 'EventSource', message: apiClientMessage },
      ],
      'no-restricted-properties': [
        'error',
        { object: 'window', property: 'fetch', message: apiClientMessage },
        { object: 'globalThis', property: 'fetch', message: apiClientMessage },
        { object: 'window', property: 'EventSource', message: apiClientMessage },
        { object: 'globalThis', property: 'EventSource', message: apiClientMessage },
      ],
    },
  },
  {
    files: ['*.js', '*.ts'],
    languageOptions: { globals: globals.node },
  },
  prettier,
)
