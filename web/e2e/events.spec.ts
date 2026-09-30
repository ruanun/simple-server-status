import { expect, test } from '@playwright/test'

test('停止的 Agent 在事件页出现进行中的离线记录', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('用户名').fill('admin')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await page.waitForURL('**/admin/**')
  await page.goto('/admin/events')
  await expect(async () => {
    await page.reload()
    await expect(page.getByRole('row', { name: /e2e-offline/ })).toContainText('进行中')
  }).toPass({ timeout: 30_000 })
})
