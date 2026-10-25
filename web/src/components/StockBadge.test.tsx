import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import DefaultStockBadge, { StockBadge } from "./StockBadge";
import { getMessages } from "@/i18n";

const t = getMessages("de");

describe("StockBadge", () => {
  it("renders the in-stock label with the green style", () => {
    render(<StockBadge inStock inStockLabel={t.stores.inStock} outOfStockLabel={t.stores.outOfStock} />);
    const badge = screen.getByText("Auf Lager");
    expect(badge.tagName).toBe("SPAN");
    expect(badge).toHaveClass("bg-green-100", "text-green-800");
    expect(screen.queryByText("Nicht verfügbar")).not.toBeInTheDocument();
  });

  it("renders the out-of-stock label with the neutral style", () => {
    render(
      <StockBadge inStock={false} inStockLabel={t.stores.inStock} outOfStockLabel={t.stores.outOfStock} />,
    );
    const badge = screen.getByText("Nicht verfügbar");
    expect(badge).toHaveClass("bg-slate-100", "text-slate-600");
    expect(badge).not.toHaveClass("bg-green-100");
    expect(screen.queryByText("Auf Lager")).not.toBeInTheDocument();
  });

  it("uses whatever labels it is given (e.g. English)", () => {
    const en = getMessages("en");
    const { rerender } = render(
      <StockBadge inStock inStockLabel={en.map.inStock} outOfStockLabel={en.map.outOfStock} />,
    );
    expect(screen.getByText("In stock")).toBeInTheDocument();
    rerender(<StockBadge inStock={false} inStockLabel={en.map.inStock} outOfStockLabel={en.map.outOfStock} />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("is also the default export", () => {
    expect(DefaultStockBadge).toBe(StockBadge);
  });
});
