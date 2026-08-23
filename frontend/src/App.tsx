import { useServiceStatus } from "./hooks/useServiceStatus";

export function App() {
  const { status, refresh } = useServiceStatus();

  return (
    <main className="app-shell">
      <header className="hero">
        <p className="eyebrow">Personal surf intelligence</p>
        <h1>The Search</h1>
        <p className="hero-copy">
          Explainable surf recommendations shaped by your spots, preferences,
          and forecast data.
        </p>
      </header>

      <section
        className="status-card"
        aria-labelledby="service-heading"
        aria-live="polite"
      >
        <div>
          <p className="section-label">Homelab connection</p>
          <h2 id="service-heading">{statusHeading(status)}</h2>
        </div>

        <StatusDetails status={status} />

        {status.kind !== "loading" && (
          <button type="button" onClick={refresh}>
            Check again
          </button>
        )}
      </section>

      <section className="next-card" aria-labelledby="next-heading">
        <p className="section-label">Foundation milestone</p>
        <h2 id="next-heading">The forecast journey comes next.</h2>
        <p>
          This first slice verifies the mobile client, API boundary, offline
          behavior, and release identity before surf-domain logic is introduced.
        </p>
      </section>
    </main>
  );
}

function StatusDetails({
  status,
}: {
  status: ReturnType<typeof useServiceStatus>["status"];
}) {
  switch (status.kind) {
    case "loading":
      return <p>Checking the API and release contract…</p>;
    case "available":
      return (
        <dl className="build-details">
          <div>
            <dt>Version</dt>
            <dd>{status.build.version}</dd>
          </div>
          <div>
            <dt>Commit</dt>
            <dd>{status.build.commit}</dd>
          </div>
        </dl>
      );
    case "offline":
      return (
        <p>
          Your device is offline and no previously verified API status is
          cached.
        </p>
      );
    case "unavailable":
      return (
        <p>The device is online, but the API cannot currently be reached.</p>
      );
    case "incompatible":
      return (
        <p>
          The API responded, but its contract is incompatible with this client.
        </p>
      );
    case "stale":
      return (
        <p>
          Showing the last verified build, {status.cached.build.version} at{" "}
          {new Date(status.cached.checkedAt).toLocaleString()}, because the{" "}
          {status.reason === "offline"
            ? "device is offline"
            : "API is unavailable"}
          .
        </p>
      );
  }
}

function statusHeading(
  status: ReturnType<typeof useServiceStatus>["status"],
): string {
  switch (status.kind) {
    case "loading":
      return "Checking connection";
    case "available":
      return "API available";
    case "offline":
      return "Device offline";
    case "unavailable":
      return "API unavailable";
    case "incompatible":
      return "API contract mismatch";
    case "stale":
      return "Cached status";
  }
}
