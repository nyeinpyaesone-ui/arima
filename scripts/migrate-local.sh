#!/bin/sh
set -eu

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGPASSWORD:?PGPASSWORD is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${ARIMA_API_DB_PASSWORD:?ARIMA_API_DB_PASSWORD is required}"
: "${ARIMA_OUTBOX_DB_PASSWORD:?ARIMA_OUTBOX_DB_PASSWORD is required}"

psql -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS public.arima_schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

found_migration=false
for migration in /migrations/[0-9]*.up.sql; do
    [ -f "$migration" ] || continue
    found_migration=true
    version="$(basename "$migration" .up.sql)"

    case "$version" in
        [0-9][0-9][0-9][0-9]_[a-z0-9_]*) ;;
        *)
            echo "Invalid migration filename: $version" >&2
            exit 1
            ;;
    esac

    applied="$(psql -X -A -t -v ON_ERROR_STOP=1 -v version="$version" <<'SQL'
SELECT EXISTS (
    SELECT 1
    FROM public.arima_schema_migrations
    WHERE version = :'version'
);
SQL
    )"
    if [ "$applied" = "t" ]; then
        echo "Migration $version already applied; skipping."
        continue
    fi

    echo "Applying migration $version from $migration."
    {
        printf 'BEGIN;\n'
        cat "$migration"
        printf "\nINSERT INTO public.arima_schema_migrations (version) VALUES (:'version');\n"
        printf 'COMMIT;\n'
    } | psql -X -v ON_ERROR_STOP=1 -v version="$version"
    echo "Migration $version applied successfully."
done

if [ "$found_migration" != true ]; then
    echo "No numbered up migrations found in /migrations." >&2
    exit 1
fi

echo "Applying local development runtime grants."
psql -X -v ON_ERROR_STOP=1 \
    -v api_password="$ARIMA_API_DB_PASSWORD" \
    -v worker_password="$ARIMA_OUTBOX_DB_PASSWORD" \
    -f /scripts/grant-runtime-roles.sql

echo "Local migrations and least-privilege role grants completed."
