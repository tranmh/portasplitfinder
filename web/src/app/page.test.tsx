import { describe, it, expect, vi } from "vitest";
import { render, screen, within, fireEvent } from "@testing-library/react";

// Leaflet needs a browser; the dynamic map is covered by Playwright. Render a
// placeholder here so the page mounts in jsdom without importing leaflet.
vi.mock("next/dynamic", () => ({
  default: () => () => <div data-testid="map-placeholder" />,
}));

// Serve the page deterministic data without hitting the network.
vi.mock("@/hooks/useAvailability", () => ({
  useAvailability: () => ({
    data: availabilityFixture,
    loading: false,
    error: null,
    refetch: () => {},
  }),
}));

import HomePage from "./page";
import { getMessages } from "@/i18n";
import { availabilityFixture } from "@/test/fixtures";

const messages = getMessages("de");

describe("HomePage in-stock filter", () => {
  it("lists both in-stock and out-of-stock stores by default", () => {
    render(<HomePage />);
    expect(screen.getByText(/OBI Markt München/)).toBeInTheDocument();
    expect(screen.getByText(/OBI Markt Berlin-Spandau/)).toBeInTheDocument();
  });

  it("hides out-of-stock stores when the toggle is checked", () => {
    render(<HomePage />);
    const toggle = screen.getByLabelText(messages.filter.inStockOnly);
    fireEvent.click(toggle);

    // München (in stock) stays; Berlin-Spandau (out of stock) is filtered out.
    expect(screen.getByText(/OBI Markt München/)).toBeInTheDocument();
    expect(screen.queryByText(/OBI Markt Berlin-Spandau/)).not.toBeInTheDocument();
  });

  it("restores all stores when the toggle is unchecked again", () => {
    render(<HomePage />);
    const toggle = screen.getByLabelText(messages.filter.inStockOnly);
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    expect(screen.getByText(/OBI Markt Berlin-Spandau/)).toBeInTheDocument();
  });
});
