import { expect, test, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
async function enterStudent(
  page: Page,
  name: string,
  group: string,
  code?: string,
) {
  await page.goto("/login");
  await expect(page.locator('input[type="password"]')).toHaveCount(0);
  await page.getByRole("textbox", { name: "ФИО", exact: true }).fill(name);
  const field = page.locator('[name="group_name"]');
  await expect(field).toBeVisible();
  if (await field.evaluate((el) => el.tagName === "SELECT"))
    await field.selectOption(group);
  else await field.fill(group);
  if (code)
    await page
      .getByRole("textbox", { name: "Код приглашения", exact: true })
      .fill(code);
  await page.getByRole("button", { name: "Войти", exact: true }).click();
}
async function enterStaff(page: Page) {
  await page.goto("/staff/login");
  await page
    .getByRole("textbox", { name: "Логин", exact: true })
    .fill(process.env.E2E_ADMIN_LOGIN || "admin");
  await page
    .getByRole("textbox", { name: "Пароль", exact: true })
    .fill(process.env.E2E_ADMIN_PASSWORD!);
  await page.getByRole("button", { name: "Войти", exact: true }).click();
  await expect(
    page.getByRole("link", { name: "Администрирование", exact: true }),
  ).toBeVisible();
}
test("student: no passwords, editor, persistent identity and progress across login", async ({
  page,
}) => {
  const fullName = `Александр Учебный ${Date.now()}`;
  const group = `ТЕСТ-${Date.now() % 1000000}`;
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await enterStudent(page, fullName, group);
  await expect(
    page.getByRole("heading", { name: /Привет, АЛЕКСАНДР/ }),
  ).toBeVisible();
  const me = await (await page.request.get("/api/auth/me")).json();
  await page
    .getByRole("link", { name: "Начать практику", exact: true })
    .click();
  await expect(page.locator(".monaco-editor")).toBeVisible();
  await page.locator(".monaco-editor").click({ position: { x: 180, y: 80 } });
  await page.keyboard.press("Control+a");
  await page.keyboard.insertText('name = input()\nprint(f"Привет, {name}!")\n');
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/exercises/1/draft")).json()).code,
    )
    .toContain("print(f");
  await page.reload();
  await expect(
    page.getByText("Черновик сохранён", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Проверить", exact: true }).click();
  if (process.env.E2E_RUNNER_AVAILABLE === "true")
    await expect(
      page.getByText("Пройдено 3 из 3 тестов", { exact: true }),
    ).toBeVisible();
  else
    await expect(page.locator('.console-body [role="alert"]')).toContainText(
      /Сервис исполнения/,
    );
  await page
    .getByRole("link", { name: "История решений", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "История отправок", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Выйти", exact: true }).click();
  await expect(page).toHaveURL(/\/login/);
  await enterStudent(
    page,
    `  ${fullName.toLowerCase().replace(" ", "   ")}  `,
    group.toLowerCase(),
  );
  await expect(
    page.getByRole("heading", { name: /Привет, АЛЕКСАНДР/ }),
  ).toBeVisible();
  const again = await (await page.request.get("/api/auth/me")).json();
  expect(again.id).toBe(me.id);
  expect(
    (await (await page.request.get("/api/exercises/1/draft")).json()).code,
  ).toContain("print(f");
  await page.screenshot({
    path: "../artifacts/identity-dashboard.png",
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
test("teacher: semester date, group roster, namesakes, multiple devices and session clearing", async ({
  page,
  browser,
}) => {
  test.skip(!process.env.E2E_ADMIN_PASSWORD, "Set E2E_ADMIN_PASSWORD");
  await enterStaff(page);
  await page.getByRole("link", { name: "Группы и вход", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Группы и вход", exact: true }),
  ).toBeVisible();
  const group = `E${Date.now()}`;
  const name = "Иванов Иван Иванович";
  const firstCode = randomBytes(4).toString("hex").toUpperCase();
  const secondCode = randomBytes(4).toString("hex").toUpperCase();
  const target = new Date(Date.now() + 45 * 86400000)
    .toISOString()
    .slice(0, 10);
  await page
    .getByLabel("Дата окончания семестра", { exact: true })
    .fill(target);
  await page.locator('select[name="group_mode"]').selectOption("list");
  await page
    .getByRole("button", { name: "Сохранить настройки", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Настройки сохранены");
  await page
    .getByRole("button", { name: "Добавить группу", exact: true })
    .click();
  let dialog = page.getByRole("dialog", { name: "Настройка группы" });
  await dialog
    .getByRole("textbox", { name: "Название группы", exact: true })
    .fill(group);
  await dialog
    .getByRole("checkbox", {
      name: "Требовать индивидуальный код",
      exact: true,
    })
    .check();
  await dialog
    .getByRole("button", { name: "Сохранить группу", exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  const card = page
    .locator(".identity-card")
    .filter({ has: page.getByRole("heading", { name: group, exact: true }) });
  await card
    .getByRole("button", { name: "Список и коды", exact: true })
    .click();
  dialog = page.getByRole("dialog", { name: "Список приглашений" });
  await dialog
    .getByRole("textbox", { name: "ФИО и коды", exact: true })
    .fill(`${name};${firstCode}\n${name};${secondCode}`);
  await dialog
    .getByRole("button", { name: "Загрузить список", exact: true })
    .click();
  await expect(dialog.getByText(firstCode, { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Закрыть", exact: true }).click();
  const options = { baseURL: process.env.E2E_URL || "http://localhost:3000" };
  const one = await browser.newContext(options);
  const two = await browser.newContext(options);
  try {
    const first = await one.newPage();
    const second = await two.newPage();
    await enterStudent(first, name, group, "WRONG999");
    await expect(first.locator('.form-error[role="alert"]')).toContainText(
      "Не удалось войти",
    );
    await first
      .getByRole("textbox", { name: "Код приглашения", exact: true })
      .fill(firstCode);
    await first.getByRole("button", { name: "Войти", exact: true }).click();
    await expect(
      first.getByRole("heading", { name: /Привет, ИВАНОВ/ }),
    ).toBeVisible();
    const identityOne = await (await one.request.get("/api/auth/me")).json();
    expect(
      (
        await one.request.put("/api/exercises/1/draft", {
          data: { code: 'print("saved on device one")' },
        })
      ).status(),
    ).toBe(200);
    await enterStudent(second, name, group, secondCode);
    await expect(
      second.getByRole("heading", { name: /Привет, ИВАНОВ/ }),
    ).toBeVisible();
    const identityTwo = await (await two.request.get("/api/auth/me")).json();
    expect(identityTwo.id).not.toBe(identityOne.id);
    expect(
      (await (await two.request.get("/api/exercises/1/draft")).json()).exists,
    ).toBe(false);
    const cookieBefore = (await one.cookies()).find(
      (c) => c.name === "nexcode_session",
    )!;
    const extended = new Date(Date.now() + 60 * 86400000)
      .toISOString()
      .slice(0, 10);
    await page
      .getByLabel("Дата окончания семестра", { exact: true })
      .fill(extended);
    await page
      .getByRole("button", { name: "Сохранить настройки", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Настройки сохранены");
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/teacher/identity")).json())
            .semester_end,
      )
      .toBe(extended);
    expect((await one.request.get("/api/auth/me")).status()).toBe(200);
    const cookieAfter = (await one.cookies()).find(
      (c) => c.name === "nexcode_session",
    )!;
    expect(cookieAfter.expires).toBeGreaterThan(cookieBefore.expires);
    await first.getByRole("button", { name: "Выйти", exact: true }).click();
    expect((await two.request.get("/api/auth/me")).status()).toBe(200);
    await enterStudent(first, name, group, firstCode);
    await expect(
      first.getByRole("heading", { name: /Привет, ИВАНОВ/ }),
    ).toBeVisible();
    expect((await (await one.request.get("/api/auth/me")).json()).id).toBe(
      identityOne.id,
    );
    expect(
      (await (await one.request.get("/api/exercises/1/draft")).json()).code,
    ).toContain("saved on device one");
    await page
      .getByRole("button", { name: "Журнал входов", exact: true })
      .click();
    await page
      .getByRole("combobox", { name: "Группа в журнале", exact: true })
      .selectOption(group);
    await expect(page.getByText("Отказ", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Успех", { exact: true }).first(),
    ).toBeVisible();
    await page.screenshot({
      path: "../artifacts/login-journal.png",
      fullPage: true,
    });
    await page
      .getByRole("button", { name: "Настройки и группы", exact: true })
      .click();
    await card
      .getByRole("button", { name: `Завершить сессии ${group}`, exact: true })
      .click();
    await page
      .getByRole("dialog", { name: "Завершение сессий" })
      .getByRole("button", { name: "Завершить сессии", exact: true })
      .click();
    await expect(
      page.getByRole("dialog", { name: "Завершение сессий" }),
    ).toHaveCount(0);
    expect((await one.request.get("/api/auth/me")).status()).toBe(401);
    expect((await two.request.get("/api/auth/me")).status()).toBe(401);
    await enterStudent(first, name, group, firstCode);
    await expect(
      first.getByRole("heading", { name: /Привет, ИВАНОВ/ }),
    ).toBeVisible();
    expect(
      (await (await one.request.get("/api/exercises/1/draft")).json()).code,
    ).toContain("saved on device one");
    await page.screenshot({
      path: "../artifacts/identity-settings.png",
      fullPage: true,
    });
  } finally {
    await one.close();
    await two.close();
  }
});
test("staff catalog and administration still work", async ({ page }) => {
  test.skip(!process.env.E2E_ADMIN_PASSWORD, "Set E2E_ADMIN_PASSWORD");
  await enterStaff(page);
  await page
    .getByRole("link", { name: "Каталог заданий", exact: true })
    .click();
  const title = `Импорт ${Date.now()}`;
  await page
    .getByLabel("Импорт каталога")
    .setInputFiles({
      name: "catalog.json",
      mimeType: "application/json",
      buffer: Buffer.from(
        JSON.stringify({
          labs: [
            {
              title,
              description: "Тест",
              position: 8,
              exercises: [
                {
                  title: "Тест суммы",
                  statement: "Выведите сумму.",
                  template: "# code",
                  tests: '{"stdin":"2\\n3\\n","stdout":"5\\n"}',
                  timeout_ms: 5000,
                  memory_mb: 128,
                },
              ],
            },
          ],
        }),
      ),
    });
  await expect(
    page.getByRole("heading", { name: title, exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Студенты", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Прогресс студентов", exact: true }),
  ).toBeVisible();
  const csv = await page.request.get("/api/teacher/export");
  expect(csv.status()).toBe(200);
  expect(await csv.text()).toContain("student_id,full_name,group_name");
  await page
    .getByRole("link", { name: "Администрирование", exact: true })
    .click();
  await expect(
    page.getByRole("combobox", { name: "Роль admin", exact: true }),
  ).toBeDisabled();
  await page.setViewportSize({ width: 1024, height: 900 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
