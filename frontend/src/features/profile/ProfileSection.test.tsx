import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { axe } from "jest-axe";
import { describe, expect, it, vi } from "vitest";

import { ApiError, type UserDataClient } from "../../api/userData";
import { ProfileSection } from "./ProfileSection";

const timestamp = "2026-08-27T10:00:00Z";

describe("ProfileSection", () => {
  it("offers an accessible setup form when the profile does not exist", async () => {
    const client = createClient({
      getProfile: vi
        .fn()
        .mockRejectedValue(new ApiError("not-found", "not found")),
    });

    const { container } = render(<ProfileSection client={client} />);

    expect(
      await screen.findByRole("heading", { name: "Set up your surf profile" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Experience level")).toBeVisible();
    expect(screen.getByLabelText("Display units")).toBeVisible();
    expect(await axe(container)).toHaveNoViolations();
  });

  it("creates a profile and switches to edit mode", async () => {
    const user = userEvent.setup();
    const createProfile = vi.fn().mockResolvedValue({
      experienceLevel: "beginner",
      displayUnits: "imperial",
      version: 1,
      createdAt: timestamp,
      updatedAt: timestamp,
    });
    const client = createClient({
      getProfile: vi
        .fn()
        .mockRejectedValue(new ApiError("not-found", "not found")),
      createProfile,
    });
    render(<ProfileSection client={client} />);
    await screen.findByRole("heading", { name: "Set up your surf profile" });

    await user.selectOptions(
      screen.getByLabelText("Display units"),
      "imperial",
    );
    await user.click(screen.getByRole("button", { name: "Save profile" }));

    await waitFor(() =>
      expect(createProfile).toHaveBeenCalledWith({
        experienceLevel: "beginner",
        displayUnits: "imperial",
      }),
    );
    expect(
      await screen.findByRole("heading", { name: "Your surf profile" }),
    ).toBeVisible();
    expect(screen.getByText("Profile saved.")).toBeVisible();
  });

  it("updates a profile with its observed version", async () => {
    const user = userEvent.setup();
    const updateProfile = vi.fn().mockResolvedValue({
      experienceLevel: "advanced",
      displayUnits: "metric",
      version: 4,
      createdAt: timestamp,
      updatedAt: timestamp,
    });
    const client = createClient({
      getProfile: vi.fn().mockResolvedValue({
        experienceLevel: "intermediate",
        displayUnits: "metric",
        version: 3,
        createdAt: timestamp,
        updatedAt: timestamp,
      }),
      updateProfile,
    });
    render(<ProfileSection client={client} />);
    await screen.findByRole("heading", { name: "Your surf profile" });

    await user.selectOptions(
      screen.getByLabelText("Experience level"),
      "advanced",
    );
    await user.click(screen.getByRole("button", { name: "Update profile" }));

    await waitFor(() =>
      expect(updateProfile).toHaveBeenCalledWith({
        experienceLevel: "advanced",
        displayUnits: "metric",
        expectedVersion: 3,
      }),
    );
  });

  it("announces conflicts, moves focus, and reloads current data", async () => {
    const user = userEvent.setup();
    const getProfile = vi
      .fn()
      .mockResolvedValueOnce({
        experienceLevel: "intermediate",
        displayUnits: "metric",
        version: 3,
        createdAt: timestamp,
        updatedAt: timestamp,
      })
      .mockResolvedValueOnce({
        experienceLevel: "expert",
        displayUnits: "imperial",
        version: 4,
        createdAt: timestamp,
        updatedAt: timestamp,
      });
    const client = createClient({
      getProfile,
      updateProfile: vi
        .fn()
        .mockRejectedValue(new ApiError("conflict", "conflict")),
    });
    render(<ProfileSection client={client} />);
    await screen.findByRole("heading", { name: "Your surf profile" });

    await user.click(screen.getByRole("button", { name: "Update profile" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/changed on the server/i);
    expect(alert).toHaveFocus();

    await user.click(screen.getByRole("button", { name: "Reload profile" }));
    await waitFor(() => expect(getProfile).toHaveBeenCalledTimes(2));
    expect(screen.getByLabelText("Experience level")).toHaveValue("expert");
    expect(screen.getByLabelText("Display units")).toHaveValue("imperial");
  });

  it("shows unavailable state with a retry action", async () => {
    const user = userEvent.setup();
    const getProfile = vi
      .fn()
      .mockRejectedValueOnce(new ApiError("unavailable", "unavailable"))
      .mockRejectedValueOnce(new ApiError("not-found", "not found"));
    render(<ProfileSection client={createClient({ getProfile })} />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /could not load your profile/i,
    );
    await user.click(screen.getByRole("button", { name: "Retry profile" }));

    expect(
      await screen.findByRole("heading", { name: "Set up your surf profile" }),
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
