import { $message, $notification } from '@/utils/discrete'

export function showApiError(e: any, fallback = '操作失败') {
  const msg: string = e?.response?.data?.msg ?? e?.message ?? fallback
  const needsDetail = /\n|sudo|权限|NOPASSWD|sudoers/i.test(msg)

  if (needsDetail) {
    $notification().error({
      title: '需要管理员权限',
      content: msg,
      duration: 15000,
      closable: true,
    })
  } else {
    $message().error(msg)
  }
}
