import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";

// next/dynamic with ssr:false renders only the loading fallback in jsdom (the
// real Leaflet map needs a browser; it is covered by Playwright). This is a
// smoke test that the panel mounts behind the dynamic-import guard without
// throwing and without importing leaflet at module load.
vi.mock("next/dynamic", () => ({
  default: () => () => <div data-testid="map-placeholder" />,
}));

import { MapPanel } from "./MapPanel";
import { getMessages } from "@/i18n";

const messages = getMessages("de");

describe("MapPanel", () => {
  it("mounts behind the dynamic import guard without crashing", () => {
    const { getByTestId } = render(
      <MapPanel stores={[]} center={null} radiusKm={50} messages={messages} />,
    );
    expect(getByTestId("map-placeholder")).toBeInTheDocument();
  });
});
