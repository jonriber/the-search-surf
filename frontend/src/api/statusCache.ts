import type { AvailableStatus } from "./serviceStatus";

const cacheKey = "the-search:last-available-status";

export function readCachedStatus(
  storage: Storage = globalThis.localStorage,
): AvailableStatus | null {
  try {
    const raw = storage.getItem(cacheKey);
    if (raw === null) {
      return null;
    }

    const parsed: unknown = JSON.parse(raw);
    if (!isAvailableStatus(parsed)) {
      storage.removeItem(cacheKey);
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function writeCachedStatus(
  status: AvailableStatus,
  storage: Storage = globalThis.localStorage,
): void {
  try {
    storage.setItem(cacheKey, JSON.stringify(status));
  } catch {
    // Storage is an optional resilience mechanism; availability must not depend on it.
  }
}

function isAvailableStatus(value: unknown): value is AvailableStatus {
  if (
    !isRecord(value) ||
    value.kind !== "available" ||
    typeof value.checkedAt !== "string"
  ) {
    return false;
  }

  const build = value.build;
  return (
    isRecord(build) &&
    typeof build.version === "string" &&
    build.version.length > 0 &&
    typeof build.commit === "string" &&
    build.commit.length > 0
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
