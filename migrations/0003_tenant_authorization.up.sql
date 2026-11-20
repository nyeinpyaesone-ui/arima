-- Tenant-scoped membership and permission policy foundation.
-- Provision roles/memberships through a trusted operator workflow, never public API input.

CREATE TABLE tenant_roles (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    role_key text NOT NULL CHECK (role_key ~ '^[a-z][a-z0-9_]{0,63}$'),
    display_name text NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 100),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_key)
);

CREATE TABLE tenant_role_permissions (
    tenant_id uuid NOT NULL,
    role_key text NOT NULL,
    permission text NOT NULL CHECK (permission ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_key, permission),
    FOREIGN KEY (tenant_id, role_key) REFERENCES tenant_roles(tenant_id, role_key) ON DELETE CASCADE
);

CREATE TABLE tenant_memberships (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    subject text NOT NULL CHECK (length(trim(subject)) BETWEEN 1 AND 255),
    role_key text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject),
    FOREIGN KEY (tenant_id, role_key) REFERENCES tenant_roles(tenant_id, role_key)
);
CREATE INDEX tenant_memberships_subject_idx ON tenant_memberships (tenant_id, subject) WHERE status = 'active';

DO $$
DECLARE table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['tenant_roles', 'tenant_role_permissions', 'tenant_memberships']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (tenant_id::text = current_setting(''app.tenant_id'', true)) WITH CHECK (tenant_id::text = current_setting(''app.tenant_id'', true))',
            table_name
        );
    END LOOP;
END $$;
