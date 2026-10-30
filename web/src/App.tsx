import { useCallback, useEffect, useState } from "react";

type ProbeName = "live" | "ready";
type ProbeState = "checking" | "healthy" | "unavailable" | "unknown";

type Probe = {
  name: ProbeName;
  state: ProbeState;
  detail?: string;
};

const initialProbes: Probe[] = [
  { name: "live", state: "unknown" },
  { name: "ready", state: "unknown" }
];

const contractLinks = [
  {
    title: "OpenAPI 3.1",
    detail: "HTTP resource and action contract",
    href: "https://github.com/nyeinpyaesone-ui/arima/blob/main/api/openapi/openapi.yaml",
    marker: "REST"
  },
  {
    title: "Protobuf bridge",
    detail: "Typed ERP ↔ AI integration contract",
    href: "https://github.com/nyeinpyaesone-ui/arima/blob/main/api/proto/arima/erp/v1/bridge.proto",
    marker: "gRPC"
  },
  {
    title: "Architecture decisions",
    detail: "Accepted foundation-level decisions",
    href: "https://github.com/nyeinpyaesone-ui/arima/blob/main/docs/architecture/decisions.md",
    marker: "ADR"
  }
];

function stateLabel(state: ProbeState): string {
  switch (state) {
    case "healthy": return "Responding";
    case "unavailable": return "Unavailable";
    case "checking": return "Checking";
    default: return "Not checked";
  }
}

function App() {
  const [probes, setProbes] = useState<Probe[]>(initialProbes);
  const [lastChecked, setLastChecked] = useState<string | null>(null);
  const [isChecking, setIsChecking] = useState(false);

  const checkHealth = useCallback(async () => {
    setIsChecking(true);
    setProbes((current) => current.map((probe) => ({ ...probe, state: "checking", detail: undefined })));
    try {
      const results = await Promise.all(
        (["live", "ready"] as const).map(async (name): Promise<Probe> => {
          try {
            const response = await fetch(`/health/${name}`, {
              headers: { Accept: "application/json" },
              cache: "no-store"
            });
            if (!response.ok) {
              return { name, state: "unavailable", detail: `HTTP ${response.status}` };
            }
            const body: unknown = await response.json();
            if (
              !body || typeof body !== "object" ||
              !("status" in body) || body.status !== name ||
              !("service" in body) || body.service !== "arima-erp-api"
            ) {
              return { name, state: "unavailable", detail: "Unexpected health response" };
            }
            return { name, state: "healthy", detail: "Endpoint returned the expected health response" };
          } catch {
            return { name, state: "unavailable", detail: "Could not reach the API" };
          }
        })
      );
      setProbes(results);
      setLastChecked(new Date().toISOString());
    } finally {
      setIsChecking(false);
    }
  }, []);

  useEffect(() => {
    void checkHealth();
  }, [checkHealth]);

  const healthyCount = probes.filter((probe) => probe.state === "healthy").length;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="#overview" aria-label="ARIMA home">
          <span className="brand-mark">A</span>
          <span><strong>ARIMA</strong><small>PRECISION OPERATIONS</small></span>
        </a>
        <div className="nav-caption">WORKSPACE</div>
        <nav aria-label="Main navigation">
          <a className="nav-item active" href="#overview"><span className="nav-glyph">◫</span> Overview</a>
          <a className="nav-item" href="#health"><span className="nav-glyph">⌁</span> API health</a>
          <a className="nav-item" href="#contracts"><span className="nav-glyph">⌘</span> Contracts</a>
          <a className="nav-item" href="#foundation"><span className="nav-glyph">▦</span> Foundation</a>
        </nav>
        <div className="sidebar-bottom">
          <span className="environment-dot" />
          <span>Engineering foundation</span>
          <span className="env-tag">v0.1</span>
        </div>
      </aside>

      <main className="main-content" id="overview">
        <header className="topbar">
          <div className="breadcrumb">Workspace <span>/</span> Overview</div>
          <a className="repo-link" href="https://github.com/nyeinpyaesone-ui/arima" target="_blank" rel="noreferrer">
            Repository <span aria-hidden="true">↗</span>
          </a>
        </header>

        <section className="page-heading">
          <div>
            <div className="eyebrow">PLATFORM WORKSPACE</div>
            <h1>Clarity across every operation.</h1>
            <p>A focused view of the ARIMA engineering foundation, service health, and integration contracts.</p>
          </div>
          <div className="heading-meta"><span className="status-pulse" /> FOUNDATION STAGE</div>
        </section>

        <section className="summary-grid" aria-label="Platform summary">
          <article className="summary-card">
            <div className="card-label">API SERVICE</div>
            <div className="summary-value">Go</div>
            <div className="summary-foot">HTTP service · port 8080 by default</div>
          </article>
          <article className="summary-card">
            <div className="card-label">HTTP CONTRACT</div>
            <div className="summary-value">OpenAPI <span className="value-version">3.1</span></div>
            <div className="summary-foot">Contract-first REST interface</div>
          </article>
          <article className="summary-card">
            <div className="card-label">SERVICE PROBES</div>
            <div className="summary-value">{healthyCount}<span className="value-total"> / 2</span></div>
            <div className="summary-foot">Health endpoints responding</div>
          </article>
        </section>

        <section className="content-section" id="health">
          <div className="section-heading">
            <div><div className="eyebrow">RUNTIME</div><h2>API health</h2></div>
            <button className="refresh-button" type="button" onClick={() => void checkHealth()} disabled={isChecking}>
              <span aria-hidden="true">↻</span> {isChecking ? "Checking…" : "Check again"}
            </button>
          </div>
          <div className="health-panel" aria-live="polite" aria-busy={isChecking}>
            {probes.map((probe) => (
              <div className="health-row" key={probe.name}>
                <div className="health-name">
                  <span className={`health-indicator ${probe.state}`} />
                  <div><strong>{probe.name === "live" ? "Liveness" : "Readiness"}</strong><small>/health/{probe.name}</small></div>
                </div>
                <div className="health-result">
                  <span className={`state-label ${probe.state}`}>{stateLabel(probe.state)}</span>
                  <small>{probe.detail ?? "Waiting for check"}</small>
                </div>
              </div>
            ))}
            <div className="health-footer">
              <span>{lastChecked ? `Last checked ${new Date(lastChecked).toLocaleTimeString()}` : "Checking service endpoints…"}</span>
              <span>Live request · no cached response</span>
            </div>
          </div>
        </section>

        <section className="content-section" id="contracts">
          <div className="section-heading">
            <div><div className="eyebrow">INTEGRATION</div><h2>Published contracts</h2></div>
            <span className="section-note">Source-controlled specifications</span>
          </div>
          <div className="contract-grid">
            {contractLinks.map((item) => (
              <a className="contract-card" href={item.href} target="_blank" rel="noreferrer" key={item.title}>
                <span className="contract-marker">{item.marker}</span>
                <span className="contract-copy"><strong>{item.title}</strong><small>{item.detail}</small></span>
                <span className="contract-arrow" aria-hidden="true">↗</span>
              </a>
            ))}
          </div>
        </section>

        <section className="content-section foundation-section" id="foundation">
          <div className="section-heading">
            <div><div className="eyebrow">CURRENT SCOPE</div><h2>Engineering foundation</h2></div>
          </div>
          <div className="foundation-panel">
            <div className="foundation-intro">
              <span className="foundation-icon">◎</span>
              <div><strong>Backend scaffold, frontend foundation</strong><p>The current API exposes health endpoints. Business resources, action execution, identity-provider integration, and audit workflows are not represented as operational features here.</p></div>
            </div>
            <div className="scope-tags"><span>Go API</span><span>REST contract</span><span>gRPC contract</span><span>React workspace</span></div>
          </div>
        </section>

        <footer className="page-footer">
          <span>ARIMA · Precision Operations</span>
          <span>Engineering foundation · Not a production ERP console</span>
        </footer>
      </main>
    </div>
  );
}

export default App;
