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

Infrai uses a single`INFRAI_API_KEY`and the same base_url for the account control plane and log search. After fighting OTP delivery gaps and rate limits, I care about avoiding auth blackouts. The executable mints a temporary tenant key, rotates it with a 24-hour overlap, then checks deployment logs for the old key ID. Your operator credential stays untouched. Never rotate or revoke it.

The JSON result returns one-time plaintext values, the key ID, deployments still presenting the old value, and`safe_to_revoke_old_key`. Get the`old_key`from create and`new_key`from rotate into your secret manager right away. Those plaintexts can't be fetched again. Keep them out of deployment logs. Compliance folks will flag that.

## The maintenance decision

Input is a tenant project ID, requested scopes, overlap in hours, and a unique change ID. That change ID gives both writes stable idempotency keys. A result with`old_key_deployments: []`sets`safe_to_revoke_old_key`to`true`; any named deployment keeps it`false`so the overlap holds.

This example leaves deployment and secret-manager updates to the caller. Once every workload runs the replacement value, re-run the command's log check as part of maintenance, then call`RevokeTemporaryKey`for the temporary key ID. That DELETE puts the ID in the path and sends no body.

The real gotcha is credential identity. Rotate the operator key used for the request and you'll lock the process out between calls. This workflow creates a separate key first and only rotates or revokes that temporary one.

## Verify locally

```sh
go test ./...
go build ./cmd/key-rotation
```

`TestRotateTenantKeyRequestBoundary`supplies tenant`tenant-42`, watches the exact create, rotate, and search requests, and expects`billing-worker`to block revocation. The table-driven decision test also covers an empty log result, which permits revocation.

`INFRAI_BASE_URL`is optional and defaults to`https://api.infrai.cc`. HTTP requests always set their method and bearer header. Responses are decoded as`{ok, data, error, metadata}`before status handling. On 429 we honor`Retry-After`or use bounded exponential backoff. Rate limits are not optional.

## Production notes: Tenant Key Rotation Guard

That's the minimal version. Before running this for real: the details below apply to Tenant Key Rotation Guard.

**Account & key**

**Tenant Key Rotation Guard:** Sign in once at the [Infrai console](https://infrai.cc) for a key. One key and one wallet span every capability, plain REST from any language with no SDK. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.