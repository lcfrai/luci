# Dedicated team HTTPS ingress

Luci 0.5.0 can serve an already commissioned, single-team installation behind an operator-managed TLS reverse proxy. The operator supplies the host, DNS and certificate. This implementation does not provision a domain, issue a certificate, attest host isolation, or prove a real public deployment.

Start with [the managed team installation procedure](TEAM-SERVICE-OPERATIONS.md). Bootstrap the owner and create exactly one team locally, stop the service, and commission its immutable `LUCI_MANAGED_TEAM_ID` binding. Other teams need separate installations, state directories and execution boundaries. The existing Docker distribution keeps ports 8730 and 8731 on host loopback; keep them private when adding ingress.

Configure the service with:

```sh
LUCI_PUBLIC_APP_ORIGIN=https://app.your-control-domain.example
LUCI_PREVIEW_BASE_DOMAIN=preview.your-applications.example
LUCI_BROWSER_PREVIEW_ADDR=127.0.0.1:8731
LUCI_BROWSER_PREVIEW_PORT=8731
```

The container distribution already sets its private gateway listener; retain that listener configuration when using it. `LUCI_PUBLIC_APP_ORIGIN` is the one exact HTTPS origin used for the UI, API CORS and subscription return URLs. It has no trailing slash, credentials, path, query or fragment. The public mode replaces development browser origins with that one origin and admits only its exact Host; forwarded headers never establish identity. A missing managed team binding stops startup. The UI's public content policy removes local-network connection grants.

The preview base must be a lowercase DNS domain on a **different registrable site** from the control UI. `app.example.com` and `preview.example.com` are insufficient: browsers partition cookies by registrable site. Use a separate application domain you control. This keeps a preview opened in the Luci partition distinct from the same address copied into another browsing context. Current membership, project grants, session expiry and service status remain authoritative for HTTP and WebSockets; possession of the address alone never grants team access.

Each private 256-bit registration key is encoded without truncation as `p-<52 lowercase base32 characters>.<preview-base>`. The 54-character first label is DNS compliant, so an ordinary `*.<preview-base>` wildcard certificate covers every app/data origin. Legacy registration files retain their full original key. Refreshing a release or browser connection projects the current URL policy from its owned loopback upstream. Old localhost invitations fail closed; no receipt is treated as a public deployment.

Use wildcard DNS pointing `*.<preview-base>` at the dedicated host and an A/AAAA record for the app hostname. The supplied [Nginx configuration](../../distribution/ingress/nginx.conf.template) terminates TLS and preserves Host and WebSocket upgrades while removing forwarding headers. Unknown TLS names are rejected. Supply a certificate whose SANs cover both the exact app hostname and `*.<preview-base>` plus its private key; keep the key outside repositories, project mounts and backups. A wildcard normally requires an external DNS challenge when obtained through ACME. Luci neither creates that credential nor chooses a certificate authority.

Render only the documented placeholders (the literal Nginx `$http_host` and other variables must remain intact), for example:

```sh
export LUCI_APP_HOST=app.your-control-domain.example
export LUCI_PREVIEW_BASE_DOMAIN=preview.your-applications.example
export LUCI_TLS_CERTIFICATE=/etc/luci-tls/fullchain.pem
export LUCI_TLS_PRIVATE_KEY=/etc/luci-tls/privkey.pem
envsubst '${LUCI_APP_HOST} ${LUCI_PREVIEW_BASE_DOMAIN} ${LUCI_TLS_CERTIFICATE} ${LUCI_TLS_PRIVATE_KEY}' \
  < distribution/ingress/nginx.conf.template > /tmp/luci-nginx.conf
nginx -t -c /tmp/luci-nginx.conf
```

Validate the rendered hostnames, file permissions and private upstreams before installing the configuration using your host's normal service management. Keep port 443 public and the control, preview and Docker daemon ports private. Access logs are disabled because app queries can contain one-use invitations or application tokens. The template adds HSTS only to the served hosts, without domain-wide preload or `includeSubDomains`. Maintain certificate renewal and test it before expiry. Back up private Luci state separately using the operator recovery procedure; certificates and Stripe credentials have separate lifecycles.

Application and data origins remain distinct. The existing one-use, session-bound handoff sets host-only Secure HttpOnly partitioned cookies; its trusted bootstrap connects only the same project's owned data origins. The gateway verifies Docker ownership and allowed data paths/keys, strips Luci control credentials before upstream traffic, and prevents application cookies from escaping to sibling origins. Membership removal closes active registered WebSockets and invalidates browser cookies. Project applications cannot change the ingress domain through an API.

Validation includes Go tests for exact Host/origin admission, full capability round trips, foreign/personal registration rejection and legacy registration reprojection. `node scripts/team-browser-eval.mjs --https` runs Chromium against a local TLS reverse proxy with a fixture-only certificate pin and DNS mappings. It exercises app assets, credentialed data CORS, realtime WebSockets, copied-address isolation and permission revocation. This is implementation evidence, not DNS, public certificate, ingress availability, certificate-renewal or real hosted customer acceptance; those remain operator release gates.

The current source run passed all 11 HTTPS Chromium checks in
`artifacts/uat/team-browser-https/results.json`. The Nginx template also passed
`nginx -t` in the official `nginx:1.28-alpine` image at digest
`sha256:a8b39bd9cf0f83869a2162827a0caf6137ddf759d50a171451b335cecc87d236`,
using a disposable fixture certificate and no container network. These receipts
are development-source checks; repeat the matching installed candidate and
operator domain/certificate acceptance before offering a public service.
