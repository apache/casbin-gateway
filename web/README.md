# Casbin Gateway web UI

The management UI, built with [Vite](https://vite.dev), React, TypeScript, Tailwind CSS and
[shadcn/ui](https://ui.shadcn.com). It talks to the Go backend over the `/api` endpoints.

## Running it

```bash
cd web && yarn install && yarn dev
```

That serves the UI on http://localhost:16002 with hot reload, and proxies `/api` and `/v1` to the
backend on port 17000, so the backend has to be running too. Point the proxy somewhere else with
`VITE_BACKEND_URL=http://host:port yarn dev`.

Because the dev server proxies rather than calling an absolute URL, the browser only ever sees one
origin: the beego session cookie is stored and replayed with no CORS or SameSite special cases, in
development exactly as in production.

## Building it

```bash
cd web && yarn build
```

The output lands in `web/build`, which is where the backend looks for the compiled UI — see
`webui.BuildDir`.

`yarn lint` runs ESLint and `yarn fix` applies what it can fix. `yarn build` type-checks first, so a
type error fails the build.

## Layout

| Path                 | What lives there                                                            |
| -------------------- | --------------------------------------------------------------------------- |
| `src/backend/`       | One typed module per REST resource; `request.ts` is the single `fetch` call   |
| `src/pages/`         | One component per route, mounted in `src/App.tsx`                            |
| `src/components/ui/` | shadcn/ui primitives, generated with the CLI settings in `components.json`   |
| `src/components/`    | Shared pieces: `DataTable`, `FormRow`, the agent and provider cards, charts  |
| `src/locales/`       | English and Chinese strings                                                  |
| `src/types.ts`       | The shapes the Go API returns, including its `{status, msg, data, data2}` envelope |

Translations are addressed as `i18next.t("general:Name")`, where the part before the colon is a
top-level key of `locales/*/data.json`.

## Adding a shadcn component

```bash
npx shadcn@latest add <component>
```

`components.json` already points the CLI at `src/components/ui`, the `@/` alias and the "new-york"
style, so a generated component needs no further edits.
