import { expect, test } from '@playwright/test'

test('登录后新建服务器并显示安装命令', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/admin\/servers/)
  await expect(page.getByText('e2e-node')).toBeVisible()

  await page.getByRole('button', { name: '新建服务器' }).click()
  await page.getByLabel('名称', { exact: true }).fill('e2e-new')
  await page.getByRole('button', { name: '保存' }).click()

  const dialog = page.getByRole('dialog', { name: '安装命令' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText(/--dashboard/)).toBeVisible()
})

test('设置公告后首页可见', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/admin\/servers/)

  await page.getByRole('link', { name: '设置' }).click()
  await expect(page).toHaveURL(/\/admin\/settings/)
  await page.getByLabel('公告', { exact: true }).fill('e2e 维护公告 https://example.com/e2e')
  await page.getByRole('button', { name: '保存' }).first().click()
  await expect(page.getByText('设置已保存')).toBeVisible()

  await page.goto('/')
  await expect(page.getByText('e2e 维护公告')).toBeVisible()
  await expect(page.getByRole('link', { name: 'https://example.com/e2e' })).toBeVisible()
})
