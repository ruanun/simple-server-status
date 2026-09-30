import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** cn 合并 className，后者覆盖前者的 Tailwind 冲突类 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** NO_AUTOFILL 非登录凭据的输入框：阻止浏览器及 1Password、LastPass、Bitwarden 把已保存的后台账号密码填入 */
export const NO_AUTOFILL = { 'data-1p-ignore': '', 'data-lpignore': 'true', 'data-bwignore': '' }
