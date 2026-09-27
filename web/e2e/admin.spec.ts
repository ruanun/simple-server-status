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
