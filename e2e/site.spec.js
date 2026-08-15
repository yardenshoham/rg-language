import { expect, test } from "@playwright/test";

// The reference set the project is defined by. This is the only place it is spelled
// out; the 5,012-item corpus pins the same transform for every one of its items.
const REFERENCE = [
  ["היי", "הרגיי"],
  ["שלום", "שרגלורגום"],
  ["מה נשמע", "מרגה נרגשמרגע"],
  ["היום יום שלישי", "הרגיורגום יורגום שלירגישירגי"],
  ["גנן", "גרגנרגן"],
  ["ערוגה", "ערגרורגוגרגה"],
  ["נחמד", "נרגחמרגד"],
  ["אני ממש אוהב פיצה", "ארגנירגי מרגמרגש אורגוהרגב פירגיצרגה"],
];

const plain = (page) => page.locator(".view.plain p");

test.describe("the page itself", () => {
  // Faux bold smears the niqqud, so the real 600 weight has to arrive.
  test("serves both weights of the Hebrew font from the binary", async ({ page }) => {
    const fonts = [];
    page.on("response", (r) => {
      if (r.url().includes("/static/fonts/")) fonts.push([r.url(), r.status()]);
    });

    await page.goto("/");
    await page.evaluate(() => document.fonts.ready);

    const loaded = await page.evaluate(() =>
      ["400", "600"].map((weight) =>
        document.fonts.check(`${weight} 16px "Noto Sans Hebrew"`),
      ),
    );
    expect(loaded, "both weights should be usable").toEqual([true, true]);
    for (const [url, status] of fonts) expect(status, url).toBe(200);
  });

  test("works with no JavaScript at all", async ({ browser }) => {
    const context = await browser.newContext({ javaScriptEnabled: false });
    const page = await context.newPage();
    await page.goto("/?text=" + encodeURIComponent("שלום"));

    await expect(plain(page)).toHaveText("שרגלורגום");
    await context.close();
  });

  test("explains the rule on the about page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "מה זה" }).click();

    await expect(page).toHaveURL(/\/about$/);
    await expect(page.getByRole("heading", { name: "הכלל" })).toBeVisible();
  });
});

test.describe("the transform", () => {
  for (const [input, expected] of REFERENCE) {
    test(`${input} becomes ${expected}`, async ({ page }) => {
      await page.goto("/?text=" + encodeURIComponent(input));
      await expect(plain(page)).toHaveText(expected);
    });
  }

  test("updates live as you type, without a navigation", async ({ page }) => {
    await page.goto("/");
    // A reload would wipe this, which is what tells a swap from a navigation.
    await page.evaluate(() => (window.sameDocument = true));

    await page.getByRole("textbox").fill("שלום");

    await expect(plain(page)).toHaveText("שרגלורגום");
    // The URL tracks the box so it stays shareable, and emptying the box must
    // not leave ?text= behind.
    await expect(page).toHaveURL("/?text=" + encodeURIComponent("שלום"));
    await page.getByRole("textbox").fill("");
    await expect(page).toHaveURL("/");
    await expect(page.locator(".view")).toHaveCount(0);
    await expect(page.locator("audio")).toHaveCount(0);
    expect(await page.evaluate(() => window.sameDocument), "htmx should not navigate").toBe(true);
  });

  test("shows all three renderings", async ({ page }) => {
    await page.goto("/?text=" + encodeURIComponent("מה נשמע"));

    await expect(plain(page)).toHaveText("מרגה נרגשמרגע");
    // The diacritizer writes a shin dot before the vowel, which is not the canonical
    // mark order even though it renders identically. Compare normalized so that an
    // assertion is about the text, not about the order marks happen to be stacked in.
    await expect
      .poll(async () => ((await page.locator(".view.vocalized p").textContent()) ?? "").normalize("NFC"))
      .toBe("מָרְגָה נִרְגִשְׁמַרְגַע".normalize("NFC"));
    await expect(page.locator(".view.latin p")).toHaveText("ma-rga ni-rgi-shma-rga");
  });

  // The highlight is the point: it shows what the rule added. Every inserted run
  // starts with רג; the o/u copies carry a vav too, so שלום ends ...רגו.
  test("highlights only the inserted רג", async ({ page }) => {
    await page.goto("/?text=" + encodeURIComponent("שלום"));

    const inserted = page.locator(".view.plain .inserted");
    await expect(inserted).toHaveCount(2);
    expect(await inserted.allTextContents()).toEqual(["רג", "רגו"]);
  });

  test("an example link fills the box and the result", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "מה נשמע" }).click();

    await expect(page.getByRole("textbox")).toHaveValue("מה נשמע");
    await expect(plain(page)).toHaveText("מרגה נרגשמרגע");
  });

  test("an empty box shows no result", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator(".view")).toHaveCount(0);
  });
});

test.describe("the audio", () => {
  // Nothing should be synthesized for a visitor who never presses play.
  test("is not fetched until play is pressed", async ({ page }) => {
    const fetched = [];
    page.on("request", (r) => {
      if (r.url().includes("/audio/")) fetched.push(r.url());
    });

    await page.goto("/?text=" + encodeURIComponent("שלום"));
    await expect(page.locator("audio")).toHaveAttribute("preload", "none");
    expect(fetched).toEqual([]);
  });

  test("plays a WAV that the browser can cache forever", async ({ page }) => {
    await page.goto("/?text=" + encodeURIComponent("שלום"));
    const src = await page.locator("audio").getAttribute("src");
    expect(src).toMatch(/^\/audio\/[0-9a-f]{32}\.wav$/);

    const response = await page.request.get(src);
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toBe("audio/wav");
    expect(response.headers()["cache-control"]).toContain("immutable");

    const body = await response.body();
    expect(body.subarray(0, 4).toString()).toBe("RIFF");
    expect(body.subarray(8, 12).toString()).toBe("WAVE");
    // 22.05 kHz, mono, 16-bit.
    expect(body.readUInt16LE(22)).toBe(1);
    expect(body.readUInt32LE(24)).toBe(22050);
    expect(body.readUInt16LE(34)).toBe(16);
  });

  // Headless Chrome has no audio device, so the clock does not necessarily
  // advance. Decoding is the part worth checking: it proves the WAV the server
  // built is one a browser can actually play.
  test("decodes in the browser", async ({ page }) => {
    await page.goto("/?text=" + encodeURIComponent("מה נשמע"));

    const media = await page.locator("audio").evaluate(async (audio) => {
      audio.load();
      await new Promise((resolve, reject) => {
        audio.addEventListener("loadedmetadata", resolve, { once: true });
        audio.addEventListener("error", () => reject(audio.error?.message), { once: true });
      });
      return { duration: audio.duration, readyState: audio.readyState };
    });
    expect(media.duration, "a four-word phrase should be over a second").toBeGreaterThan(1);
    expect(media.readyState).toBeGreaterThanOrEqual(1);
  });

  // Safari probes a media URL with a Range request before it will play anything,
  // and seeking needs them everywhere.
  test("answers range requests", async ({ page }) => {
    await page.goto("/?text=" + encodeURIComponent("שלום"));
    const src = await page.locator("audio").getAttribute("src");
    await page.request.get(src); // make sure it is synthesized

    const partial = await page.request.get(src, { headers: { Range: "bytes=0-1" } });
    expect(partial.status()).toBe(206);
    expect(partial.headers()["content-range"]).toMatch(/^bytes 0-1\/\d+$/);
    expect((await partial.body()).length).toBe(2);

    const whole = await page.request.get(src);
    expect(whole.headers()["accept-ranges"]).toBe("bytes");
  });

  test("404s on a hash the server never handed out", async ({ page }) => {
    const response = await page.request.get("/audio/" + "0".repeat(32) + ".wav");
    expect(response.status()).toBe(404);
  });
});

// Untrusted input is rendered as text, never as markup.
test("escapes HTML in the input", async ({ page }) => {
  await page.goto("/?text=" + encodeURIComponent('<img src=x onerror=alert(1)>שלום'));

  expect(await page.locator("img").count()).toBe(0);
  await expect(page.getByRole("textbox")).toHaveValue('<img src=x onerror=alert(1)>שלום');
});
