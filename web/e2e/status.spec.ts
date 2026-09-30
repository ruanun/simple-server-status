import { expect, test } from '@playwright/test'

test('首页显示在线服务器，可进入详情页并切换时间范围', async ({ page }) => {
  // 在线率窗口需要服务器创建后经过约 2–3 分钟（含 1 分钟降采样入库延迟）才有数据，等待期需要更长的用例超时
  test.setTimeout(240_000)
  await page.goto('/')
  const card = page.getByRole('link', { name: /e2e-node/ })
  await expect(card).toBeVisible()
  await expect(card.getByTitle(/^在线 /)).toBeVisible()
  await card.click()
  await expect(page).toHaveURL(/\/server\//)
  await expect(page.getByRole('heading', { name: 'e2e-node' })).toBeVisible()
  await expect(page.getByText('历史趋势')).toBeVisible()
  await expect(page.getByText(/每日流量/)).toBeVisible()
  await expect(async () => {
    await page.reload()
    await expect(page.getByText(/在线率/)).toBeVisible()
  }).toPass({ timeout: 210_000 })
  await page.getByRole('tab', { name: '1 小时' }).click()
  await expect(page.getByRole('tab', { name: '1 小时' })).toHaveAttribute('data-state', 'active')
})
