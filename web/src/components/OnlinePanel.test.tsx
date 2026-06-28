import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { OnlinePanel } from "./OnlinePanel";
import { getMessages } from "@/i18n";
import { onlineFixture } from "@/test/fixtures";

const messages = getMessages("de");

describe("OnlinePanel", () => {
  it("renders online sources as 'deutschlandweit verfügbar'", () => {
    render(<OnlinePanel online={onlineFixture} messages={messages} />);
    expect(screen.getByText("Idealo")).toBeInTheDocument();
    expect(screen.getByText("Amazon")).toBeInTheDocument();
    expect(screen.getAllByText(messages.online.nationwide).length).toBeGreaterThan(0);
  });

  it("formats a known price and shows 'Preis unbekannt' for null", () => {
    render(<OnlinePanel online={onlineFixture} messages={messages} />);
    // 49900 cents -> 499,00 € (de-DE)
    expect(screen.getByText(/499,00/)).toBeInTheDocument();
    expect(screen.getByText(messages.online.priceUnknown)).toBeInTheDocument();
  });

  it("renders in-stock and out-of-stock badges", () => {
    render(<OnlinePanel online={onlineFixture} messages={messages} />);
    expect(screen.getByText(messages.online.inStock)).toBeInTheDocument();
    expect(screen.getByText(messages.online.outOfStock)).toBeInTheDocument();
  });

  it("shows the deep link and 'never' last-checked for un-scraped sources", () => {
    render(<OnlinePanel online={onlineFixture} messages={messages} />);
    const links = screen.getAllByText(messages.online.viewListing);
    expect(links.length).toBe(2);
    expect(screen.getByText(new RegExp(messages.online.never))).toBeInTheDocument();
  });

  it("renders an empty state when there are no online sources", () => {
    render(<OnlinePanel online={[]} messages={messages} />);
    expect(screen.getByText(messages.online.none)).toBeInTheDocument();
  });
});
