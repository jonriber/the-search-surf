import type { ZodType } from "zod";

import type {
  Favorite,
  FavoritePosition,
  FavoriteWrite,
  Profile,
  ProfileUpdate,
  ProfileWrite,
  Spot,
  SpotUpdate,
  SpotWrite,
} from "./generated/types.gen";
import {
  zAddFavoriteResponse,
  zCreateProfileResponse,
  zCreateSpotResponse,
  zError,
  zGetProfileResponse,
  zListFavoritesResponse,
  zListSpotsResponse,
  zUpdateFavoritePositionResponse,
  zUpdateProfileResponse,
  zUpdateSpotResponse,
} from "./generated/zod.gen";

export type ApiFailureKind =
  | "validation"
  | "not-found"
  | "conflict"
  | "unavailable"
  | "incompatible"
  | "unknown";

export class ApiError extends Error {
  readonly kind: ApiFailureKind;
  readonly status?: number;
  readonly code?: string;

  constructor(
    kind: ApiFailureKind,
    message: string,
    options: { status?: number; code?: string; cause?: unknown } = {},
  ) {
    super(message, { cause: options.cause });
    this.name = "ApiError";
    this.kind = kind;
    this.status = options.status;
    this.code = options.code;
  }
}

export type UserDataClient = {
  getProfile(): Promise<Profile>;
  createProfile(profile: ProfileWrite): Promise<Profile>;
  updateProfile(profile: ProfileUpdate): Promise<Profile>;
  listSpots(): Promise<Spot[]>;
  createSpot(spot: SpotWrite): Promise<Spot>;
  updateSpot(spotId: string, spot: SpotUpdate): Promise<Spot>;
  deleteSpot(spotId: string, expectedVersion: number): Promise<void>;
  listFavorites(): Promise<Favorite[]>;
  addFavorite(favorite: FavoriteWrite): Promise<Favorite>;
  updateFavoritePosition(
    spotId: string,
    position: FavoritePosition,
  ): Promise<Favorite>;
  removeFavorite(spotId: string): Promise<void>;
};

type ClientOptions = {
  fetch?: typeof fetch;
  timeoutMilliseconds?: number;
};

type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: object;
};

const publicMessages: Record<ApiFailureKind, string> = {
  validation: "The request contains invalid data.",
  "not-found": "The requested data could not be found.",
  conflict: "This data changed since it was loaded.",
  unavailable: "The server is temporarily unavailable.",
  incompatible: "The server response is incompatible with this app.",
  unknown: "The request could not be completed.",
};

export function createUserDataClient({
  fetch: fetcher = globalThis.fetch,
  timeoutMilliseconds = 5_000,
}: ClientOptions = {}): UserDataClient {
  async function request(
    path: string,
    options: RequestOptions = {},
  ): Promise<unknown> {
    const controller = new AbortController();
    const timeout = globalThis.setTimeout(
      () => controller.abort(),
      timeoutMilliseconds,
    );

    try {
      const response = await fetcher(`/api${path}`, {
        method: options.method ?? "GET",
        headers: {
          Accept: "application/json",
          ...(options.body === undefined
            ? {}
            : { "Content-Type": "application/json" }),
        },
        body:
          options.body === undefined ? undefined : JSON.stringify(options.body),
        signal: controller.signal,
      });

      if (!response.ok) {
        throw await toApiError(response);
      }

      if (response.status === 204) {
        return undefined;
      }

      try {
        return await response.json();
      } catch (cause) {
        throw new ApiError("incompatible", publicMessages.incompatible, {
          cause,
        });
      }
    } catch (error) {
      if (error instanceof ApiError) {
        throw error;
      }

      throw new ApiError("unavailable", publicMessages.unavailable, {
        cause: error,
      });
    } finally {
      globalThis.clearTimeout(timeout);
    }
  }

  return {
    async getProfile() {
      return parseVersioned(zGetProfileResponse, await request("/profile"));
    },
    async createProfile(profile) {
      return parseVersioned(
        zCreateProfileResponse,
        await request("/profile", { method: "POST", body: profile }),
      );
    },
    async updateProfile(profile) {
      assertSafeVersion(profile.expectedVersion);
      return parseVersioned(
        zUpdateProfileResponse,
        await request("/profile", { method: "PUT", body: profile }),
      );
    },
    async listSpots() {
      const payload = await request("/spots");
      assertSafeVersions(payload, "items");
      const parsed = parse(zListSpotsResponse, payload);
      return parsed.items.map(normalizeVersion);
    },
    async createSpot(spot) {
      return parseVersioned(
        zCreateSpotResponse,
        await request("/spots", { method: "POST", body: spot }),
      );
    },
    async updateSpot(spotId, spot) {
      assertSafeVersion(spot.expectedVersion);
      return parseVersioned(
        zUpdateSpotResponse,
        await request(`/spots/${encodeURIComponent(spotId)}`, {
          method: "PUT",
          body: spot,
        }),
      );
    },
    async deleteSpot(spotId, expectedVersion) {
      assertSafeVersion(expectedVersion);
      await request(
        `/spots/${encodeURIComponent(spotId)}?expectedVersion=${String(expectedVersion)}`,
        { method: "DELETE" },
      );
    },
    async listFavorites() {
      return parse(zListFavoritesResponse, await request("/favorites")).items;
    },
    async addFavorite(favorite) {
      return parse(
        zAddFavoriteResponse,
        await request("/favorites", { method: "POST", body: favorite }),
      );
    },
    async updateFavoritePosition(spotId, position) {
      return parse(
        zUpdateFavoritePositionResponse,
        await request(`/favorites/${encodeURIComponent(spotId)}`, {
          method: "PUT",
          body: position,
        }),
      );
    },
    async removeFavorite(spotId) {
      await request(`/favorites/${encodeURIComponent(spotId)}`, {
        method: "DELETE",
      });
    },
  };
}

async function toApiError(response: Response): Promise<ApiError> {
  let code: string | undefined;
  try {
    const parsed = zError.safeParse(await response.json());
    code = parsed.success ? parsed.data.code : undefined;
  } catch {
    // A proxy or unavailable upstream may return HTML or an empty body.
  }

  const kind = statusKind(response.status);
  return new ApiError(kind, publicMessages[kind], {
    status: response.status,
    code,
  });
}

function statusKind(status: number): ApiFailureKind {
  if (status === 400 || status === 415 || status === 422) return "validation";
  if (status === 404) return "not-found";
  if (status === 409) return "conflict";
  if (status >= 500) return "unavailable";
  return "unknown";
}

function parse<Output>(schema: ZodType<Output>, payload: unknown): Output {
  const result = schema.safeParse(payload);
  if (!result.success) {
    throw new ApiError("incompatible", publicMessages.incompatible, {
      cause: result.error,
    });
  }
  return result.data;
}

function parseVersioned<Output extends { version: bigint }>(
  schema: ZodType<Output>,
  payload: unknown,
): Omit<Output, "version"> & { version: number } {
  assertSafeVersions(payload);
  return normalizeVersion(parse(schema, payload));
}

function normalizeVersion<Output extends { version: bigint }>(
  value: Output,
): Omit<Output, "version"> & { version: number } {
  return { ...value, version: Number(value.version) };
}

function assertSafeVersions(payload: unknown, collectionKey?: string): void {
  if (collectionKey !== undefined) {
    if (!isRecord(payload) || !Array.isArray(payload[collectionKey])) {
      return;
    }
    for (const item of payload[collectionKey]) {
      assertSafeWireVersion(item);
    }
    return;
  }
  assertSafeWireVersion(payload);
}

function assertSafeWireVersion(value: unknown): void {
  if (!isRecord(value) || !Number.isSafeInteger(value.version)) {
    throw new ApiError("incompatible", publicMessages.incompatible);
  }
}

function assertSafeVersion(version: number): void {
  if (!Number.isSafeInteger(version) || version < 1) {
    throw new ApiError("validation", publicMessages.validation);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
