import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../api/openapi/the-search.yaml",
  output: process.env.THE_SEARCH_OPENAPI_OUTPUT ?? "src/api/generated",
  plugins: ["@hey-api/typescript", "zod"],
});
