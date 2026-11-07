import { expect, test, type Locator } from "@playwright/test";

/**
 * WCAG 2.2 SC 1.4.3 (contrast minimum): normal-size text needs a contrast
 * ratio of at least 4.5:1 against its background.
 */

async function contrastRatio(locator: Locator): Promise<number> {
  return locator.evaluate((el) => {
    const ctx = document.createElement("canvas").getContext("2d")!;
    const rgba = (css: string): [number, number, number, number] => {
      ctx.clearRect(0, 0, 1, 1);
      ctx.fillStyle = css;
      ctx.fillRect(0, 0, 1, 1);
      const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
      return [r, g, b, a / 255];
    };
    const over = (top: number[], bottom: number[]) =>
      [0, 1, 2].map((i) => top[i] * top[3] + bottom[i] * (1 - top[3]));

    // Composite the element's and its ancestors' backgrounds onto white.
    const layers: [number, number, number, number][] = [];
    for (let node: Element | null = el; node; node = node.parentElement) {
      const bg = rgba(getComputedStyle(node).backgroundColor);
      if (bg[3] > 0) layers.push(bg);
      if (bg[3] === 1) break;
    }
    let bg = [255, 255, 255];
    for (const layer of layers.reverse()) bg = over(layer, bg);
    const fg = over(rgba(getComputedStyle(el).color), bg);

    const luminance = (c: number[]) => {
      const [r, g, b] = c.map((v) => {
        const s = v / 255;
        return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
      });
      return 0.2126 * r + 0.7152 * g + 0.0722 * b;
    };
    const [hi, lo] = [luminance(fg), luminance(bg)].sort((a, b) => b - a);
    return (hi + 0.05) / (lo + 0.05);
  });
}

test.describe("accessibility: colour contrast", () => {
  // aqaa: open bug, see report
  test.skip("subscribe button text meets the 4.5:1 contrast minimum", async ({ page }) => {
    await page.goto("/");

    const button = page.getByRole("button", { name: "Benachrichtigen lassen" });
    await expect(button).toBeVisible();
    await expect(button).toBeEnabled();

    expect(await contrastRatio(button)).toBeGreaterThanOrEqual(4.5);
  });
});
