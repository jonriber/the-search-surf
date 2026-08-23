import { useCallback, useEffect, useState } from "react";

import { fetchServiceStatus, type ServiceStatus } from "../api/serviceStatus";
import { readCachedStatus, writeCachedStatus } from "../api/statusCache";

export function useServiceStatus(): {
  status: ServiceStatus;
  refresh: () => void;
} {
  const [status, setStatus] = useState<ServiceStatus>({ kind: "loading" });
  const [refreshSequence, setRefreshSequence] = useState(0);

  const refresh = useCallback(() => {
    setStatus({ kind: "loading" });
    setRefreshSequence((current) => current + 1);
  }, []);

  useEffect(() => {
    let active = true;

    void fetchServiceStatus().then((nextStatus) => {
      if (!active) {
        return;
      }

      if (nextStatus.kind === "available") {
        writeCachedStatus(nextStatus);
        setStatus(nextStatus);
        return;
      }

      if (nextStatus.kind === "offline" || nextStatus.kind === "unavailable") {
        const cached = readCachedStatus();
        if (cached !== null) {
          setStatus({ kind: "stale", reason: nextStatus.kind, cached });
          return;
        }
      }

      setStatus(nextStatus);
    });

    return () => {
      active = false;
    };
  }, [refreshSequence]);

  return { status, refresh };
}
