import { expect, test } from "@playwright/test";

const timestamp = "2026-08-27T10:00:00Z";
const spotId = "a77e3c45-9d6c-4e7f-896f-25bf4f0b8ee6";

test("sets up a profile and favorite spot without caching private API data", async ({
  page,
}) => {
  let profile: Record<string, unknown> | undefined;
  let spot: Record<string, unknown> | undefined;
  let favorite: Record<string, unknown> | undefined;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();

    if (path === "/api/health/ready") {
      await route.fulfill({ json: { status: "ready" } });
      return;
    }
    if (path === "/api/version") {
      await route.fulfill({
        json: { version: "0.1.0", commit: "browser-test" },
      });
      return;
    }
    if (path === "/api/profile" && method === "GET") {
      await route.fulfill(
        profile === undefined
          ? { status: 404, json: { code: "not_found", message: "not found" } }
          : { json: profile },
      );
      return;
    }
    if (path === "/api/profile" && method === "POST") {
      profile = {
        ...(request.postDataJSON() as Record<string, unknown>),
        version: 1,
        createdAt: timestamp,
        updatedAt: timestamp,
      };
      await route.fulfill({ status: 201, json: profile });
      return;
    }
    if (path === "/api/spots" && method === "GET") {
      await route.fulfill({
        json: { items: spot === undefined ? [] : [spot] },
      });
      return;
    }
    if (path === "/api/spots" && method === "POST") {
      spot = {
        ...(request.postDataJSON() as Record<string, unknown>),
        id: spotId,
        version: 1,
        createdAt: timestamp,
        updatedAt: timestamp,
      };
      await route.fulfill({ status: 201, json: spot });
      return;
    }
    if (path === "/api/favorites" && method === "GET") {
      await route.fulfill({
        json: { items: favorite === undefined ? [] : [favorite] },
      });
      return;
    }
    if (path === "/api/favorites" && method === "POST") {
      favorite = {
        ...(request.postDataJSON() as Record<string, unknown>),
        createdAt: timestamp,
        updatedAt: timestamp,
      };
      await route.fulfill({ status: 201, json: favorite });
      return;
    }
    if (path === `/api/favorites/${spotId}` && method === "DELETE") {
      favorite = undefined;
      await route.fulfill({ status: 204, body: "" });
      return;
    }

    await route.fulfill({
      status: 404,
      json: { code: "not_found", message: "route not mocked" },
    });
  });

  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Set up your surf profile" }),
  ).toBeVisible();
  await page.getByLabel("Experience level").selectOption("intermediate");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(
    page.getByRole("heading", { name: "Your surf profile" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Add a favorite spot" }).click();
  await page.getByLabel("Spot name").fill("Supertubos");
  await page.getByLabel("Latitude").fill("39.34");
  await page.getByLabel("Longitude").fill("-9.36");
  await page.getByLabel("Time zone").fill("Europe/Lisbon");
  await page.getByRole("button", { name: "Save favorite spot" }).click();

  await expect(page.getByRole("heading", { name: "Supertubos" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Supertubos" })).toBeVisible();

  await page
    .getByRole("button", { name: "Remove Supertubos from favorites" })
    .click();
  await expect(page.getByText("No favorite spots yet.")).toBeVisible();
  expect(spot).toBeDefined();

  await page.evaluate(async () => navigator.serviceWorker.ready);
  const cachedEntries = await page.evaluate(async () => {
    const values: Array<{ url: string; body: string }> = [];
    for (const cacheName of await caches.keys()) {
      const cache = await caches.open(cacheName);
      for (const request of await cache.keys()) {
        values.push({
          url: request.url,
          body: await (await cache.match(request))!.text(),
        });
      }
    }
    return values;
  });

  expect(cachedEntries.map(({ url }) => url)).not.toEqual(
    expect.arrayContaining([expect.stringContaining("/api/")]),
  );
  const cachedBodies = cachedEntries.map(({ body }) => body).join("\n");
  expect(cachedBodies).not.toContain("39.34");
  expect(cachedBodies).not.toContain("-9.36");
});
