# Architecture Decisions

## ADR-001: Go ERP core
**Status:** Accepted for foundation. Go is the primary language for core services and the bridge.

## ADR-002: Contract-first dual transport
**Status:** Accepted. OpenAPI 3.1 describes REST; Protobuf describes gRPC. Compatibility must be tested before APIs are released.

## ADR-003: Transactional outbox
**Status:** Required for business mutations. Do not dual-write to PostgreSQL and brokers/search/analytics systems in a request transaction.

## ADR-004: Tenant and approval boundaries
**Status:** Required. Tenant scope comes from verified identity claims; high-impact actions are policy-gated and may require independent human approval.

## ADR-005: Incremental architecture
**Status:** Accepted. Start with a modular monolith. Extract services only when measured scaling, isolation, or operational needs justify the complexity.
