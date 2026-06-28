import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { SubscribeForm } from "./SubscribeForm";
import { getMessages } from "@/i18n";

const messages = getMessages("de");

function fill(email: string, plz: string) {
  fireEvent.change(screen.getByLabelText(messages.subscribe.emailLabel), {
    target: { value: email },
  });
  fireEvent.change(screen.getByLabelText(messages.subscribe.plzLabel), {
    target: { value: plz },
  });
}

function submit() {
  fireEvent.click(screen.getByRole("button", { name: messages.subscribe.submit }));
}

describe("SubscribeForm", () => {
  const originalFetch = global.fetch;
  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("shows client validation errors and does not call fetch", () => {
    const fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof fetch;

    render(<SubscribeForm messages={messages} />);
    fill("not-an-email", "12");
    submit();

    expect(screen.getByText(messages.subscribe.emailInvalid)).toBeInTheDocument();
    expect(screen.getByText(messages.subscribe.plzInvalid)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows the success message on a 202 response", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ status: "pending_confirmation" }), {
        status: 202,
        headers: { "Content-Type": "application/json" },
      }),
    ) as unknown as typeof fetch;

    render(<SubscribeForm messages={messages} initialPlz="10115" />);
    fill("a@b.de", "10115");
    submit();

    await waitFor(() => {
      expect(screen.getByText(messages.subscribe.success)).toBeInTheDocument();
    });
  });

  it("surfaces a server field validation error", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: "validation_error",
          message: "E-Mail ist gesperrt",
          field: "email",
        }),
        { status: 400, headers: { "Content-Type": "application/json" } },
      ),
    ) as unknown as typeof fetch;

    render(<SubscribeForm messages={messages} />);
    fill("a@b.de", "10115");
    submit();

    await waitFor(() => {
      expect(screen.getByText("E-Mail ist gesperrt")).toBeInTheDocument();
    });
    expect(screen.queryByText(messages.subscribe.success)).not.toBeInTheDocument();
  });

  it("shows a generic error when the server returns a non-field error", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "rate_limited", message: "" }), {
        status: 429,
        headers: { "Content-Type": "application/json" },
      }),
    ) as unknown as typeof fetch;

    render(<SubscribeForm messages={messages} />);
    fill("a@b.de", "10115");
    submit();

    await waitFor(() => {
      expect(screen.getByText(messages.subscribe.genericError)).toBeInTheDocument();
    });
  });
});
