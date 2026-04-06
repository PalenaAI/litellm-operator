# Database Configuration

LiteLLM requires a PostgreSQL database for state storage. The operator supports three database modes.

## External Database

Connect to an existing PostgreSQL instance:

```bash
kubectl create secret generic litellm-db \
  --from-literal=DATABASE_URL='postgresql://user:pass@host:5432/litellm'
```

```yaml
spec:
  database:
    external:
      connectionSecretRef:
        name: litellm-db
        key: DATABASE_URL
```

## CloudNativePG

Use a [CloudNativePG](https://cloudnative-pg.io/) managed database. The CloudNativePG operator must be installed on the cluster.

```yaml
spec:
  database:
    cloudnativepg:
      clusterName: litellm-pg-cluster
```

The operator reads the connection URL from the CloudNativePG Cluster's status.

### Scheduled Backups (CloudNativePG)

When using CloudNativePG, the operator can create a `ScheduledBackup` CR for automated backups:

```yaml
spec:
  database:
    cloudnativepg:
      clusterName: litellm-pg-cluster
      backup:
        enabled: true
        schedule: "0 2 * * *"   # cron schedule (daily at 2am)
        retention: 7             # number of backups to keep
        method: snapshot         # snapshot or barmanObjectStore
        suspend: false           # pause scheduling without deleting
```

Backup status is reported in `.status.backup`:

```bash
kubectl get li my-gateway -o jsonpath='{.status.backup}'
```

::: info
This requires the CloudNativePG operator to be installed and a properly configured CNPG Cluster with a backup destination (e.g., S3, Azure Blob, GCS). See the [CloudNativePG backup documentation](https://cloudnative-pg.io/documentation/current/backup/) for details.
:::

## Operator-Managed Database

For development and testing, the operator can deploy a simple single-pod PostgreSQL:

```yaml
spec:
  database:
    managed:
      enabled: true
      storageSize: 10Gi
      storageClassName: standard
```

::: warning
The managed database is a single pod with no replication or backup. Use external PostgreSQL or CloudNativePG for production.
:::

## Connection Pool

Configure the database connection pool:

```yaml
spec:
  database:
    connectionPool:
      maxConnections: 20
```

## Database Migrations

The operator runs a migration Job before starting or updating the Deployment:

```yaml
spec:
  database:
    migration:
      enabled: true    # default: true
      timeout: "300s"  # default: 300s
```

The migration Job uses the same LiteLLM image as the Deployment. If migration fails, the operator sets a `DatabaseReady=False` status condition and does not proceed with the Deployment update.
