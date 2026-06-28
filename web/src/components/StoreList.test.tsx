import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { StoreList } from "./StoreList";
import { getMessages } from "@/i18n";
import { storesFixture } from "@/test/fixtures";

const messages = getMessages("de");

describe("StoreList", () => {
  it("renders stores sorted nearest-first by distance", () => {
    render(<StoreList stores={storesFixture} messages={messages} hasCenter />);
    const items = screen.getAllByRole("listitem");
    // Berlin (3.1 km) should come before München (12.3 km).
    const first = within(items[0]);
    expect(first.getByText(/Berlin-Spandau/)).toBeInTheDocument();
  });

  it("shows distance per store", () => {
    render(<StoreList stores={storesFixture} messages={messages} hasCenter />);
    expect(screen.getByText(/3,1\s*km/)).toBeInTheDocument();
    expect(screen.getByText(/12,3\s*km/)).toBeInTheDocument();
  });

  it("renders 'Preis unbekannt' for a null price listing", () => {
    render(<StoreList stores={storesFixture} messages={messages} hasCenter />);
    expect(screen.getByText(messages.online.priceUnknown)).toBeInTheDocument();
    expect(screen.getByText(/479,00/)).toBeInTheDocument();
  });

  it("shows the no-center hint when no PLZ is set and list is empty", () => {
    render(<StoreList stores={[]} messages={messages} hasCenter={false} />);
    expect(screen.getByText(messages.stores.noCenter)).toBeInTheDocument();
  });

  it("shows the empty-radius message when a center is set but no stores", () => {
    render(<StoreList stores={[]} messages={messages} hasCenter />);
    expect(screen.getByText(messages.stores.none)).toBeInTheDocument();
  });
});
