import { useCallback, useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";

import type { Profile, ProfileWrite } from "../../api/generated/types.gen";
import { ApiError, type UserDataClient } from "../../api/userData";

type ProfileSectionProps = {
  client: UserDataClient;
};

type LoadState = "loading" | "ready" | "unavailable";

const initialForm: ProfileWrite = {
  experienceLevel: "beginner",
  displayUnits: "metric",
};

export function ProfileSection({ client }: ProfileSectionProps) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [form, setForm] = useState<ProfileWrite>(initialForm);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alertRef = useRef<HTMLDivElement>(null);

  const loadProfile = useCallback(async () => {
    setLoadState("loading");
    setError(null);
    setNotice(null);
    try {
      const loaded = await client.getProfile();
      setProfile(loaded);
      setForm({
        experienceLevel: loaded.experienceLevel,
        displayUnits: loaded.displayUnits,
      });
      setLoadState("ready");
    } catch (cause) {
      if (cause instanceof ApiError && cause.kind === "not-found") {
        setProfile(null);
        setForm(initialForm);
        setLoadState("ready");
        return;
      }
      setLoadState("unavailable");
      setError(
        "We could not load your profile. Check the server and try again.",
      );
    }
  }, [client]);

  useEffect(() => {
    let active = true;
    void client
      .getProfile()
      .then((loaded) => {
        if (!active) return;
        setProfile(loaded);
        setForm({
          experienceLevel: loaded.experienceLevel,
          displayUnits: loaded.displayUnits,
        });
        setLoadState("ready");
      })
      .catch((cause: unknown) => {
        if (!active) return;
        if (cause instanceof ApiError && cause.kind === "not-found") {
          setProfile(null);
          setForm(initialForm);
          setLoadState("ready");
          return;
        }
        setLoadState("unavailable");
        setError(
          "We could not load your profile. Check the server and try again.",
        );
      });
    return () => {
      active = false;
    };
  }, [client]);

  useEffect(() => {
    if (error !== null) {
      alertRef.current?.focus();
    }
  }, [error]);

  async function saveProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError(null);
    setNotice(null);

    try {
      const saved =
        profile === null
          ? await client.createProfile(form)
          : await client.updateProfile({
              ...form,
              expectedVersion: profile.version,
            });
      setProfile(saved);
      setForm({
        experienceLevel: saved.experienceLevel,
        displayUnits: saved.displayUnits,
      });
      setNotice("Profile saved.");
    } catch (cause) {
      setError(profileErrorMessage(cause));
    } finally {
      setSaving(false);
    }
  }

  if (loadState === "loading") {
    return (
      <section className="content-card" aria-labelledby="profile-heading">
        <p className="section-label">Personalization</p>
        <h2 id="profile-heading">Loading your surf profile</h2>
        <p role="status">Checking your experience and display preferences…</p>
      </section>
    );
  }

  if (loadState === "unavailable") {
    return (
      <section className="content-card" aria-labelledby="profile-heading">
        <p className="section-label">Personalization</p>
        <h2 id="profile-heading">Profile unavailable</h2>
        <div
          className="feedback error"
          role="alert"
          tabIndex={-1}
          ref={alertRef}
        >
          {error}
        </div>
        <button type="button" onClick={() => void loadProfile()}>
          Retry profile
        </button>
      </section>
    );
  }

  return (
    <section className="content-card" aria-labelledby="profile-heading">
      <p className="section-label">Personalization</p>
      <h2 id="profile-heading">
        {profile === null ? "Set up your surf profile" : "Your surf profile"}
      </h2>
      <p className="card-copy">
        Your level and preferred units will shape future surf recommendations.
      </p>

      {error !== null && (
        <div
          className="feedback error"
          role="alert"
          tabIndex={-1}
          ref={alertRef}
        >
          <p>{error}</p>
          {error.includes("changed on the server") && (
            <button type="button" onClick={() => void loadProfile()}>
              Reload profile
            </button>
          )}
        </div>
      )}
      {notice !== null && (
        <p className="feedback success" role="status">
          {notice}
        </p>
      )}

      <form
        className="stacked-form"
        onSubmit={(event) => void saveProfile(event)}
      >
        <label>
          <span>Experience level</span>
          <select
            value={form.experienceLevel}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                experienceLevel: event.target
                  .value as ProfileWrite["experienceLevel"],
              }))
            }
          >
            <option value="beginner">Beginner</option>
            <option value="intermediate">Intermediate</option>
            <option value="advanced">Advanced</option>
            <option value="expert">Expert</option>
          </select>
        </label>

        <label>
          <span>Display units</span>
          <select
            value={form.displayUnits}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                displayUnits: event.target
                  .value as ProfileWrite["displayUnits"],
              }))
            }
          >
            <option value="metric">Metric (m, km/h)</option>
            <option value="imperial">Imperial (ft, mph)</option>
          </select>
        </label>

        <button type="submit" disabled={saving}>
          {saving
            ? "Saving profile…"
            : profile === null
              ? "Save profile"
              : "Update profile"}
        </button>
      </form>
    </section>
  );
}

function profileErrorMessage(cause: unknown): string {
  if (cause instanceof ApiError) {
    if (cause.kind === "conflict") {
      return "Your profile changed on the server. Reload it before saving again.";
    }
    if (cause.kind === "validation") {
      return "The profile values are invalid. Review them and try again.";
    }
    if (cause.kind === "unavailable") {
      return "The server is unavailable. Your changes were not saved.";
    }
  }
  return "Your profile could not be saved. Try again.";
}
