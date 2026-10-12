# API 参考：系统与平台管理

这一组是部署级接口：读系统信息，以及系统管理员专属的平台控制面（全局设置、运行时队列、平台 API Key、跨空间审计、重置密码）。功能说明见[平台管理与系统管理员](../03-features/20-platform-admin.md)。

路由注册：`internal/router/routes_auth_tenant.go` 的 `RegisterSystemAdminRoutes` 与 `RegisterSystemRoutes`。Handler：`internal/handler/system.go`、`internal/handler/audit_log.go`。

`/system/admin/*` 全组挂 `SystemAdmin()` 守卫；平台 API Key 按能力细分（`system_settings_read/manage`、`system_runtime_read/manage`、`system_tenants_read/manage`、`system_audit_read`）。

## 系统信息（/api/v1/system）

Handler: `internal/handler/system.go`。API key：`manage_vector_stores`/full。本组响应使用 `{"code":0,"msg":"success","data":...}` 包装。

### GET /api/v1/system/info

用途：系统版本与引擎信息。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{version,edition,commit_id,build_time,go_version,keyword_index_engine,vector_store_engine,graph_database_engine,minio_enabled,db_version,started_at,uptime_seconds}}`

```bash
curl $BASE/api/v1/system/info -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/parser-engines

用途：解析引擎列表与 DocReader 连接状态。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":[...],"docreader_addr","docreader_transport","connected"}`

```bash
curl $BASE/api/v1/system/parser-engines -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/parser-engines/check

用途：用给定配置探测解析引擎（`types.ParserEngineConfig` 请求体）。权限：Admin+。

响应：200，同上。

```bash
curl -X POST $BASE/api/v1/system/parser-engines/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}'
```

### POST /api/v1/system/docreader/reconnect

用途：重连 DocReader。权限：Admin+。请求体：`{"addr":"host:port"}`（`binding:"required"`）。

响应：200 `{"code":0,"msg":"连接成功",...,"connected":true}`

```bash
curl -X POST $BASE/api/v1/system/docreader/reconnect -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"addr":"docreader:50051"}'
```

### GET /api/v1/system/storage-engine-status

用途：对象存储引擎可用性。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{"engines":[{name,allowed,available,description}],"allowed_providers":[...],"minio_env_available":bool}}`

```bash
curl $BASE/api/v1/system/storage-engine-status -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/storage-engine-check

用途：校验存储配置（SSRF 防护后探测）。权限：Admin+。请求体：`provider`（必填，`minio/cos/tos/s3/oss/ks3/obs`）+ 对应 `minio|cos|tos|s3|oss|ks3|obs` 配置对象。

响应：200 `{"code":0,"data":{"ok","message","bucket_created"}}`

```bash
curl -X POST $BASE/api/v1/system/storage-engine-check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"provider":"minio","minio":{"endpoint":"minio:9000"}}'
```

## 系统管理（/api/v1/system/admin，SystemAdmin 专属）

组级挂载 `SystemAdmin()` 守卫（始终强制，不受 EnableRBAC 影响）；平台 API key 需对应 `system_*` capability。本组读取接口多返回原始行/数组（无包装）。Handler: `internal/handler/system.go`、`internal/handler/audit_log.go`。

### POST /api/v1/system/admin/promote

用途：授予 SystemAdmin。请求体：`user_id`（UUID，优先）或 `email`（二选一）。

响应：200 `UserInfo`（原始对象）。

```bash
curl -X POST $BASE/api/v1/system/admin/promote -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"admin@ex.com"}'
```

### POST /api/v1/system/admin/revoke

用途：撤销 SystemAdmin。请求体：`{"user_id":"..."}`（`binding:"required"`）。

响应：200 `UserInfo`

```bash
curl -X POST $BASE/api/v1/system/admin/revoke -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"user_id":"u-1"}'
```

### GET /api/v1/system/admin/list

用途：SystemAdmin 列表。查询参数：`offset`（默认 0）、`limit`（默认 50，上限 200）。

响应：200 `{"total":N,"admins":[UserInfo]}`

```bash
curl $BASE/api/v1/system/admin/list -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/users/reset-password

用途：重置用户密码。请求体：`email`（`binding:"required,email"`）、`new_password`（`binding:"required"`）。

响应：200 `{"message":"Password reset successfully"}`

```bash
curl -X POST $BASE/api/v1/system/admin/users/reset-password -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"a@ex.com","new_password":"newpass1"}'
```

### GET /api/v1/system/admin/users

> v2.19 账号管理。以下九条 users 系列端点同属平台级账号治理，全部 SystemAdmin 专属，且**不对 API Key 开放**（default-deny）。所有写操作在目标为调用者本人时返回 400（自我保护，见[平台管理与系统管理员](../03-features/20-platform-admin.md)）。

用途：平台账号目录，分页 + 关键字搜索（用户名/邮箱，不区分大小写）+ 三态过滤。查询参数：`search`、`page`（默认 1）、`page_size`（默认 20，上限 100）、`is_active`、`is_system_admin`。

响应：200 `{"users":[UserInfo],"total":N,"page":1,"page_size":20}`

```bash
curl "$BASE/api/v1/system/admin/users?search=ali&page=1&page_size=20" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/users/:user_id

用途：账号详情——资料 + 全部跨空间成员关系（含空间名、角色、生命周期状态）。

响应：200 `{"user":UserInfo,"memberships":[{tenant_id,tenant_name,role,status,joined_at}]}`

```bash
curl $BASE/api/v1/system/admin/users/<user_id> -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/system/admin/users/:user_id

用途：编辑账号资料（PATCH 语义，仅提交字段变更；用户名 2-50 字符；用户名/邮箱唯一性冲突返回 409）。请求体：`username?`、`email?`。

响应：200 `UserInfo`

```bash
curl -X PUT $BASE/api/v1/system/admin/users/<user_id> -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"new@ex.com"}'
```

### PUT /api/v1/system/admin/users/:user_id/status

用途：启用/禁用账号。禁用 = `is_active=false` 并撤销该账号全部令牌（下一请求 401 即刻下线；登录路径本身已拒绝禁用账号）。幂等。请求体：`is_active`（必填布尔）。

响应：200 `UserInfo`

```bash
curl -X PUT $BASE/api/v1/system/admin/users/<user_id>/status -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"is_active":false}'
```

### POST /api/v1/system/admin/users/:user_id/force-logout

用途：强制下线该账号的全部活跃会话（不改变启用状态）。

响应：200 `{"message":"All sessions for the account have been revoked"}`

```bash
curl -X POST $BASE/api/v1/system/admin/users/<user_id>/force-logout -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/users/:user_id/sessions

用途：查看该账号的全部活跃登录会话（设备描述/IP/建立时间/最近活跃；与个人「登录设备」面板同构，无"本机"标记）。

响应：200 `[{session_id,device_label,device_ip,created_at,expires_at,last_refresh_at,current}]`

```bash
curl $BASE/api/v1/system/admin/users/<user_id>/sessions -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/system/admin/users/:user_id/sessions/:session_id

用途：踢除该账号的一个登录会话（撤销该 session_id 的全部令牌）。

响应：200 `{"message":"Session revoked"}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/users/<user_id>/sessions/<session_id> -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/system/admin/users/:user_id/memberships/:tenant_id

用途：调整该账号在某空间内的角色。请求体：`role`（`owner`/`admin`/`contributor`/`viewer`）。空间至少保留一位活跃 Owner（`ErrLastOwner` → 400），成员变更同时落 `rbac.member_role_changed`（空间侧）与 `system.membership_role_changed`（平台侧）审计。

响应：200 `{"message":"Member role updated","old_role":"contributor","new_role":"admin"}`

```bash
curl -X PUT $BASE/api/v1/system/admin/users/<user_id>/memberships/<tenant_id> -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}'
```

### DELETE /api/v1/system/admin/users/:user_id/memberships/:tenant_id

用途：将该账号移出指定空间（沿用成员服务语义：最后 Owner 保护、移出即撤销其全部令牌）。

响应：200 `{"message":"Member removed from workspace"}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/users/<user_id>/memberships/<tenant_id> -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/api-keys

用途：平台 API key 列表（掩码）。

响应：200 `{"success":true,"data":[{id,name,api_key,capabilities,expires_at_unix,...}]}`

```bash
curl $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/api-keys

用途：创建平台 API key（明文仅返回一次）。请求体：`name`（非空）、`capabilities`（`system_*` 列表，必填）、`expires_at_unix`（可选，须为未来时间）。

响应：201 `{"success":true,"data":{...,"api_key":"<明文>","token":"<明文>"}}`

```bash
curl -X POST $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"ops","capabilities":["system_tenants_read"]}'
```

### DELETE /api/v1/system/admin/api-keys/:key_id

用途：删除平台 API key。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/api-keys/3 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/settings 与 GET /api/v1/system/admin/settings/:key

用途：平台运行时设置列表 / 单项（平台 key 需 `system_settings_read|manage`）。

响应：200 `[SystemSetting]` / `SystemSetting`（原始，无包装；字段：`key,value,value_type,description,last_modified_by,last_modified_at`）。

```bash
curl $BASE/api/v1/system/admin/settings -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/system/admin/settings/:key

用途：更新设置（平台 key 需 `system_settings_manage`）。请求体：`{"value":<任意 JSON，按注册表类型校验>}`（必填）。

响应：200 `SystemSetting`

```bash
curl -X PUT $BASE/api/v1/system/admin/settings/default_storage_quota -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"value":10737418240}'
```

### DELETE /api/v1/system/admin/settings/:key

用途：恢复设置默认值。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/settings/default_storage_quota -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues

用途：asynq 队列深度与并发状态（Lite 模式返回 `available:false`；平台 key 需 `system_runtime_read|manage`）。

响应：200 `{"available",upstream_concurrency,parse_concurrency,wiki_concurrency,pools,queues,model_limiter_available,models,timestamp}`

```bash
curl $BASE/api/v1/system/admin/runtime/queues -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues/:queue/tasks

用途：队列任务列表。查询参数：`state`（`pending/active/scheduled/retry/archived/completed`）、`cursor`、`page_size`（默认 20，上限 100）。

响应：200 `{"available","tasks":[RuntimeTaskInfo],"page_size","has_more","next_cursor"}`

```bash
curl "$BASE/api/v1/system/admin/runtime/queues/default/tasks?state=pending" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/runtime/queues/:queue/tasks/:task_id/actions/:action

用途：任务操作（`action` ∈ `cancel/run_now/delete`；平台 key 需 `system_runtime_manage`）。

响应：200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/system/admin/runtime/queues/default/tasks/t-1/actions/cancel \
  -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/system/admin/runtime/queues/:queue/archived

用途：清空归档任务。

响应：200 `{"success":true,"deleted":N}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/runtime/queues/default/archived -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/tenants/apply-default-storage-quota

用途：把当前默认存储配额批量写到全部空间（平台 key 需 `system_tenants_manage`）。无请求体。

响应：200 `{"affected":N,"quota_bytes":N,"quota_gb":N}`

```bash
curl -X POST $BASE/api/v1/system/admin/tenants/apply-default-storage-quota -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/audit-log

用途：平台级审计日志（tenant_id=0 行；平台 key 需 `system_audit_read`）。查询参数同空间审计（`after_id/limit/action/outcome/actor`）。Handler: `internal/handler/audit_log.go`

响应：200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl $BASE/api/v1/system/admin/audit-log -H "Authorization: Bearer $TOKEN"
```
