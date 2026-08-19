module.exports = {
  root: true,
  env: {browser: true, es2022: true},
  parser: "@typescript-eslint/parser",
  parserOptions: {ecmaVersion: "latest", sourceType: "module", ecmaFeatures: {jsx: true}},
  settings: {react: {version: "detect"}},
  plugins: ["@typescript-eslint", "react", "react-hooks"],
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react/recommended",
    "plugin:react/jsx-runtime",
    "plugin:react-hooks/recommended",
  ],
  ignorePatterns: ["build", "dist", "node_modules", "*.cjs"],
  rules: {
    // The house style the rest of the Casbin projects' frontends are linted with.
    quotes: ["error", "double"],
    semi: ["error", "always"],
    indent: ["error", 2, {SwitchCase: 0}],
    "comma-dangle": ["error", "always-multiline"],
    "object-curly-spacing": ["error", "never"],
    "@typescript-eslint/no-explicit-any": "off",
    "@typescript-eslint/no-unused-vars": ["error", {argsIgnorePattern: "^_"}],
    "react/prop-types": "off",
  },
};
