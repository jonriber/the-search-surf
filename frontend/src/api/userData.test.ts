import { describe, expect, it, vi } from "vitest";

import {
  ApiError,
  createUserDataClient,
  type UserDataClient,
} from "./userData";

const timestamp = "2026-08-27T10:00:00Z";
const spotId = "a77e3c45-9d6c-4e7f-896f-25bf4f0b8ee6";

describe("user data API client", () => {
  it("returns a contract-validated profile while preserving numeric versions", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json({
        experienceLevel: "intermediate",
        displayUnits: "metric",
        version: 2,
        createdAt: timestamp,
        updatedAt: timestamp,
      }),
    );
    const client = createClient(fetcher);

    await expect(client.getProfile()).resolves.toMatchObject({
      experienceLevel: "intermediate",
      displayUnits: "metric",
      version: 2,
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/profile",
      expect.objectContaining({
        method: "GET",
        headers: { Accept: "application/json" },
      }),
    );
  });

  it("maps a missing profile to an explicit not-found failure", async () => {
    const client = createClient(
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          Response.json(
            { code: "not_found", message: "profile not found" },
            { status: 404 },
          ),
        ),
    );

    await expect(client.getProfile()).rejects.toMatchObject({
      kind: "not-found",
      status: 404,
      code: "not_found",
    });
  });

  it.each([
    [400, "invalid_request", "validation"],
    [409, "version_conflict", "conflict"],
    [503, "unavailable", "unavailable"],
  ] as const)(
    "maps HTTP %s errors to %s failures",
    async (status, code, kind) => {
      const client = createClient(
        vi
          .fn<typeof fetch>()
          .mockResolvedValue(
            Response.json({ code, message: "safe message" }, { status }),
          ),
      );

      await expect(client.getProfile()).rejects.toMatchObject({
        kind,
        status,
        code,
      });
    },
  );

  it("rejects an incompatible success payload", async () => {
    const client = createClient(
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(Response.json({ experienceLevel: "wizard" })),
    );

    await expect(client.getProfile()).rejects.toMatchObject({
      kind: "incompatible",
    });
  });

  it("rejects versions that cannot be represented safely in JavaScript", async () => {
    const client = createClient(
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json({
          experienceLevel: "intermediate",
          displayUnits: "metric",
          version: Number.MAX_SAFE_INTEGER + 1,
          createdAt: timestamp,
          updatedAt: timestamp,
        }),
      ),
    );

    await expect(client.getProfile()).rejects.toMatchObject({
      kind: "incompatible",
    });
  });

  it("maps network and timeout failures to unavailability", async () => {
    const networkClient = createClient(
      vi.fn<typeof fetch>().mockRejectedValue(new TypeError("network failed")),
    );
    const timeoutClient = createUserDataClient({
      timeoutMilliseconds: 1,
      fetch: vi.fn<typeof fetch>().mockImplementation(
        (_input, init) =>
          new Promise((_resolve, reject) => {
            init?.signal?.addEventListener("abort", () =>
              reject(new DOMException("aborted", "AbortError")),
            );
          }),
      ),
    });

    await expect(networkClient.listSpots()).rejects.toMatchObject({
      kind: "unavailable",
    });
    await expect(timeoutClient.listSpots()).rejects.toMatchObject({
      kind: "unavailable",
    });
  });

  it("serializes profile mutations with optimistic concurrency", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json({
        experienceLevel: "advanced",
        displayUnits: "imperial",
        version: 4,
        createdAt: timestamp,
        updatedAt: timestamp,
      }),
    );
    const client = createClient(fetcher);

    await client.updateProfile({
      experienceLevel: "advanced",
      displayUnits: "imperial",
      expectedVersion: 3,
    });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/profile",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          experienceLevel: "advanced",
          displayUnits: "imperial",
          expectedVersion: 3,
        }),
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
      }),
    );
  });

  it("lists and mutates spots through encoded ownership-safe routes", async () => {
    const spot = {
      id: spotId,
      name: "Supertubos",
      longitude: -9.36,
      latitude: 39.34,
      timeZone: "Europe/Lisbon",
      version: 3,
      createdAt: timestamp,
      updatedAt: timestamp,
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json({ items: [spot] }))
      .mockResolvedValueOnce(Response.json(spot))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const client = createClient(fetcher);

    await expect(client.listSpots()).resolves.toEqual([spot]);
    await client.updateSpot(spotId, {
      name: spot.name,
      longitude: spot.longitude,
      latitude: spot.latitude,
      timeZone: spot.timeZone,
      expectedVersion: 2,
    });
    await client.deleteSpot(spotId, 3);

    expect(fetcher).toHaveBeenNthCalledWith(
      2,
      `/api/spots/${spotId}`,
      expect.objectContaining({ method: "PUT" }),
    );
    expect(fetcher).toHaveBeenNthCalledWith(
      3,
      `/api/spots/${spotId}?expectedVersion=3`,
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("lists and mutates favorite relationships without deleting spots", async () => {
    const favorite = {
      spotId,
      sortPosition: 0,
      createdAt: timestamp,
      updatedAt: timestamp,
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json({ items: [favorite] }))
      .mockResolvedValueOnce(Response.json(favorite))
      .mockResolvedValueOnce(Response.json({ ...favorite, sortPosition: 1 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const client = createClient(fetcher);

    await expect(client.listFavorites()).resolves.toEqual([favorite]);
    await client.addFavorite({ spotId, sortPosition: 0 });
    await client.updateFavoritePosition(spotId, { sortPosition: 1 });
    await client.removeFavorite(spotId);

    expect(fetcher).toHaveBeenNthCalledWith(
      4,
      `/api/favorites/${spotId}`,
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("uses a stable public error without exposing invalid server payloads", async () => {
    const client = createClient(
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(new Response("proxy secret", { status: 502 })),
    );

    try {
      await client.getProfile();
      throw new Error("expected request to fail");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({ kind: "unavailable", status: 502 });
      expect((error as Error).message).not.toContain("proxy secret");
    }
  });
});

function createClient(fetcher: typeof fetch): UserDataClient {
  return createUserDataClient({ fetch: fetcher, timeoutMilliseconds: 50 });
}
