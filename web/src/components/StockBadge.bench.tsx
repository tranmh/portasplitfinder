import { bench, describe } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { StockBadge } from "./StockBadge";

const SIZES = [10, 100, 1_000] as const;

let sink: unknown;

describe("StockBadge: render a list of badges", () => {
  for (const n of SIZES) {
    // Alternating in-stock / out-of-stock, like a store list.
    const states = Array.from({ length: n }, (_, i) => i % 2 === 0);

    bench(`render ${n} badges`, () => {
      sink = renderToStaticMarkup(
        <ul>
          {states.map((inStock, i) => (
            <li key={i}>
              <StockBadge inStock={inStock} inStockLabel="Auf Lager" outOfStockLabel="Nicht verfügbar" />
            </li>
          ))}
        </ul>,
      );
    });
  }
});

export { sink };
