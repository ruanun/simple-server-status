import { expect, test } from '@playwright/test'

test('首页显示在线服务器，可进入详情页并切换时间范围', async ({ page }) => {
  await page.goto('/')
  const card = page.getByRole('link', { name: /e2e-node/ })
  await expect(card).toBeVisible()
  await expect(card.getByText(/^在线/)).toBeVisible()
  await card.click()
  await expect(page).toHaveURL(/\/server\//)
  await expect(page.getByRole('heading', { name: 'e2e-node' })).toBeVisible()
  await expect(page.getByText('历史趋势')).toBeVisible()
  await page.getByRole('tab', { name: '1 小时' }).click()
  await expect(page.getByRole('tab', { name: '1 小时' })).toHaveAttribute('data-state', 'active')
})
