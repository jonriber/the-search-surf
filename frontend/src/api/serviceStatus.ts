export type BuildVersion = {
  version: string;
  commit: string;
};

export type AvailableStatus = {
  kind: "available";
  checkedAt: string;
  build: BuildVersion;
};

export type ServiceStatus =
  | { kind: "loading" }
  | AvailableStatus
  | { kind: "offline" }
  | { kind: "unavailable" }
  | { kind: "incompatible" }
  | {
      kind: "stale";
      reason: "offline" | "unavailable";
      cached: AvailableStatus;
    };

type Fetch = typeof globalThis.fetch;

type StatusOptions = {
  fetch?: Fetch;
  online?: boolean;
  timeoutMilliseconds?: number;
  now?: () => Date;
};

export async function fetchServiceStatus(
  options: StatusOptions = {},
): Promise<ServiceStatus> {
  const fetcher = options.fetch ?? globalThis.fetch;
  const online = options.online ?? globalThis.navigator.onLine;
  const timeoutMilliseconds = options.timeoutMilliseconds ?? 5_000;
  const now = options.now ?? (() => new Date());

  if (!online) {
    return { kind: "offline" };
  }

  const controller = new AbortController();
  const timeout = globalThis.setTimeout(
    () => controller.abort(),
    timeoutMilliseconds,
  );

  try {
    const [readinessResponse, versionResponse] = await Promise.all([
      fetcher("/api/health/ready", {
        headers: { Accept: "application/json" },
        signal: controller.signal,
      }),
      fetcher("/api/version", {
        headers: { Accept: "application/json" },
        signal: controller.signal,
      }),
    ]);

    if (!readinessResponse.ok || !versionResponse.ok) {
      return { kind: "unavailable" };
    }

    const readiness: unknown = await readinessResponse.json();
    const build: unknown = await versionResponse.json();

    if (!isReady(readiness) || !isBuildVersion(build)) {
      return { kind: "incompatible" };
    }

    return {
      kind: "available",
      checkedAt: now().toISOString(),
      build,
    };
  } catch {
    return { kind: "unavailable" };
  } finally {
    globalThis.clearTimeout(timeout);
  }
}

function isReady(value: unknown): value is { status: "ready" } {
  return isRecord(value) && value.status === "ready";
}

function isBuildVersion(value: unknown): value is BuildVersion {
  return (
    isRecord(value) &&
    typeof value.version === "string" &&
    value.version.length > 0 &&
    typeof value.commit === "string" &&
    value.commit.length > 0
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
