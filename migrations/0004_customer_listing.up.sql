-- Index for tenant/resource keyset pagination by external ID.
CREATE INDEX tenant_records_external_id_page_idx
    ON tenant_records (tenant_id, resource_type, external_id ASC);
