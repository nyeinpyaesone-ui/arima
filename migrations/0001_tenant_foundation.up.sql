-- ARIMA tenant isolation, audit, idempotency and transactional outbox baseline.
-- Deploy using a least-privilege migration identity; never run migrations at API startup.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE CHECK (length(trim(slug)) BETWEEN 1 AND 100),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenant_records (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    resource_type text NOT NULL CHECK (resource_type ~ '^[a-z][a-z0-9_]{0,63}$'),
    external_id text NOT NULL CHECK (length(trim(external_id)) BETWEEN 1 AND 200),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, resource_type, external_id)
);
CREATE INDEX tenant_records_lookup_idx ON tenant_records (tenant_id, resource_type, updated_at DESC);

CREATE TABLE audit_events (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    actor_subject text NOT NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('accepted', 'denied', 'succeeded', 'failed')),
    correlation_id text NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object'),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);
CREATE INDEX audit_events_timeline_idx ON audit_events (tenant_id, occurred_at DESC);

CREATE TABLE idempotency_keys (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    key text NOT NULL CHECK (length(key) BETWEEN 8 AND 200),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    state text NOT NULL CHECK (state IN ('in_progress', 'completed')),
    response_status integer CHECK (response_status BETWEEN 100 AND 599),
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, key),
    CHECK ((state = 'in_progress' AND response_status IS NULL) OR (state = 'completed' AND response_status IS NOT NULL))
);
CREATE INDEX idempotency_expiry_idx ON idempotency_keys (expires_at);

CREATE TABLE outbox_events (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'published', 'dead_letter')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    last_error_code text,
    PRIMARY KEY (tenant_id, id)
);
CREATE INDEX outbox_pending_idx ON outbox_events (available_at, created_at) WHERE status = 'pending';

-- Tenant context is set transaction-locally by the repository from the verified principal.
-- Missing context matches no tenant rows. FORCE RLS also applies to table owners.
DO $$
DECLARE table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['tenant_records', 'audit_events', 'idempotency_keys', 'outbox_events']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (tenant_id::text = current_setting(''app.tenant_id'', true)) WITH CHECK (tenant_id::text = current_setting(''app.tenant_id'', true))',
            table_name
        );
    END LOOP;
END $$;
