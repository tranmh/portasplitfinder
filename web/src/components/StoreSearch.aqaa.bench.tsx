import { bench, describe } from "vitest";
import { act, fireEvent, render } from "@testing-library/react";
import { StoreSearch } from "./StoreSearch";
import { getMessages } from "@/i18n";
import type { StoreAvailability, StoreSearchResponse } from "@/lib/api-client";

// searchStores is driven through a stubbed fetch that answers synchronously
// with pre-built pages, so each iteration measures render + submit + parse.

const messages = getMessages("de");

function makeStores(offset: number, n: number): StoreAvailability[] {
  const stores: StoreAvailability[] = [];
  for (let i = offset; i < offset + n; i++) {
    const id = `store-${i}`;
    stores.push({
      store: {
        id,
        sourceKey: "obi",
        name: `OBI Markt ${i}`,
        street: `Hauptstraße ${i}`,
        plz: String(10000 + i).padStart(5, "0"),
        city: `Stadt ${i}`,
        lat: 48,
        lon: 9,
      },
      distanceKm: i % 7 === 0 ? null : i * 0.5,
      listings: [
        {
          sourceKey: "obi",
          productId: "p1",
          storeId: id,
          inStock: i % 2 === 0,
          quantity: 1,
          priceCents: 47900,
          currency: "EUR",
          url: `https://example.test/p1?store=${i}`,
          checkedAt: "2026-10-26T12:00:00Z",
        },
      ],
    });
  }
  return stores;
}

/** Pages 1..pageCount of pageSize stores each; total = pageSize * pageCount. */
function makePages(pageSize: number, pageCount: number): StoreSearchResponse[] {
  const pages: StoreSearchResponse[] = [];
  for (let p = 0; p < pageCount; p++) {
    pages.push({
      stores: makeStores(p * pageSize, pageSize),
      total: pageSize * pageCount,
      page: p + 1,
      pageSize,
    });
  }
  return pages;
}

function stubFetchPages(pages: StoreSearchResponse[]): void {
  globalThis.fetch = ((url: string) => {
    const page = Number(new URL(url).searchParams.get("page") ?? "1");
    const body = pages[page - 1];
    return Promise.resolve({ ok: true, status: 200, statusText: "OK", json: () => Promise.resolve(body) });
  }) as unknown as typeof fetch;
}

function stubFetchError(): void {
  globalThis.fetch = (() =>
    Promise.resolve({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ error: "internal" }),
    })) as unknown as typeof fetch;
}

async function flush(): Promise<void> {
  for (let i = 0; i < 30; i++) await Promise.resolve();
}

let sink: unknown;

async function submitSearch(plz: string): Promise<ReturnType<typeof render>> {
  const view = render(<StoreSearch messages={messages} plz={plz} />);
  const input = view.container.querySelector("input")!;
  fireEvent.change(input, { target: { value: " obi " } });
  await act(async () => {
    fireEvent.submit(view.container.querySelector("form")!);
    await flush();
  });
  return view;
}

async function searchAndCount(plz: string, expected: number): Promise<void> {
  const view = await submitSearch(plz);
  const count = view.container.querySelectorAll("li").length;
  if (count !== expected) throw new Error(`expected ${expected} results, got ${count}`);
  sink = count;
  view.unmount();
}

describe("StoreSearch: submit renders a page of results", () => {
  const small = makePages(10, 1);
  const medium = makePages(100, 1);
  const large = makePages(500, 1);

  bench("10 results", async () => {
    stubFetchPages(small);
    await searchAndCount("73035", 10);
  });
  bench("100 results", async () => {
    stubFetchPages(medium);
    await searchAndCount("73035", 100);
  });
  bench("500 results", async () => {
    stubFetchPages(large);
    await searchAndCount("73035", 500);
  });
});

describe("StoreSearch: load more appends pages until all are loaded", () => {
  async function loadAll(pages: StoreSearchResponse[]): Promise<void> {
    stubFetchPages(pages);
    const view = await submitSearch("73035");
    for (let p = 1; p < pages.length; p++) {
      const button = view.container.querySelector('button[type="button"]')!;
      await act(async () => {
        fireEvent.click(button);
        await flush();
      });
    }
    const count = view.container.querySelectorAll("li").length;
    const total = pages[0].total;
    if (count !== total) throw new Error(`expected ${total} results, got ${count}`);
    if (view.container.querySelector('button[type="button"]')) throw new Error("load more still shown");
    sink = count;
    view.unmount();
  }

  const two = makePages(20, 2);
  const five = makePages(20, 5);
  const twenty = makePages(20, 20);

  bench("2 pages x 20", async () => {
    await loadAll(two);
  });
  bench("5 pages x 20", async () => {
    await loadAll(five);
  });
  bench("20 pages x 20", async () => {
    await loadAll(twenty);
  });
});

describe("StoreSearch: error, empty result and plz handling", () => {
  const empty = makePages(0, 1);
  const one = makePages(1, 1);

  bench("error shows the message", async () => {
    stubFetchError();
    const view = await submitSearch("73035");
    if (!view.container.textContent!.includes(messages.storeSearch.error)) throw new Error("no error shown");
    sink = view.container.textContent;
    view.unmount();
  });
  bench("empty result shows noResults", async () => {
    stubFetchPages(empty);
    const view = await submitSearch("73035");
    if (!view.container.textContent!.includes(messages.storeSearch.noResults)) throw new Error("no noResults");
    sink = view.container.textContent;
    view.unmount();
  });
  bench("valid plz", async () => {
    stubFetchPages(one);
    await searchAndCount(" 73035 ", 1);
  });
  bench("invalid plz omitted", async () => {
    stubFetchPages(one);
    await searchAndCount("abc", 1);
  });
});

export { sink };
