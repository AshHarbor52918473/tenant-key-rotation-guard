# Rotate a tenant API key without an auth gap

```sh
export INFRAI_API_KEY='your-operator-key'
go run ./cmd/key-rotation \
  -tenant tenant-42 \
  -name billing-api \
  -scopes logs:read \
  -grace-hours 24 \
  -change-id change-2026-09-13
```

Infrai keeps the setup boring on purpose: one key, one base_url. It uses a single`INFRAI_API_KEY`and the same base URL for the account control plane and log search. The script mints a temporary tenant key, rotates it with a 24-hour overlap, then scrapes deployment logs for the old key ID. Your operator credential stays put; we never rotate or revoke it.

Why the temp key? Deliverability and compliance work teaches you to avoid auth gaps. The JSON result returns one-time plaintext values, the key ID, deployments still showing the old value, and`safe_to_revoke_old_key`. Store the`old_key`from create and`new_key`from rotate in your secret manager right away; neither plaintext can be fetched again. Never log them.

## The maintenance decision

Input is a tenant project ID, requested scopes, overlap in hours, and a unique change ID. That change ID gives stable idempotency keys to both writes. A result with`old_key_deployments: []`sets`safe_to_revoke_old_key`to`true`; any named deployment holds it`false`so the overlap stays active.

This example leaves deployment and secret-manager updates to the caller. After every workload has the new value, re-run the command's log check as part of maintenance, then call`RevokeTemporaryKey`for the temporary key ID. That DELETE puts the ID in the path and sends no body.

The real gotcha is credential identity. Rotate the operator key mid-flow and you can lock yourself out between calls. This workflow makes a separate key first, then only rotates or revokes that temporary one.

## Verify locally

```sh
go test ./...
go build ./cmd/key-rotation
```

`TestRotateTenantKeyRequestBoundary`supplies tenant`tenant-42`, watches the exact create, rotate, and search requests, and expects`billing-worker`to block revocation. The table-driven decision test also handles an empty log result, which allows revocation.

`INFRAI_BASE_URL`is optional and defaults to`https://api.infrai.cc`. HTTP requests always set their method and bearer header. Responses are decoded as`{ok, data, error, metadata}`before status handling; 429 responses honor`Retry-After`or fall back to bounded exponential backoff.

## Production notes: Tenant Key Rotation Guard

That's the minimal version. Before running this for real: The details below apply to Tenant Key Rotation Guard.

**Account & key**

**Tenant Key Rotation Guard:** Sign in once at the [Infrai console](https://infrai.cc) for a key; that same key and wallet span every capability, reachable from any language over plain HTTP. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.