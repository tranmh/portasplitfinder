import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { PlzFilter } from "./PlzFilter";
import { getMessages } from "@/i18n";

const messages = getMessages("de");

describe("PlzFilter", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  function typePlz(value: string) {
    const input = screen.getByLabelText(messages.filter.plzLabel);
    fireEvent.change(input, { target: { value } });
  }

  it("shows a validation error for an incomplete PLZ and does not emit it", () => {
    const onChange = vi.fn();
    render(<PlzFilter messages={messages} onChange={onChange} debounceMs={300} />);
    onChange.mockClear(); // ignore the initial mount emit

    typePlz("123");
    act(() => {
      vi.advanceTimersByTime(400);
    });

    expect(screen.getByText(messages.filter.plzInvalid)).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("debounces and emits a valid 5-digit PLZ", () => {
    const onChange = vi.fn();
    render(<PlzFilter messages={messages} onChange={onChange} debounceMs={300} />);
    onChange.mockClear();

    typePlz("10115");

    // Before debounce fires, nothing emitted.
    expect(onChange).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ plz: "10115", radius: 50 });
  });

  it("strips non-digit characters from input", () => {
    const onChange = vi.fn();
    render(<PlzFilter messages={messages} onChange={onChange} debounceMs={300} />);
    typePlz("1a2b3c");
    const input = screen.getByLabelText(messages.filter.plzLabel) as HTMLInputElement;
    expect(input.value).toBe("123");
  });

  it("emits an empty PLZ when cleared (to reset the parent)", () => {
    const onChange = vi.fn();
    render(<PlzFilter messages={messages} onChange={onChange} debounceMs={300} />);

    typePlz("10115");
    act(() => vi.advanceTimersByTime(300));
    onChange.mockClear();

    typePlz("");
    act(() => vi.advanceTimersByTime(300));

    expect(onChange).toHaveBeenCalledWith({ plz: "", radius: 50 });
  });
});
