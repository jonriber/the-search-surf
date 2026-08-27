import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { axe } from "jest-axe";
import { describe, expect, it, vi } from "vitest";

import type { Favorite, Spot, SpotWrite } from "../../api/generated/types.gen";
import { ApiError, type UserDataClient } from "../../api/userData";
import { FavoriteSpotsSection } from "./FavoriteSpotsSection";

const timestamp = "2026-08-27T10:00:00Z";
const spotId = "a77e3c45-9d6c-4e7f-896f-25bf4f0b8ee6";
const spot: Spot = {
  id: spotId,
  name: "Supertubos",
  longitude: -9.36,
  latitude: 39.34,
  timeZone: "Europe/Lisbon",
  version: 2,
  createdAt: timestamp,
  updatedAt: timestamp,
};
const favorite: Favorite = {
  spotId,
  sortPosition: 0,
  createdAt: timestamp,
  updatedAt: timestamp,
};

describe("FavoriteSpotsSection", () => {
  it("shows an accessible empty state", async () => {
    const client = createClient({
      listSpots: vi.fn().mockResolvedValue([]),
      listFavorites: vi.fn().mockResolvedValue([]),
    });

    const { container } = render(<FavoriteSpotsSection client={client} />);

    expect(await screen.findByText("No favorite spots yet.")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Add a favorite spot" }),
    ).toBeVisible();
    expect(await axe(container)).toHaveNoViolations();
  });

  it("correlates favorite relationships with private spots", async () => {
    render(
      <FavoriteSpotsSection
        client={createClient({
          listSpots: vi.fn().mockResolvedValue([spot]),
          listFavorites: vi.fn().mockResolvedValue([favorite]),
        })}
      />,
    );

    expect(
      await screen.findByRole("heading", { name: "Supertubos" }),
    ).toBeVisible();
    expect(screen.getByText("39.3400, -9.3600")).toBeVisible();
    expect(screen.getByText("Europe/Lisbon")).toBeVisible();
  });

  it("creates a private spot and then its favorite relationship", async () => {
    const user = userEvent.setup();
    const createSpot = vi.fn().mockResolvedValue({ ...spot, version: 1 });
    const addFavorite = vi.fn().mockResolvedValue(favorite);
    const client = createClient({
      listSpots: vi.fn().mockResolvedValue([]),
      listFavorites: vi.fn().mockResolvedValue([]),
      createSpot,
      addFavorite,
    });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByText("No favorite spots yet.");

    await user.click(
      screen.getByRole("button", { name: "Add a favorite spot" }),
    );
    await completeSpotForm(user);
    await user.click(
      screen.getByRole("button", { name: "Save favorite spot" }),
    );

    await waitFor(() => expect(createSpot).toHaveBeenCalledWith(spotWrite));
    expect(addFavorite).toHaveBeenCalledWith({ spotId, sortPosition: 0 });
    expect(
      await screen.findByRole("heading", { name: "Supertubos" }),
    ).toBeVisible();
    expect(screen.getByText("Favorite spot saved.")).toBeVisible();
  });

  it("keeps invalid coordinates and time zones out of the API", async () => {
    const user = userEvent.setup();
    const createSpot = vi.fn();
    const client = createClient({
      listSpots: vi.fn().mockResolvedValue([]),
      listFavorites: vi.fn().mockResolvedValue([]),
      createSpot,
    });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByText("No favorite spots yet.");

    await user.click(
      screen.getByRole("button", { name: "Add a favorite spot" }),
    );
    await user.type(screen.getByLabelText("Spot name"), "Impossible Beach");
    await user.type(screen.getByLabelText("Latitude"), "91");
    await user.type(screen.getByLabelText("Longitude"), "-9.36");
    await user.type(screen.getByLabelText("Time zone"), "Lisbon-ish");
    await user.click(
      screen.getByRole("button", { name: "Save favorite spot" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /valid coordinates, and an IANA time zone/i,
    );
    expect(createSpot).not.toHaveBeenCalled();
  });

  it("preserves a created spot and offers retry when favoriting fails", async () => {
    const user = userEvent.setup();
    const addFavorite = vi
      .fn()
      .mockRejectedValueOnce(new ApiError("unavailable", "unavailable"))
      .mockResolvedValueOnce(favorite);
    const client = createClient({
      listSpots: vi.fn().mockResolvedValue([]),
      listFavorites: vi.fn().mockResolvedValue([]),
      createSpot: vi.fn().mockResolvedValue({ ...spot, version: 1 }),
      addFavorite,
    });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByText("No favorite spots yet.");

    await user.click(
      screen.getByRole("button", { name: "Add a favorite spot" }),
    );
    await completeSpotForm(user);
    await user.click(
      screen.getByRole("button", { name: "Save favorite spot" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/spot was saved, but could not be added/i);
    expect(alert).toHaveFocus();
    await user.click(
      screen.getByRole("button", { name: "Retry adding favorite" }),
    );

    expect(addFavorite).toHaveBeenCalledTimes(2);
    expect(
      await screen.findByRole("heading", { name: "Supertubos" }),
    ).toBeVisible();
  });

  it("edits a spot with optimistic concurrency", async () => {
    const user = userEvent.setup();
    const updateSpot = vi.fn().mockResolvedValue({
      ...spot,
      name: "Supertubos North",
      version: 3,
    });
    const client = createLoadedClient({ updateSpot });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByRole("heading", { name: "Supertubos" });

    await user.click(screen.getByRole("button", { name: "Edit Supertubos" }));
    const name = screen.getByLabelText("Spot name");
    await user.clear(name);
    await user.type(name, "Supertubos North");
    await user.click(screen.getByRole("button", { name: "Save spot changes" }));

    await waitFor(() =>
      expect(updateSpot).toHaveBeenCalledWith(spotId, {
        name: "Supertubos North",
        longitude: -9.36,
        latitude: 39.34,
        timeZone: "Europe/Lisbon",
        expectedVersion: 2,
      }),
    );
    expect(
      await screen.findByRole("heading", { name: "Supertubos North" }),
    ).toBeVisible();
  });

  it("reloads the collection after an edit conflict", async () => {
    const user = userEvent.setup();
    const listSpots = vi
      .fn()
      .mockResolvedValueOnce([spot])
      .mockResolvedValueOnce([{ ...spot, name: "Server name", version: 3 }]);
    const client = createClient({
      listSpots,
      listFavorites: vi.fn().mockResolvedValue([favorite]),
      updateSpot: vi
        .fn()
        .mockRejectedValue(new ApiError("conflict", "conflict")),
    });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByRole("heading", { name: "Supertubos" });

    await user.click(screen.getByRole("button", { name: "Edit Supertubos" }));
    await user.click(screen.getByRole("button", { name: "Save spot changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /changed on the server/i,
    );
    await user.click(
      screen.getByRole("button", { name: "Reload favorite spots" }),
    );

    expect(
      await screen.findByRole("heading", { name: "Server name" }),
    ).toBeVisible();
  });

  it("removes only the favorite relationship", async () => {
    const user = userEvent.setup();
    const removeFavorite = vi.fn().mockResolvedValue(undefined);
    const deleteSpot = vi.fn();
    const client = createLoadedClient({ removeFavorite, deleteSpot });
    render(<FavoriteSpotsSection client={client} />);
    await screen.findByRole("heading", { name: "Supertubos" });

    await user.click(
      screen.getByRole("button", { name: "Remove Supertubos from favorites" }),
    );

    await waitFor(() => expect(removeFavorite).toHaveBeenCalledWith(spotId));
    expect(deleteSpot).not.toHaveBeenCalled();
    expect(screen.getByText("No favorite spots yet.")).toBeVisible();
  });

  it("recovers from a loading failure", async () => {
    const user = userEvent.setup();
    const listSpots = vi
      .fn()
      .mockRejectedValueOnce(new ApiError("unavailable", "unavailable"))
      .mockResolvedValueOnce([]);
    const listFavorites = vi.fn().mockResolvedValue([]);
    render(
      <FavoriteSpotsSection
        client={createClient({ listSpots, listFavorites })}
      />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /could not load your favorite spots/i,
    );
    await user.click(
      screen.getByRole("button", { name: "Retry favorite spots" }),
    );

    expect(await screen.findByText("No favorite spots yet.")).toBeVisible();
  });
});

const spotWrite: SpotWrite = {
  name: "Supertubos",
  longitude: -9.36,
  latitude: 39.34,
  timeZone: "Europe/Lisbon",
};

async function completeSpotForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Spot name"), spotWrite.name);
  await user.type(
    screen.getByLabelText("Latitude"),
    String(spotWrite.latitude),
  );
  await user.type(
    screen.getByLabelText("Longitude"),
    String(spotWrite.longitude),
  );
  await user.type(screen.getByLabelText("Time zone"), spotWrite.timeZone);
}

function createLoadedClient(
  overrides: Partial<UserDataClient> = {},
): UserDataClient {
  return createClient({
    listSpots: vi.fn().mockResolvedValue([spot]),
    listFavorites: vi.fn().mockResolvedValue([favorite]),
    ...overrides,
  });
}

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
