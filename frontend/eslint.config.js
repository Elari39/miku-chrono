import js from "@eslint/js";
import vue from "eslint-plugin-vue";
import tseslint from "typescript-eslint";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default tseslint.config(
  { ignores: ["dist/**", "bindings/**", "node_modules/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs["flat/recommended"],
  {
    files: ["**/*.vue"],
    languageOptions: {
      parserOptions: { parser: tseslint.parser },
      // Script blocks run in the browser (Wails webview): DOM globals are
      // real values there, so no-undef must know about them.
      globals: { ...globals.browser },
    },
    rules: {
      // The core no-unused-vars is type-blind; the @typescript-eslint
      // variant (enabled globally) handles it for .vue scripts.
      "no-unused-vars": "off",
    },
  },
  prettier,
  {
    rules: {
      // Single-word SFC names are fine for this app (Modal, Heatmap, …).
      "vue/multi-word-component-names": "off",
      // Handlers are wired to user actions; the app keeps no unused vars.
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
);
