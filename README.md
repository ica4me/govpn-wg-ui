# GoVPN WireGuard Console

A self-hosted Go + Templ + HTMX console for `govpn-wg`, with locally embedded Tailwind CSS and local scripts. The blue sidebar, Segoe UI typography, rounded controls and white tables follow the supplied VPNaaS reference. No CDN, browser JWT storage, privileged container, or host network-administration tooling is used by the UI.

## Run

```sh
cd /root/govpn-wg-ui
go build -o bin/govpn-wg-ui ./cmd/ui
./bin/govpn-wg-ui
```

Open **http://127.0.0.1:8090**. The backend defaults to `http://127.0.0.1:8080/api/v1`. Sign in with the backend administrator username/password. The fresh backend bootstrap account is `admin` / `admin`, but the UI does **not** create accounts or bypass backend authentication. Change the bootstrap password before production use via the account menu. Passwords are never prefilled or embedded in page source.

### Configuration

| Environment | Default | Meaning |
|---|---|---|
| `GOVPN_API_URL` | `http://127.0.0.1:8080/api/v1` | Trusted backend root, configured server-side only |
| `GOVPN_UI_LISTEN` | `127.0.0.1:8090` | HTTP listen address |
| `GOVPN_COOKIE_SECURE` | `false` | Set `true` when serving the browser over HTTPS |

For access from another machine, use an existing secure tunnel or TLS reverse proxy. **Do not expose plain HTTP publicly.** Behind a TLS proxy preserve the original `Host`, configure `GOVPN_COOKIE_SECURE=true`, and forward to loopback. The application deliberately does not trust forwarded client IP or host headers. It never modifies routes, DNS, firewall, interfaces, SSH, or system configuration itself.

```sh
docker compose up -d --build
```

Compose uses host networking so loopback reaches the backend, runs UID 65532 with a read-only root filesystem, drops ALL capabilities and sets no-new-privileges. It does not mount host files. Default binding remains loopback. Build steps also use host networking, supporting Docker daemons configured without a default bridge. Debian 13 needs `docker.io`, `docker-cli`, and `docker-compose` packages. Docker image build and Compose runtime were verified on Debian 13.

## Features and backend mapping

- Account: `POST /auth/password`; current-password verification, confirmation, CSRF, and forced re-login after success.
- Login: `POST /auth/login`; JWT and its expiry retained only in an in-memory server-side session. The browser receives a random opaque HttpOnly/SameSite=Strict cookie. Session identifier and CSRF token rotate on login; logout destroys the session. Backend token expiry or 401 requires a fresh sign-in. Restarting the UI invalidates all sessions. Single instance only; no shared session store.
- Dashboard/system: `GET /system/status`, refreshed with HTMX every 15 seconds. All fields are represented: endpoint, public interface, LANs, site/client/group counts, online counts, uptime, version, last apply/error, base port, MTU and MSS mode. No fake traffic history or invented metrics.
- Sites: list/detail/create/update/delete via `/sites` and `/sites/{id}`. All input fields supported: name, local_subnets, allow_all_subnets, exposed_subnets, can_access_server_lan, allowed_target_sites, tunnel_mode, port, mtu, enabled and update-only rotate_keys. Multi-site checkbox selection excludes the site itself.
- Groups: list/create/update/delete via `/groups` and `/groups/{id}`. Supports name, port and MTU. Group details come from the list because the backend has no individual group GET. Backend rejects deletion of populated groups.
- Clients: list/detail/create/update/delete via `/clients` and `/clients/{id}`. Supports username, group, target_sites, allow_server_lan, tunnel_mode, dns, enabled and update-only rotate_keys. Username/group are immutable and omitted from updates. Multiple sites are exported as an array, including an explicit empty array when all are cleared.
- Every edit screen includes every returned field in an escaped JSON inspector: allocation, public keys, allowed IPs, runtime endpoint, handshake, traffic counters and timestamps. No private key appears here.
- Site downloads: `conf`, `linux`, `windows`. Backend `windows` is a **.conf**, not an installer; Windows forwarding/NAT remain the operator's responsibility.
- Client downloads: `conf`, `linux`, `qr` (inline PNG). Windows/macOS use `conf`; the UI does not invent a client Windows endpoint. Configuration/QR contain private keys: download over TLS and protect them.
- System maintenance: `POST /system/tune` and `/system/reconcile`, explicit confirmation and CSRF protection required; full backend report shown. These can change kernel/NIC/routes/firewall on the **backend**. Viewing system status never triggers them. Live mutations were intentionally not exercised.
- Search across resource data; responsive tables/sidebar; visible error/empty/loading states; ordinary HTML forms work without HTMX.

Backend validates overlap, addressing, ports, DNS, permission consistency, object existence and resource conflicts. Errors are shown without claiming success. Changes to routes, keys, ports or MTU require redownloading and redeploying configuration.

## Security

CSRF synchronizer tokens protect login, logout, password changes, CRUD and maintenance. Requests with a mismatching Origin or cross-site Fetch Metadata are rejected. Destructive actions require an explicit confirmation field. Credentials/tokens/config files are never logged. Responses use `Cache-Control: no-store`, CSP restricted to local assets, nosniff, frame denial and no-referrer. Backend redirects are not followed, avoiding bearer-token forwarding. Bodies are limited and upstream requests have timeouts. HTML text/attributes are escaped by Templ. Session storage is concurrency-protected and bounded; stale sessions are pruned on creation/access. Use a TLS proxy and perimeter access controls for production; the backend enforces login rate limiting.

## Develop and verify

Generated Templ Go is committed to the working tree, so normal build needs only Go and the module download. To change templates install the pinned compiler:

```sh
go install github.com/a-h/templ/cmd/templ@v0.3.1118
templ generate
gofmt -w cmd internal/console/*.go
go build ./...
go vet ./...
go test -race -cover ./...
```

`internal/console/static/app.css` is the supplied reference's precompiled local Tailwind stylesheet (v4). `console.css` adds the WireGuard console components and responsive styling; `input.css` documents Tailwind's source inputs. No Node process or CSS compiler is needed at runtime.

Tests use isolated `httptest` backend doubles, never the live WireGuard engine. They cover login/rotation/logout, cookie security, expiry, token non-disclosure, all pages, CSRF/Origin rejection, complete CRUD/maintenance dispatch, multi-site serialization, immutable client fields, numeric validation, protected downloads and no GET maintenance side effects.

### Verified in this environment

- `templ generate`: PASS (Templ v0.3.1118).
- `go build ./...`, `go vet ./...`, `go build -o bin/govpn-wg-ui ./cmd/ui`: PASS, exit 0.
- `go test -race -cover ./...`: PASS, initial console coverage **68.4%**, command package 0.0%, no race report (initial output in `verification.txt`; subsequent password/CSRF regression suite also passes).
- Deployed on Debian 13: HTTP login with the bootstrap account and authenticated dashboard/sites/groups/clients pages passed. UI health and route/DNS baseline checks passed repeatedly.
- Chromium login-page rendering inspected after installing local browser libraries.
- Docker Compose build/start passed; container runs non-root, read-only, with host networking and all capabilities dropped.
