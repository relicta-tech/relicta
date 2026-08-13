import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'

// The dashboard had eslint, eslint-plugin-vue, @vue/eslint-config-typescript and an `npm run
// lint` script, and no configuration file — so linting had never run. The script also passed
// --ext and --ignore-path, which eslint 9 removed, so it failed on its flags before it could
// report the missing config. Two layers of "installed but not wired", in the toolchain whose
// job is catching exactly that.
//
// Flat config discovers files itself and takes ignores from here rather than .gitignore,
// which is why those flags are gone rather than translated.
export default defineConfigWithVueTs(
  {
    // Build output and test artifacts. An ignores-only object applies globally.
    ignores: ['dist/**', 'node_modules/**', 'test-results/**', 'playwright-report/**'],
  },

  js.configs.recommended,
  pluginVue.configs['flat/recommended'],

  // Type-aware rules are deliberately not enabled. They need a program built from the app's
  // tsconfig for every file eslint visits, which the config files and Playwright specs are
  // not part of — and a lint that fails on its own configuration is how the previous setup
  // ended up never running. `npm run build` already runs vue-tsc --noEmit over the app, so
  // type errors are caught there rather than twice, in two ways, one of them broken.
  vueTsConfigs.recommended,

  {
    // Routed views are named for the route, not for a tag. vue/multi-word-component-names
    // exists so a component cannot collide with a current or future HTML element, and a
    // view rendered by the router is never written as <Settings> in a template — so the
    // collision it guards against cannot happen here. Scoped to views/ rather than
    // switched off globally, because the rule is right everywhere else.
    files: ['src/views/**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
)
