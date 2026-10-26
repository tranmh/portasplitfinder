import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StoreSearch } from "./StoreSearch";
import { getMessages } from "@/i18n";
import { searchStores, type StoreSearchResponse } from "@/lib/api-client";
import {
  buildListing,
  buildStore,
  buildStoreAvailability,
  buildStoreSearchResponse,
} from "@/test/__tests__/doubles";

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return { ...actual, searchStores: vi.fn() };
});

const searchMock = vi.mocked(searchStores);
const messages = getMessages("de");
const t = messages.storeSearch;

function storesPage(ids: string[]) {
  return ids.map((id) =>
    buildStoreAvailability({ store: buildStore({ id, name: `Markt ${id}` }) }),
  );
}

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function search(term: string) {
  const user = userEvent.setup();
  const input = screen.getByPlaceholderText(t.placeholder);
  await user.clear(input);
  if (term) await user.type(input, term);
  await user.click(submitButton());
  return user;
}

function submitButton(): HTMLButtonElement {
  return screen.getByPlaceholderText(t.placeholder).closest("form")!.querySelector('button[type="submit"]')!;
}

describe("StoreSearch", () => {
  beforeEach(() => {
    searchMock.mockReset();
  });

  it("renders heading and search input without results initially", () => {
    render(<StoreSearch messages={messages} plz="" />);
    expect(screen.getByRole("heading", { name: t.heading })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(t.placeholder)).toHaveValue("");
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
    expect(screen.queryByText(t.noResults)).not.toBeInTheDocument();
    expect(searchMock).not.toHaveBeenCalled();
  });

  it("submits the trimmed term on page 1 and sends a valid plz", async () => {
    searchMock.mockResolvedValue(buildStoreSearchResponse());
    render(<StoreSearch messages={messages} plz=" 10115 " />);

    await search("  obi  ");

    expect(searchMock).toHaveBeenCalledTimes(1);
    expect(searchMock).toHaveBeenCalledWith({ q: "obi", plz: "10115", page: 1 });
  });

  it.each(["", "1234", "123456", "abcde"])("omits an invalid plz %j", async (plz) => {
    searchMock.mockResolvedValue(buildStoreSearchResponse());
    render(<StoreSearch messages={messages} plz={plz} />);

    await search("Berlin");

    expect(searchMock).toHaveBeenCalledWith({ q: "Berlin", plz: undefined, page: 1 });
  });

  it("submits with the Enter key", async () => {
    searchMock.mockResolvedValue(buildStoreSearchResponse());
    render(<StoreSearch messages={messages} plz="" />);
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText(t.placeholder), "Stuttgart{Enter}");

    expect(searchMock).toHaveBeenCalledWith({ q: "Stuttgart", plz: undefined, page: 1 });
  });

  it("shows nothing until the response arrives", async () => {
    const pending = deferred<StoreSearchResponse>();
    searchMock.mockReturnValue(pending.promise);
    render(<StoreSearch messages={messages} plz="" />);

    await search("obi");

    expect(screen.queryByText(new RegExp(t.results))).not.toBeInTheDocument();
    expect(screen.queryByText(t.noResults)).not.toBeInTheDocument();

    pending.resolve(buildStoreSearchResponse(storesPage(["a"])));
    expect(await screen.findByText("Markt a")).toBeInTheDocument();
  });

  it("shows the total with the results label and one row per store", async () => {
    searchMock.mockResolvedValue(
      buildStoreSearchResponse(storesPage(["a", "b"]), { total: 2 }),
    );
    render(<StoreSearch messages={messages} plz="" />);

    await search("Markt");

    expect(await screen.findByText(`2 ${t.results}`)).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.queryByText(t.noResults)).not.toBeInTheDocument();
  });

  it("shows the no-results message and a zero total when empty", async () => {
    searchMock.mockResolvedValue(buildStoreSearchResponse([], { total: 0 }));
    render(<StoreSearch messages={messages} plz="" />);

    await search("nichts");

    expect(await screen.findByText(t.noResults)).toBeInTheDocument();
    expect(screen.getByText(`0 ${t.results}`)).toBeInTheDocument();
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
    expect(screen.queryByText(t.loadMore)).not.toBeInTheDocument();
  });

  it("shows the error message when the request fails", async () => {
    searchMock.mockRejectedValue(new Error("boom"));
    render(<StoreSearch messages={messages} plz="" />);

    await search("obi");

    expect(await screen.findByText(t.error)).toBeInTheDocument();
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  });

  it("clears the error message after a successful retry", async () => {
    searchMock
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["a"])));
    render(<StoreSearch messages={messages} plz="" />);

    const user = await search("obi");
    expect(await screen.findByText(t.error)).toBeInTheDocument();

    await user.click(submitButton());

    expect(await screen.findByText("Markt a")).toBeInTheDocument();
    expect(screen.queryByText(t.error)).not.toBeInTheDocument();
  });

  describe("rows", () => {
    it("shows name, 'plz city' and the formatted distance", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([
          buildStoreAvailability({
            store: buildStore({ id: "s1", name: "OBI Göppingen", plz: "73035", city: "Göppingen" }),
            distanceKm: 4.2,
          }),
        ]),
      );
      render(<StoreSearch messages={messages} plz="73035" />);

      await search("obi");

      const row = within(await screen.findByRole("listitem"));
      expect(row.getByText("OBI Göppingen")).toBeInTheDocument();
      expect(row.getByText("73035 Göppingen · 4,2 km")).toBeInTheDocument();
    });

    it("omits the distance when it is not available", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([
          buildStoreAvailability({
            store: buildStore({ plz: "10115", city: "Berlin" }),
            distanceKm: null,
          }),
        ]),
      );
      render(<StoreSearch messages={messages} plz="" />);

      await search("Berlin");

      const row = within(await screen.findByRole("listitem"));
      const line = row.getByText(/10115 Berlin/);
      expect(line.textContent).toBe("10115 Berlin");
      expect(row.queryByText(/km/)).not.toBeInTheDocument();
    });

    it("shows a distance of 0 km", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([
          buildStoreAvailability({ store: buildStore({ plz: "10115", city: "Berlin" }), distanceKm: 0 }),
        ]),
      );
      render(<StoreSearch messages={messages} plz="10115" />);

      await search("Berlin");

      expect(await screen.findByText("10115 Berlin · 0 km")).toBeInTheDocument();
    });

    it("is in stock when any listing is in stock", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([
          buildStoreAvailability({
            listings: [
              buildListing({ inStock: false, quantity: 0 }),
              buildListing({ inStock: true, quantity: null }),
            ],
          }),
        ]),
      );
      render(<StoreSearch messages={messages} plz="" />);

      await search("obi");

      const row = within(await screen.findByRole("listitem"));
      expect(row.getByText(messages.stores.inStock)).toBeInTheDocument();
      expect(row.queryByText(messages.stores.outOfStock)).not.toBeInTheDocument();
    });

    it("is out of stock when no listing is in stock", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([
          buildStoreAvailability({
            listings: [buildListing({ inStock: false }), buildListing({ inStock: false })],
          }),
        ]),
      );
      render(<StoreSearch messages={messages} plz="" />);

      await search("obi");

      const row = within(await screen.findByRole("listitem"));
      expect(row.getByText(messages.stores.outOfStock)).toBeInTheDocument();
    });

    it("is out of stock when there are no listings", async () => {
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([buildStoreAvailability({ listings: [] })]),
      );
      render(<StoreSearch messages={messages} plz="" />);

      await search("obi");

      const row = within(await screen.findByRole("listitem"));
      expect(row.getByText(messages.stores.outOfStock)).toBeInTheDocument();
    });

    it("uses the English labels", async () => {
      const en = getMessages("en");
      searchMock.mockResolvedValue(
        buildStoreSearchResponse([buildStoreAvailability({ listings: [] })], { total: 1 }),
      );
      render(<StoreSearch messages={en} plz="" />);

      const user = userEvent.setup();
      await user.type(screen.getByPlaceholderText(en.storeSearch.placeholder), "obi{Enter}");

      expect(await screen.findByText(`1 ${en.storeSearch.results}`)).toBeInTheDocument();
      expect(screen.getByText(en.stores.outOfStock)).toBeInTheDocument();
    });
  });

  describe("paging", () => {
    it("shows load more while fewer results than total and appends the next page", async () => {
      searchMock
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["a", "b"]), { total: 3 }))
        .mockResolvedValueOnce(
          buildStoreSearchResponse(storesPage(["c"]), { total: 3, page: 2 }),
        );
      render(<StoreSearch messages={messages} plz="10115" />);

      const user = await search("Markt");
      await screen.findByText("Markt a");

      await user.click(screen.getByRole("button", { name: t.loadMore }));

      expect(await screen.findByText("Markt c")).toBeInTheDocument();
      expect(searchMock).toHaveBeenLastCalledWith({ q: "Markt", plz: "10115", page: 2 });
      expect(screen.getAllByRole("listitem").map((li) => li.querySelector("p")?.textContent)).toEqual([
        "Markt a",
        "Markt b",
        "Markt c",
      ]);
      expect(screen.queryByRole("button", { name: t.loadMore })).not.toBeInTheDocument();
    });

    it("hides load more when all results are loaded", async () => {
      searchMock.mockResolvedValue(buildStoreSearchResponse(storesPage(["a", "b"]), { total: 2 }));
      render(<StoreSearch messages={messages} plz="" />);

      await search("Markt");
      await screen.findByText("Markt a");

      expect(screen.queryByRole("button", { name: t.loadMore })).not.toBeInTheDocument();
    });

    it("keeps loaded results and shows an error when loading more fails", async () => {
      searchMock
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["a"]), { total: 2 }))
        .mockRejectedValueOnce(new Error("boom"));
      render(<StoreSearch messages={messages} plz="" />);

      const user = await search("Markt");
      await screen.findByText("Markt a");
      await user.click(screen.getByRole("button", { name: t.loadMore }));

      expect(await screen.findByText(t.error)).toBeInTheDocument();
      expect(screen.getByText("Markt a")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: t.loadMore })).toBeInTheDocument();
    });

    it("a new search replaces the results and restarts on page 1", async () => {
      searchMock
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["a"]), { total: 2 }))
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["b"]), { total: 2, page: 2 }))
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["x"]), { total: 1 }));
      render(<StoreSearch messages={messages} plz="" />);

      const user = await search("Markt");
      await screen.findByText("Markt a");
      await user.click(screen.getByRole("button", { name: t.loadMore }));
      await screen.findByText("Markt b");

      await search("Neu");

      expect(await screen.findByText("Markt x")).toBeInTheDocument();
      expect(searchMock).toHaveBeenLastCalledWith({ q: "Neu", plz: undefined, page: 1 });
      expect(screen.queryByText("Markt a")).not.toBeInTheDocument();
      expect(screen.queryByText("Markt b")).not.toBeInTheDocument();
      expect(screen.getByText(`1 ${t.results}`)).toBeInTheDocument();
    });

    it.skip("aqaa: open bug, see report — load more keeps the submitted term (B21)", async () => {
      // aqaa: suspected bug — load more reads the live input value instead of the
      // submitted search term, so editing the input without submitting and then
      // clicking load more appends page 2 of a different query (B21: "clicking
      // appends the next page" of the current search).
      searchMock
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["a"]), { total: 2 }))
        .mockResolvedValueOnce(buildStoreSearchResponse(storesPage(["b"]), { total: 2, page: 2 }));
      render(<StoreSearch messages={messages} plz="" />);

      const user = await search("Markt");
      await screen.findByText("Markt a");
      await user.type(screen.getByPlaceholderText(t.placeholder), " Berlin");
      await user.click(screen.getByRole("button", { name: t.loadMore }));

      await waitFor(() => expect(searchMock).toHaveBeenCalledTimes(2));
      expect(searchMock).toHaveBeenLastCalledWith({ q: "Markt", plz: undefined, page: 2 });
    });
  });
});
