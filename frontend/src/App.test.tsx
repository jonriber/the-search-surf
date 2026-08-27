import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { axe } from "jest-axe";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { UserDataClient } from "./api/userData";
import { App } from "./App";

describe("App", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it("shows the available API release and passes baseline accessibility checks", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json({ status: "ready" }))
        .mockResolvedValueOnce(
          Response.json({ version: "1.2.3", commit: "abc123" }),
        ),
    );

    const { container } = render(<App />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "API available" }),
      ).toBeVisible(),
    );
    expect(screen.getByText("1.2.3")).toBeVisible();
    expect(await axe(container)).toHaveNoViolations();
  });

  it("uses a cached verified build when the API becomes unavailable", async () => {
    localStorage.setItem(
      "the-search:last-available-status",
      JSON.stringify({
        kind: "available",
        checkedAt: "2026-08-23T12:00:00.000Z",
        build: { version: "1.2.3", commit: "abc123" },
      }),
    );
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockRejectedValue(new TypeError("network failed")),
    );

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Cached status" }),
      ).toBeVisible(),
    );
    expect(screen.getByText(/API is unavailable/)).toBeVisible();
  });

  it("distinguishes a device without connectivity", async () => {
    vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
    const fetcher = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetcher);

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Device offline" }),
      ).toBeVisible(),
    );
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("reports an unavailable API without presenting cached data", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockRejectedValue(new TypeError("network failed")),
    );

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "API unavailable" }),
      ).toBeVisible(),
    );
  });

  it("reports a contract mismatch", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json({ status: "ready" }))
        .mockResolvedValueOnce(Response.json({ release: "unexpected" })),
    );

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "API contract mismatch" }),
      ).toBeVisible(),
    );
  });

  it("refreshes the release identity on demand", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json({ status: "ready" }))
        .mockResolvedValueOnce(
          Response.json({ version: "1.2.3", commit: "abc123" }),
        )
        .mockResolvedValueOnce(Response.json({ status: "ready" }))
        .mockResolvedValueOnce(
          Response.json({ version: "1.2.4", commit: "def456" }),
        ),
    );

    render(<App />);
    await waitFor(() => expect(screen.getByText("1.2.3")).toBeVisible());

    await user.click(screen.getByRole("button", { name: "Check again" }));

    await waitFor(() => expect(screen.getByText("1.2.4")).toBeVisible());
  });

  it("opens the profile and favorite workflows after verifying the API", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json({ status: "ready" }))
        .mockResolvedValueOnce(
          Response.json({ version: "1.2.3", commit: "abc123" }),
        ),
    );
    const client = createClient({
      getProfile: vi.fn().mockResolvedValue({
        experienceLevel: "intermediate",
        displayUnits: "metric",
        version: 1,
        createdAt: "2026-08-27T10:00:00Z",
        updatedAt: "2026-08-27T10:00:00Z",
      }),
      listSpots: vi.fn().mockResolvedValue([]),
      listFavorites: vi.fn().mockResolvedValue([]),
    });

    render(<App client={client} />);

    expect(
      await screen.findByRole("heading", { name: "Your surf profile" }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Your favorite spots" }),
    ).toBeVisible();
  });
});

function createClient(overrides: Partial<UserDataClient> = {}): UserDataClient {
  return {
    getProfile: vi.fn(),
    createProfile: vi.fn(),
    updateProfile: vi.fn(),
    listSpots: vi.fn(),
    createSpot: vi.fn(),
    updateSpot: vi.fn(),
    deleteSpot: vi.fn(),
    listFavorites: vi.fn(),
    addFavorite: vi.fn(),
    updateFavoritePosition: vi.fn(),
    removeFavorite: vi.fn(),
    ...overrides,
  };
}
