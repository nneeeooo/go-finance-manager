# Go Finance Manager

Self-hosted personal finance tracking system with automated bank statement importers and interactive analytics.

## Tech Stack

* **Backend**: Go 1.24, Gin, GORM, PostgreSQL 17, JWT with refresh token rotation (RFC 6819), UUIDv7.
* **Frontend**: React 19, Vite, Tailwind CSS v4, TanStack (Router, Virtual, Table), Recharts, Zustand.
* **Web Server & Reverse Proxy**: Nginx (multi-stage container, single-origin API proxy, SPA routing).
* **Importers**: Monobank REST API fetcher & PrivatBank XLSX parser with deterministic transaction deduplication.

---

## Architecture

```
                       [ Cloudflare Edge ]
                               │
                        (QUIC Tunnel)
                               ▼
                       [ cloudflared ]
                               │
                          (HTTP :8085)
                               ▼
     ┌──────────────────────────────────────────────────┐
     │           finance-ui (Nginx Container)           │
     │                                                  │
     │  ├── /*       ──> Serves React 19 SPA (HTML/JS)  │
     │  └── /api/*   ──> Reverse proxies to backend     │
     └─────────────────────────┬────────────────────────┘
                               │
                        (HTTP :8080)
                               ▼
     ┌──────────────────────────────────────────────────┐
     │              finance-api (Go Binary)             │
     └─────────────────────────┬────────────────────────┘
                               │
                        (TCP :5432)
                               ▼
     ┌──────────────────────────────────────────────────┐
     │                   PostgreSQL 17                  │
     └──────────────────────────────────────────────────┘
```

---

## Local Development

All backend and frontend services run natively without local Docker. Database runs on the Debian server via Tailscale.

```bash
# 1. Configure environment (points to server PostgreSQL over Tailscale)
cp .env.example .env

# 2. Run Go Backend
cd finance-api
go run ./cmd/api

# 3. Run Frontend (Vite HMR)
cd ../finance-ui-react
npm install
npm run dev
```

* **Web UI**: `http://localhost:5173`
* **API**: `http://localhost:8080/api/v1`

---

## Production Deployment (Debian 13 + Rootless Podman)

The project includes ready-to-use Quadlet files in `deploy/quadlets/`:

1. Copy Quadlet definitions:
   ```bash
   cp deploy/quadlets/* ~/.config/containers/systemd/
   ```
2. Create environment file `~/.config/finance/api.env`:
   ```env
   APP_HOST=0.0.0.0
   APP_PORT=8080
   APP_JWT_SECRET=your_production_secret
   POSTGRES_HOST=postgres
   POSTGRES_PORT=5432
   POSTGRES_DB=finance-db
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=your_postgres_password
   ```
3. Reload systemd:
   ```bash
   systemctl --user daemon-reload
   systemctl --user restart finance-api finance-ui
   ```
4. Point `cloudflared` ingress to `http://localhost:8085`.

---

## Imports

Both importers are part of the API and the web UI (the separate `monobank-importer` and `privatbank-importer` programs are gone).

### Monobank
- **Live:** the Monobank webhook (`POST /api/v1/webhooks/monobank`) saves each purchase and the balance.
- **History:** web UI → Import → Monobank (`POST /api/v1/imports/monobank`): pick the account and the months.
  Monobank allows one statement call a minute, so each month takes about a minute; the page shows progress.
  The personal token is used for that fetch only and never stored; admins may leave it empty to use `MONOBANK_TOKEN`.

### PrivatBank
Web UI → Import → PrivatBank (`POST /api/v1/imports/privatbank`): upload the Privat24 statement `.xlsx`, check the
preview, import. Operation ids are deterministic, so an overlapping statement adds only the new lines.

---

## License

Copyright (c) 2026 Andrii Lytvyn. All Rights Reserved. See [LICENSE](LICENSE) for details.
