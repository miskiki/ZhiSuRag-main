import { get, del } from '@/utils/request'

// v2.15/v2.16 多端会话管理：本人登录会话面板的数据接口。
// 会话是个人资产（本人操作本人会话），后端 g.Viewer() 门控。

export interface LoginSession {
  /** 会话标识（空 = 迁移前的早期会话桶） */
  session_id: string
  /** 设备友好描述（登录 UA 解析，如 "Chrome 129 · Windows"） */
  device_label: string
  /** 会话建立时的客户端 IP */
  device_ip: string
  /** 会话建立时间（该次登录首枚令牌） */
  created_at: string
  /** 刷新令牌过期时间（会话寿命上界） */
  expires_at: string
  /** 会话内最新令牌的更新时间（最近活跃的近似值） */
  last_refresh_at: string
  /** 是否为当前请求所属会话（面板高亮"本机"） */
  current: boolean
}

/** 列出我的全部活跃登录会话 */
export function listLoginSessions() {
  return get<{ code: number; msg: string; data: LoginSession[] }>('/api/v1/auth/sessions')
}

/** 获取当前请求所属会话的 session_id（面板高亮"本机"） */
export function getCurrentLoginSession() {
  return get<{ code: number; msg: string; data: { session_id: string } }>('/api/v1/auth/sessions/current')
}

/** 踢除指定会话（不允许踢除当前会话——后端自踢保护 + 前端本机行不提供按钮） */
export function revokeLoginSession(sessionId: string) {
  return del<{ code: number; msg: string; data: { revoked: boolean } }>(
    `/api/v1/auth/sessions/${encodeURIComponent(sessionId)}`,
  )
}
