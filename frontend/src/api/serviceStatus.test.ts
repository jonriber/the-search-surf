import { describe, expect, it, vi } from "vitest";

import { fetchServiceStatus } from "./serviceStatus";

describe("fetchServiceStatus", () => {
  it("returns the validated ready build", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json({ status: "ready" }))
      .mockResolvedValueOnce(
        Response.json({ version: "1.2.3", commit: "abc123" }),
      );

    const status = await fetchServiceStatus({
      fetch: fetcher,
      online: true,
      now: () => new Date("2026-08-23T12:00:00.000Z"),
    });

    expect(status).toEqual({
      kind: "available",
      checkedAt: "2026-08-23T12:00:00.000Z",
      build: { version: "1.2.3", commit: "abc123" },
    });
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("distinguishes an offline device without calling the API", async () => {
    const fetcher = vi.fn<typeof fetch>();

    await expect(
      fetchServiceStatus({ fetch: fetcher, online: false }),
    ).resolves.toEqual({
      kind: "offline",
    });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("reports an unavailable API", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        Response.json({ status: "unavailable" }, { status: 503 }),
      )
      .mockResolvedValueOnce(
        Response.json({ version: "1.2.3", commit: "abc123" }),
      );

    await expect(
      fetchServiceStatus({ fetch: fetcher, online: true }),
    ).resolves.toEqual({
      kind: "unavailable",
    });
  });

  it("rejects an incompatible API contract", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json({ status: "ready" }))
      .mockResolvedValueOnce(Response.json({ release: "unexpected" }));

    await expect(
      fetchServiceStatus({ fetch: fetcher, online: true }),
    ).resolves.toEqual({
      kind: "incompatible",
    });
  });

  it("maps network failures to API unavailability", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new TypeError("network failed"));

    await expect(
      fetchServiceStatus({ fetch: fetcher, online: true }),
    ).resolves.toEqual({
      kind: "unavailable",
    });
  });
});
