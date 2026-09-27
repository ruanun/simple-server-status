import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** cn 合并 className，后者覆盖前者的 Tailwind 冲突类 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
