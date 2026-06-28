# TLS

The operator can configure TLS on three hops of a `LiteLLMInstance`:

1. **Serving HTTPS** — the proxy terminates TLS itself with an operator-mounted certificate.
2. **Outbound CA trust** — the proxy trusts a custom CA when calling model providers and logging callbacks (e.g. Langfuse).
3. **Outbound client certificate (mTLS)** — the proxy presents a client cert to upstreams.

Plus **PostgreSQL TLS** for the database connection, and a generic **escape hatch** (`extraVolumes` / `extraVolumeMounts` / `extraEnvVars`) for anything not covered by a typed field.

All references are Kubernetes Secrets and accept cert-manager's standard keys: `tls.crt` / `tls.key` for certificate Secrets, `ca.crt` for CA bundles.

## Serving HTTPS

Point `spec.tls.serverCertSecretRef` at a `kubernetes.io/tls` Secret (for example one issued by cert-manager). The operator mounts it and sets both `SSL_CERTFILE_PATH` and `SSL_KEYFILE_PATH`, so uvicorn serves HTTPS on the proxy port (4000).

```yaml
spec:
  tls:
    serverCertSecretRef:
      name: gateway-server-tls   # contains tls.crt + tls.key
```

When serving HTTPS:

- The container's liveness/readiness/startup probes automatically switch to the `HTTPS` scheme so the handshake succeeds.
- The internal `PROXY_BASE_URL` (used by the SSO flow) becomes `https://`.
- **Clients must use `https://`.** Anything in front of the proxy (Service consumers, an Ingress, an OpenShift Route, or an HTTPRoute) must speak HTTPS to the pod. If you terminate TLS at an Ingress instead, you usually do **not** need this field — leave the proxy on plain HTTP behind the Ingress.

## Outbound CA trust

To make the proxy trust a private CA on its **outbound** calls — both model-provider traffic and logging callbacks such as Langfuse — set `spec.tls.trustedCASecretRef`. The operator mounts the bundle and sets `SSL_CERT_FILE` to the mounted path.

```yaml
spec:
  tls:
    trustedCASecretRef:
      name: navique-internal-ca-bundle
      key: ca.crt                # default; override if your key differs
```

`SSL_CERT_FILE` is the documented LiteLLM knob for a custom outbound CA (LiteLLM is Python/httpx — this is **not** `REQUESTS_CA_BUNDLE`). Because it is the process-level OpenSSL default, it is honored by the callback HTTP clients too, which is what makes the Langfuse tracing callback verify a private CA (historically a gap — see [BerriAI/litellm#7046](https://github.com/BerriAI/litellm/issues/7046); verify on your pinned LiteLLM version).

The CA bundle must contain the **full chain** (root + any intermediates).

## Outbound client certificate (mTLS)

To present a client certificate on outbound calls, set `spec.tls.clientCertSecretRef`. The operator mounts the Secret and sets `SSL_CERTIFICATE` to the mounted certificate path.

```yaml
spec:
  tls:
    clientCertSecretRef:
      name: outbound-client-tls  # contains tls.crt + tls.key
```

## PostgreSQL TLS

::: warning Prisma connection-string semantics
LiteLLM talks to Postgres through **Prisma**, whose native connector reads SSL parameters from the **connection string**, not from libpq `PG*` environment variables — and it does **not** accept libpq's `sslmode=verify-full` / `sslrootcert=system` spellings. It uses `sslmode=require` together with `sslaccept=strict` plus `sslrootcert` / `sslcert` / `sslkey` paths.

Because `DATABASE_URL` is supplied via a Secret that the operator does **not** rebuild, the operator cannot inject these parameters for you. `spec.database.tls` therefore only **mounts** the certificate material at deterministic paths; you add the SSL parameters to the `DATABASE_URL` value yourself.
:::

```yaml
spec:
  database:
    tls:
      caSecretRef:
        name: pg-ca               # mounted at /etc/litellm/db-tls/ca/ca.crt
      clientCertSecretRef:        # optional, for Postgres mTLS
        name: pg-client           # mounted at /etc/litellm/db-tls/client/
```

The CA bundle and client cert/key are mounted on **both** the proxy Deployment and the migration Job. Reference the mounted paths in your `DATABASE_URL` Secret:

```
postgresql://user:pass@host:5432/litellm?sslmode=require&sslaccept=strict&sslrootcert=/etc/litellm/db-tls/ca/ca.crt
```

For mutual TLS add `&sslcert=/etc/litellm/db-tls/client/tls.crt&sslkey=/etc/litellm/db-tls/client/tls.key`.

Mounted paths:

| Material | Path |
|---|---|
| DB CA bundle | `/etc/litellm/db-tls/ca/<key>` (default key `ca.crt`) |
| DB client cert | `/etc/litellm/db-tls/client/tls.crt` |
| DB client key | `/etc/litellm/db-tls/client/tls.key` |

## Escape hatch: extra volumes, mounts, env

For anything the typed fields don't cover, mount arbitrary Secrets/ConfigMaps and inject env directly. `extraEnvVars` already exists (and overrides operator-set vars of the same name); `extraVolumes` / `extraVolumeMounts` were added alongside the `tls` block.

```yaml
spec:
  extraEnvVars:
    - name: SSL_VERIFY
      value: "/etc/litellm/tls/ca/ca.crt"
  extraVolumes:
    - name: extra-ca
      secret:
        secretName: another-ca
  extraVolumeMounts:
    - name: extra-ca
      mountPath: /etc/extra-ca
      readOnly: true
```

## Validation

The operator validates the referenced Secrets on each reconcile and emits warning events (it does not block the reconcile):

- `SecretNotFound` — a referenced Secret does not exist.
- `SecretKeyMissing` — a `serverCertSecretRef` / `clientCertSecretRef` is missing `tls.crt` or `tls.key`, or a CA ref is missing its key.

## Full example

```yaml
apiVersion: litellm.palena.ai/v1alpha1
kind: LiteLLMInstance
metadata:
  name: secure-gateway
spec:
  masterKey:
    autoGenerate: true
  database:
    external:
      connectionSecretRef:
        name: litellm-db          # URL includes ?sslmode=require&sslaccept=strict&sslrootcert=/etc/litellm/db-tls/ca/ca.crt
        key: DATABASE_URL
    tls:
      caSecretRef:
        name: pg-ca
  tls:
    serverCertSecretRef:
      name: gateway-server-tls
    trustedCASecretRef:
      name: navique-internal-ca-bundle
      key: ca.crt
```
