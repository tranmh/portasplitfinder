import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StockBadge } from "./StockBadge";
import { getMessages, type Locale } from "@/i18n";

describe.each<Locale>(["de", "en"])("StockBadge (%s)", (locale) => {
  const t = getMessages(locale).stores;

  it("renders the in-stock label when inStock is true", () => {
    render(<StockBadge inStock inStockLabel={t.inStock} outOfStockLabel={t.outOfStock} />);
    expect(screen.getByText(t.inStock)).toBeInTheDocument();
    expect(screen.queryByText(t.outOfStock)).not.toBeInTheDocument();
  });

  it("renders the out-of-stock label when inStock is false", () => {
    render(
      <StockBadge inStock={false} inStockLabel={t.inStock} outOfStockLabel={t.outOfStock} />,
    );
    expect(screen.getByText(t.outOfStock)).toBeInTheDocument();
    expect(screen.queryByText(t.inStock)).not.toBeInTheDocument();
  });
});

describe("StockBadge", () => {
  it("uses the localized labels", () => {
    const de = getMessages("de").stores;
    const en = getMessages("en").stores;
    const { rerender } = render(
      <StockBadge inStock inStockLabel={de.inStock} outOfStockLabel={de.outOfStock} />,
    );
    expect(screen.getByText("Auf Lager")).toBeInTheDocument();
    rerender(<StockBadge inStock={false} inStockLabel={en.inStock} outOfStockLabel={en.outOfStock} />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("styles in-stock and out-of-stock badges differently", () => {
    const { rerender } = render(<StockBadge inStock inStockLabel="yes" outOfStockLabel="no" />);
    const inStockClass = screen.getByText("yes").className;
    rerender(<StockBadge inStock={false} inStockLabel="yes" outOfStockLabel="no" />);
    expect(screen.getByText("no").className).not.toBe(inStockClass);
  });
});
