import { test, expect } from "@playwright/test";

/**
 * WCAG 2.2 SC 1.4.3 (AA): normal-size text needs a contrast ratio of at
 * least 4.5:1 against its background.
 */
test.describe("Subscribe form accessibility", () => {
  test("submit button text has a contrast ratio of at least 4.5:1", async ({
    page,
  }) => {
    await page.goto("/");

    const submit = page.getByRole("button", { name: "Benachrichtigen lassen" });
    await expect(submit).toBeVisible({ timeout: 15_000 });

    const ratio = await submit.evaluate((el) => {
      // Resolve any CSS colour (rgb, oklch, ...) to sRGB via a canvas.
      const ctx = document.createElement("canvas").getContext("2d")!;
      const toRgba = (color: string): number[] => {
        ctx.clearRect(0, 0, 1, 1);
        ctx.fillStyle = color;
        ctx.fillRect(0, 0, 1, 1);
        const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
        return [r, g, b, a / 255];
      };
      const luminance = ([r, g, b]: number[]): number => {
        const lin = (c: number) => {
          const s = c / 255;
          return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
        };
        return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
      };

      // First non-transparent background going up from the element.
      let bg = [255, 255, 255, 1];
      for (let node: Element | null = el; node; node = node.parentElement) {
        const c = toRgba(getComputedStyle(node).backgroundColor);
        if (c[3] > 0) {
          bg = c;
          break;
        }
      }
      const fg = toRgba(getComputedStyle(el).color);

      const l1 = luminance(fg);
      const l2 = luminance(bg);
      return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
    });

    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });
});
