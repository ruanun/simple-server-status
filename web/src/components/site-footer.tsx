const link = 'underline-offset-4 hover:text-foreground hover:underline'

/** SiteFooter 公开页页脚：程序名与作者，不显示版本号 */
export function SiteFooter() {
  return (
    <footer className="mt-auto border-t">
      <div className="mx-auto flex max-w-[1400px] flex-wrap items-center justify-center gap-x-2 gap-y-1 px-4 py-4 text-xs text-muted-foreground">
        <a href="https://github.com/ruanun/simple-server-status" target="_blank" rel="noopener noreferrer" className={link}>
          Simple Server Status
        </a>
        <span aria-hidden>|</span>
        <span>
          ©{new Date().getFullYear()} Created by{' '}
          <a href="https://github.com/ruanun" target="_blank" rel="noopener noreferrer" className={link}>
            Ruan
          </a>
        </span>
      </div>
    </footer>
  )
}
