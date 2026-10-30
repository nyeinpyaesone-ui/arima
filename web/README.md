# ARIMA Web Foundation

A React + TypeScript + Vite workspace using the **ARIMA — Precision Operations** design tokens.

## Scope

This is a frontend foundation, not a completed ERP console. The current Go executable implements only:

- `GET /health/live`
- `GET /health/ready`
- `GET /`

The workspace displays those health probes and links to source-controlled API and architecture contracts. It does not expose business workflows, tenant data, authentication controls, or audit operations because those runtime capabilities are not implemented yet.

## Requirements

- Node.js 22+
- npm 10+

## Development

From this directory:

```sh
cp .env.example .env
npm install
npm run dev
```

The Vite development server listens on `http://localhost:5173` and proxies `/health/*` to the Go API. Start the API separately from the repository root:

```sh
go run ./cmd/erp-api
```

Override the proxy target with `ARIMA_API_PROXY_TARGET`.

## Validation and build

```sh
npm run typecheck
npm run build
npm run preview
```

The production output is written to `web/dist`. The deployment layer must serve these static assets and route `/health/*` to the Go API on the same origin (or provide an equivalent reverse-proxy configuration). No production hosting or authentication integration is configured by this foundation. For an Nginx-based same-origin setup, see [deploy/README.md](deploy/README.md) and the upstream-configurable template in `deploy/nginx.conf.template`.

## Design tokens

The CSS custom properties in `src/styles.css` are the source of truth for the current palette, spacing-compatible surface system, borders, radii, and shadow. Keep future components on these tokens rather than introducing isolated colors.
