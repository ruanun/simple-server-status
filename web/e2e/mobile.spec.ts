import { expect, test, type Page } from '@playwright/test'

test.use({ viewport: { width: 390, height: 844 } })

async function horizontalOverflow(page: Page): Promise<number> {
  return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
}

test('390px 宽度下首页与后台无横向滚动', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('link', { name: /e2e-node/ })).toBeVisible()
  expect(await horizontalOverflow(page)).toBeLessThanOrEqual(0)

  await page.goto('/login')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/admin\/servers/)
  await expect(page.getByText('e2e-node')).toBeVisible()
  expect(await horizontalOverflow(page)).toBeLessThanOrEqual(0)
})
