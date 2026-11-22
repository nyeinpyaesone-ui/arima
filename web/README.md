# ARIMA Web Foundation

A React + TypeScript + Vite workspace using the **ARIMA — Precision Operations** design tokens.

## Scope

This is a frontend foundation, not a completed ERP console. The Go API currently implements:

- `GET /health/live` and `GET /health/ready`
- `GET /api/v1/me` for verified caller identity and tenant context
- `POST /api/v1/customers` for tenant-authorized customer creation
- `GET /api/v1/customers/{externalID}` for tenant-authorized customer reads
- `GET /api/v1/customers?limit=&after=` for bounded, tenant-authorized customer listing

The Vite workspace currently displays health probes and links to source-controlled API and architecture contracts. It does not yet provide browser authentication, customer workflows, tenant data views, or audit operations. A real OIDC provider and browser session/token boundary must be selected and implemented before authenticated business screens are added.

## Requirements

- Node.js 22+
- npm 10+

## Development

From this directory:

```sh
cp .env.example .env
npm ci
npm run dev
```

Use `npm ci` to install the exact dependency graph recorded in the committed `package-lock.json`. When intentionally changing dependencies, update the manifest and lockfile together, then commit both.

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

The production output is written to `web/dist`. The deployment layer must serve these static assets and route `/health/*` to the Go API on the same origin (or provide an equivalent reverse-proxy configuration). No production hosting or browser authentication integration is configured by this foundation. For an Nginx-based same-origin setup, see [deploy/README.md](deploy/README.md) and the upstream-configurable template in `deploy/nginx.conf.template`.

## Design tokens

The CSS custom properties in `src/styles.css` are the source of truth for the current palette, spacing-compatible surface system, borders, radii, and shadow. Keep future components on these tokens rather than introducing isolated colors.
