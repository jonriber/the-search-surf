import { describe, expect, it } from "vitest";

import type { AvailableStatus } from "./serviceStatus";
import { readCachedStatus, writeCachedStatus } from "./statusCache";

describe("status cache", () => {
  it("round trips a validated available status", () => {
    const storage = new MemoryStorage();
    const status: AvailableStatus = {
      kind: "available",
      checkedAt: "2026-08-23T12:00:00.000Z",
      build: { version: "1.2.3", commit: "abc123" },
    };

    writeCachedStatus(status, storage);

    expect(readCachedStatus(storage)).toEqual(status);
  });

  it("rejects and removes an incompatible cached contract", () => {
    const storage = new MemoryStorage();
    storage.setItem(
      "the-search:last-available-status",
      JSON.stringify({ kind: "available" }),
    );

    expect(readCachedStatus(storage)).toBeNull();
    expect(storage.length).toBe(0);
  });
});

class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>();

  get length(): number {
    return this.values.size;
  }

  clear(): void {
    this.values.clear();
  }

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  key(index: number): string | null {
    return [...this.values.keys()][index] ?? null;
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }
}
