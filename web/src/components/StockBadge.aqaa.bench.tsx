import { bench, describe } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { StockBadge } from "./StockBadge";

function badges(n: number, inStock: (i: number) => boolean) {
  const items = [];
  for (let i = 0; i < n; i++) {
    items.push(
      <StockBadge key={i} inStock={inStock(i)} inStockLabel="Auf Lager" outOfStockLabel="Nicht auf Lager" />,
    );
  }
  return <div>{items}</div>;
}

let sink = "";

describe("StockBadge: render by number of badges", () => {
  const inSmall = badges(1, () => true);
  const inMedium = badges(100, () => true);
  const inLarge = badges(1000, () => true);
  const outLarge = badges(1000, () => false);
  const mixedLarge = badges(1000, (i) => i % 2 === 0);

  bench("1 in stock", () => {
    sink = renderToStaticMarkup(inSmall);
  });
  bench("100 in stock", () => {
    sink = renderToStaticMarkup(inMedium);
  });
  bench("1000 in stock", () => {
    sink = renderToStaticMarkup(inLarge);
  });
  bench("1000 out of stock", () => {
    sink = renderToStaticMarkup(outLarge);
  });
  bench("1000 mixed", () => {
    sink = renderToStaticMarkup(mixedLarge);
  });
});

export { sink };
