-- Remove authorization policy data only after exporting the tenant role/membership mapping.
DROP TABLE IF EXISTS tenant_memberships;
DROP TABLE IF EXISTS tenant_role_permissions;
DROP TABLE IF EXISTS tenant_roles;
