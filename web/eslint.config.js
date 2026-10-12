import vue from 'eslint-plugin-vue'
import ts from 'typescript-eslint'

export default ts.config(
  { ignores: ['node_modules/**', '**/*.gen.ts'] },
  ...ts.configs.recommended,
  ...vue.configs['flat/recommended'],
  { files: ['**/*.vue'], languageOptions: { parserOptions: { parser: ts.parser } } },
  { files: ['src/admin/**/*.{ts,vue}'], rules: { 'no-restricted-imports': ['error', { patterns: ['**/site/**'] }] } },
  { files: ['src/site/**/*.{ts,vue}'], rules: { 'no-restricted-imports': ['error', { patterns: ['**/admin/**', '@tiptap/*', 'react', '@excalidraw/*'] }] } },
  { files: ['src/shared/**/*.{ts,vue}'], rules: { 'no-restricted-imports': ['error', { patterns: ['**/admin/**', '**/site/**'] }] } },
)
