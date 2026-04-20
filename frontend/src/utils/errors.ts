import { MessagePlugin, NotifyPlugin } from 'tdesign-vue-next'

/**
 * 统一错误提示：当后端返回的错误消息包含换行 / sudo / 权限 关键字时，
 * 自动改用 NotifyPlugin 展示（支持多行 + 可复制修复指引）；否则走 MessagePlugin.error。
 */
export function showApiError(e: any, fallback = '操作失败') {
  const msg: string = e?.response?.data?.msg ?? e?.message ?? fallback
  const needsDetail = /\n|sudo|权限|NOPASSWD|sudoers/i.test(msg)

  if (needsDetail) {
    NotifyPlugin.error({
      title: '需要管理员权限',
      content: msg,
      duration: 15000, // 15s 便于阅读 / 复制
      closeBtn: true,
    })
  } else {
    MessagePlugin.error(msg)
  }
}
