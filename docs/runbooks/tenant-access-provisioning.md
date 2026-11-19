# Tenant access provisioning runbook

## Security model

The OIDC token supplies the immutable identity-provider subject (sub) and tenant claim (tenant_id). ARIMA does not trust a role name or permission list supplied by the client token. Authorization is resolved from the database using:

- tenant_memberships: one subject's active/suspended/revoked membership per tenant.
- tenant_roles: tenant-local role definitions.
- tenant_role_permissions: explicit tenant-local grants.
- Forced RLS on all three tables, using the transaction-local app.tenant_id set by Postgres.WithinTenant.

Customer writes require customer.write; reads require customer.read. The service calls TenantTx.RequirePermission inside the same transaction as the business operation, so a concurrent membership revocation cannot slip between a separate authorization check and the mutation.

## Required migration

Apply migrations 0001, 0002 and 0003 in order with the dedicated migration identity. Do not apply migrations at application startup. Migration 0003 creates the role, permission and membership tables; it does not seed an administrator or default tenant roles.

## Provision a tenant role and membership

Run the following through a trusted, audited operator session after replacing the example tenant UUID and exact OIDC sub. Do not expose this SQL through a public API. Use a migration/tenant-administration identity with controlled access; the API runtime identity must not have INSERT, UPDATE or DELETE privileges on authorization tables.

~~~sql
BEGIN;

INSERT INTO tenant_roles (tenant_id, role_key, display_name)
VALUES ('11111111-1111-4111-8111-111111111111', 'operator', 'Operator')
ON CONFLICT (tenant_id, role_key)
DO UPDATE SET display_name = EXCLUDED.display_name;

INSERT INTO tenant_role_permissions (tenant_id, role_key, permission)
VALUES
  ('11111111-1111-4111-8111-111111111111', 'operator', 'customer.read'),
  ('11111111-1111-4111-8111-111111111111', 'operator', 'customer.write')
ON CONFLICT DO NOTHING;

INSERT INTO tenant_memberships (tenant_id, subject, role_key, status)
VALUES ('11111111-1111-4111-8111-111111111111', 'exact-oidc-subject', 'operator', 'active')
ON CONFLICT (tenant_id, subject)
DO UPDATE SET role_key = EXCLUDED.role_key,
              status = 'active',
              updated_at = now();

COMMIT;
~~~

Grant only the required permissions. Avoid assigning customer.write to read-only users. For suspension/revocation, update tenant_memberships.status to suspended or revoked through the same trusted operator process and retain the change in the operator audit trail.

## Runtime grants

The API runtime role needs SELECT on tenant_roles, tenant_role_permissions, and tenant_memberships, plus only the DML required on tenant business, audit, idempotency and outbox tables. It must be NOSUPERUSER NOBYPASSRLS. Never grant the application runtime role authorization-table write privileges or membership administration rights.

Verify effective grants and role attributes from the actual staging database using catalog queries before exposure. Also verify each RLS table has relrowsecurity=true, relforcerowsecurity=true, and a tenant-isolation policy with both USING and WITH CHECK.

## Operational validation

1. Obtain a valid access token from the selected OIDC provider and confirm sub and tenant_id map to the expected tenant.
2. Verify an active membership with customer.read can read only its own tenant's customer records.
3. Verify a suspended/revoked or missing membership receives 403.
4. Verify a role without customer.write cannot create customers.
5. Verify the same OIDC subject in another tenant cannot inherit permissions from the first tenant.
6. Verify a failed customer mutation leaves no partial customer, audit, idempotency or outbox rows.
7. Verify repeated create requests with the same idempotency key and payload return the original result; a different payload with the same key returns 409.
8. Record operator changes and review membership grants periodically.

No default administrator, self-service membership endpoint, or cross-tenant support override is provided. Build those only with an explicit identity-proofing, approval, audit and break-glass design.
