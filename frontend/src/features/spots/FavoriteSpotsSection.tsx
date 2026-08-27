import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";

import type { Favorite, Spot, SpotWrite } from "../../api/generated/types.gen";
import { ApiError, type UserDataClient } from "../../api/userData";

type FavoriteSpotsSectionProps = {
  client: UserDataClient;
};

type SpotForm = {
  name: string;
  latitude: string;
  longitude: string;
  timeZone: string;
};

const emptyForm: SpotForm = {
  name: "",
  latitude: "",
  longitude: "",
  timeZone: "",
};

export function FavoriteSpotsSection({ client }: FavoriteSpotsSectionProps) {
  const [spots, setSpots] = useState<Spot[]>([]);
  const [favorites, setFavorites] = useState<Favorite[]>([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState<SpotForm>(emptyForm);
  const [editingSpot, setEditingSpot] = useState<Spot | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [pendingFavorite, setPendingFavorite] = useState<Spot | null>(null);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alertRef = useRef<HTMLDivElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);

  const loadFavorites = useCallback(async () => {
    setLoading(true);
    setError(null);
    setShowForm(false);
    setEditingSpot(null);
    setForm(emptyForm);
    try {
      const [loadedSpots, loadedFavorites] = await Promise.all([
        client.listSpots(),
        client.listFavorites(),
      ]);
      setSpots(loadedSpots);
      setFavorites(loadedFavorites);
    } catch {
      setError(
        "We could not load your favorite spots. Check the server and try again.",
      );
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    let active = true;
    void Promise.all([client.listSpots(), client.listFavorites()])
      .then(([loadedSpots, loadedFavorites]) => {
        if (!active) return;
        setSpots(loadedSpots);
        setFavorites(loadedFavorites);
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setError(
          "We could not load your favorite spots. Check the server and try again.",
        );
        setLoading(false);
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

  useEffect(() => {
    if (showForm) {
      nameRef.current?.focus();
    }
  }, [showForm]);

  const favoriteSpots = useMemo(
    () =>
      favorites
        .map((favorite) => ({
          favorite,
          spot: spots.find((spot) => spot.id === favorite.spotId),
        }))
        .filter(
          (item): item is { favorite: Favorite; spot: Spot } =>
            item.spot !== undefined,
        )
        .sort(
          (left, right) =>
            left.favorite.sortPosition - right.favorite.sortPosition ||
            left.spot.name.localeCompare(right.spot.name),
        ),
    [favorites, spots],
  );

  function openCreateForm() {
    setEditingSpot(null);
    setForm(emptyForm);
    setError(null);
    setNotice(null);
    setShowForm(true);
  }

  function openEditForm(spot: Spot) {
    setEditingSpot(spot);
    setForm({
      name: spot.name,
      latitude: String(spot.latitude),
      longitude: String(spot.longitude),
      timeZone: spot.timeZone,
    });
    setError(null);
    setNotice(null);
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditingSpot(null);
    setForm(emptyForm);
  }

  async function saveSpot(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const values = toSpotWrite(form);
    if (values === null) {
      setError(
        "Enter a name, valid coordinates, and an IANA time zone before saving.",
      );
      return;
    }

    setSaving(true);
    setError(null);
    setNotice(null);

    if (editingSpot !== null) {
      try {
        const updated = await client.updateSpot(editingSpot.id, {
          ...values,
          expectedVersion: editingSpot.version,
        });
        setSpots((current) =>
          current.map((spot) => (spot.id === updated.id ? updated : spot)),
        );
        closeForm();
        setNotice("Favorite spot updated.");
      } catch (cause) {
        setError(spotMutationError(cause, "updated"));
      } finally {
        setSaving(false);
      }
      return;
    }

    let created: Spot;
    try {
      created = await client.createSpot(values);
      setSpots((current) => [...current, created]);
    } catch (cause) {
      setError(spotMutationError(cause, "saved"));
      setSaving(false);
      return;
    }

    try {
      const favorite = await client.addFavorite({
        spotId: created.id,
        sortPosition: nextSortPosition(favorites),
      });
      setFavorites((current) => [...current, favorite]);
      closeForm();
      setNotice("Favorite spot saved.");
    } catch {
      setPendingFavorite(created);
      closeForm();
      setError(
        "The spot was saved, but could not be added to favorites. Retry without creating a duplicate spot.",
      );
    } finally {
      setSaving(false);
    }
  }

  async function retryFavorite() {
    if (pendingFavorite === null) return;
    setSaving(true);
    setError(null);
    try {
      const favorite = await client.addFavorite({
        spotId: pendingFavorite.id,
        sortPosition: nextSortPosition(favorites),
      });
      setFavorites((current) => [...current, favorite]);
      setPendingFavorite(null);
      setNotice("Favorite spot saved.");
    } catch {
      setError(
        "The spot is still saved, but adding it to favorites failed again.",
      );
    } finally {
      setSaving(false);
    }
  }

  async function removeFavorite(spot: Spot) {
    setError(null);
    setNotice(null);
    try {
      await client.removeFavorite(spot.id);
      setFavorites((current) =>
        current.filter((favorite) => favorite.spotId !== spot.id),
      );
      setNotice(
        `${spot.name} was removed from favorites. The private spot remains saved.`,
      );
    } catch (cause) {
      setError(spotMutationError(cause, "removed from favorites"));
    }
  }

  if (loading) {
    return (
      <section className="content-card" aria-labelledby="favorites-heading">
        <p className="section-label">Favorite breaks</p>
        <h2 id="favorites-heading">Loading favorite spots</h2>
        <p role="status">Correlating your private spots and favorites…</p>
      </section>
    );
  }

  return (
    <section className="content-card" aria-labelledby="favorites-heading">
      <div className="section-heading-row">
        <div>
          <p className="section-label">Favorite breaks</p>
          <h2 id="favorites-heading">Your favorite spots</h2>
        </div>
        {!showForm && (
          <button type="button" onClick={openCreateForm}>
            Add a favorite spot
          </button>
        )}
      </div>
      <p className="card-copy">
        Coordinates stay in your private API and are never added to the offline
        cache.
      </p>

      {error !== null && (
        <div
          className="feedback error"
          role="alert"
          tabIndex={-1}
          ref={alertRef}
        >
          <p>{error}</p>
          {pendingFavorite !== null && (
            <button
              type="button"
              disabled={saving}
              onClick={() => void retryFavorite()}
            >
              {saving ? "Adding favorite…" : "Retry adding favorite"}
            </button>
          )}
          {error.includes("could not load") && (
            <button type="button" onClick={() => void loadFavorites()}>
              Retry favorite spots
            </button>
          )}
          {error.includes("changed on the server") && (
            <button type="button" onClick={() => void loadFavorites()}>
              Reload favorite spots
            </button>
          )}
        </div>
      )}
      {notice !== null && (
        <p className="feedback success" role="status">
          {notice}
        </p>
      )}

      {showForm && (
        <SpotFormFields
          form={form}
          saving={saving}
          editing={editingSpot !== null}
          nameRef={nameRef}
          onChange={setForm}
          onCancel={closeForm}
          onSubmit={(event) => void saveSpot(event)}
        />
      )}

      {favoriteSpots.length === 0 && !showForm ? (
        <div className="empty-state">
          <h3>No favorite spots yet.</h3>
          <p>Add the first beach you want to check at a glance.</p>
        </div>
      ) : (
        <ul className="spot-list" aria-label="Favorite spots">
          {favoriteSpots.map(({ spot }) => (
            <li key={spot.id} className="spot-card">
              <div>
                <h3>{spot.name}</h3>
                <p>{spot.timeZone}</p>
                <p className="coordinates">
                  {spot.latitude.toFixed(4)}, {spot.longitude.toFixed(4)}
                </p>
              </div>
              <div className="spot-actions">
                <button type="button" onClick={() => openEditForm(spot)}>
                  Edit {spot.name}
                </button>
                <button
                  type="button"
                  className="secondary-button"
                  onClick={() => void removeFavorite(spot)}
                >
                  Remove {spot.name} from favorites
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

type SpotFormFieldsProps = {
  form: SpotForm;
  saving: boolean;
  editing: boolean;
  nameRef: React.RefObject<HTMLInputElement | null>;
  onChange: (form: SpotForm) => void;
  onCancel: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function SpotFormFields({
  form,
  saving,
  editing,
  nameRef,
  onChange,
  onCancel,
  onSubmit,
}: SpotFormFieldsProps) {
  return (
    <form className="stacked-form spot-form" noValidate onSubmit={onSubmit}>
      <h3>{editing ? "Edit favorite spot" : "New favorite spot"}</h3>
      <label>
        <span>Spot name</span>
        <input
          ref={nameRef}
          required
          maxLength={120}
          autoComplete="off"
          value={form.name}
          onChange={(event) => onChange({ ...form, name: event.target.value })}
        />
      </label>
      <div className="coordinate-fields">
        <label>
          <span>Latitude</span>
          <input
            required
            type="number"
            inputMode="decimal"
            step="any"
            min="-90"
            max="90"
            value={form.latitude}
            onChange={(event) =>
              onChange({ ...form, latitude: event.target.value })
            }
          />
        </label>
        <label>
          <span>Longitude</span>
          <input
            required
            type="number"
            inputMode="decimal"
            step="any"
            min="-180"
            max="180"
            value={form.longitude}
            onChange={(event) =>
              onChange({ ...form, longitude: event.target.value })
            }
          />
        </label>
      </div>
      <label>
        <span>Time zone</span>
        <input
          required
          maxLength={255}
          autoComplete="off"
          placeholder="Europe/Lisbon"
          value={form.timeZone}
          onChange={(event) =>
            onChange({ ...form, timeZone: event.target.value })
          }
        />
      </label>
      <div className="form-actions">
        <button type="submit" disabled={saving}>
          {saving
            ? "Saving spot…"
            : editing
              ? "Save spot changes"
              : "Save favorite spot"}
        </button>
        <button type="button" className="secondary-button" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}

function toSpotWrite(form: SpotForm): SpotWrite | null {
  const latitude = Number(form.latitude);
  const longitude = Number(form.longitude);
  const name = form.name.trim();
  const timeZone = form.timeZone.trim();

  if (
    name.length === 0 ||
    timeZone.length === 0 ||
    !isTimeZone(timeZone) ||
    !Number.isFinite(latitude) ||
    latitude < -90 ||
    latitude > 90 ||
    !Number.isFinite(longitude) ||
    longitude < -180 ||
    longitude > 180
  ) {
    return null;
  }

  return { name, latitude, longitude, timeZone };
}

function isTimeZone(value: string): boolean {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: value });
    return true;
  } catch {
    return false;
  }
}

function nextSortPosition(favorites: Favorite[]): number {
  return favorites.reduce(
    (next, favorite) => Math.max(next, favorite.sortPosition + 1),
    0,
  );
}

function spotMutationError(cause: unknown, action: string): string {
  if (cause instanceof ApiError) {
    if (cause.kind === "conflict") {
      return `This spot changed on the server and could not be ${action}. Reload favorite spots before trying again.`;
    }
    if (cause.kind === "validation") {
      return `The spot values are invalid and it could not be ${action}.`;
    }
    if (cause.kind === "unavailable") {
      return `The server is unavailable, so the spot could not be ${action}.`;
    }
  }
  return `The spot could not be ${action}. Try again.`;
}
