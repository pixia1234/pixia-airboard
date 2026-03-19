# Pixia Airboard

一个用 Go 实现的订阅管理面板，数据库使用 SQLite，前后端均在本仓库内。

当前版本以 `Yohann0617/Xboard-airplane` 为参考，但已经主动收缩到最小闭环，只保留：

- 管理员登录
- 用户订阅管理
- 节点管理
- 节点对接与流量上报
- 流量统计与到期时间展示
- 多订阅链接与自定义订阅后缀
- Clash / Shadowrocket / 通用 V2 订阅格式适配

当前 API 分层：

- 面板主接口继续保持 Xboard 兼容：
  - 用户侧主用 `/api/v1/*`
  - 管理侧主用 `/api/v2/*`
- 节点主接口已收敛到 `/api/v1/agent/xrayr/*`
- 旧的 `/api/v1/server/UniProxy/*` 保留为兼容别名

核心兼容 API 目前覆盖：

- `GET /api/v1/guest/comm/config`
- `GET /api/v1/guest/plan/fetch`
- `POST /api/v1/passport/auth/register`
- `POST /api/v1/passport/auth/login`
- `GET /api/v1/passport/auth/token2Login`
- `GET /api/v1/user/info`
- `GET /api/v1/user/getSubscribe`
- `GET /api/v1/user/getStat`
- `GET /api/v1/user/checkLogin`
- `GET /api/v1/user/server/fetch`
- `GET /api/v1/user/plan/fetch`
- `GET /api/v1/user/notice/fetch`
- `GET /api/v1/client/subscribe`
- `GET /sub/{suffix}`
- `GET /api/v1/agent/xrayr/config`
- `GET /api/v1/agent/xrayr/users`
- `POST /api/v1/agent/xrayr/traffic`
- `POST /api/v1/agent/xrayr/alive`
- `GET /api/v1/server/UniProxy/config`（兼容别名）
- `GET /api/v1/server/UniProxy/user`（兼容别名）
- `POST /api/v1/server/UniProxy/push`（兼容别名）
- `POST /api/v1/server/UniProxy/alive`（兼容别名）
- `GET /api/v1/{secure_path}/config/fetch`
- `GET /api/v1/{secure_path}/plan/fetch`
- `GET /api/v1/{secure_path}/user/fetch`
- `GET /api/v1/{secure_path}/user/subscription/fetch`
- `POST /api/v1/{secure_path}/user/subscription/save`
- `POST /api/v1/{secure_path}/user/subscription/drop`
- `POST /api/v1/{secure_path}/user/subscription/reset`
- `GET /api/v1/{secure_path}/server/manage/getNodes`
- `GET /api/v1/{secure_path}/stat/getStat`

## 已移除模块

这版已经删除或不再暴露以下复杂模块：

- 支付
- 订单
- 工单
- 优惠券
- 知识库
- 路由管理
- 主题配置
- 复杂营销/佣金逻辑

## 技术栈

- Go 1.19
- Chi
- SQLite
- Redis（可选，用于热点缓存）
- 原生 HTML + CSS + JavaScript

## 已实现功能

- SQLite 自动建库和种子初始化
- 默认管理员和演示用户创建
- JWT 登录态与会话记录
- Redis 设置缓存
- Redis 会话和快捷登录缓存
- Redis 订阅内容短缓存
- Redis 节点用户列表短缓存
- 用户面板
- 管理员面板
- 套餐、用户、节点、公告基础管理
- 一个用户多订阅链接
- 管理员自定义订阅后缀
- 通用 V2 / Clash / Shadowrocket 订阅输出
- `/sub/{suffix}` 直接订阅入口
- UniProxy 风格节点对接与流量上报
- 仿 Xboard 风格的轻量前端

## 默认账号

- 管理员：`admin@example.com / admin123456`
- 演示用户：`demo@example.com / demo123456`

## 启动

```bash
go run ./cmd/airboard
```

默认监听：

```text
http://127.0.0.1:8080
```

前台地址：

```text
/
```

管理台地址：

```text
/admin
```

## 环境变量

- `AIRBOARD_ADDR`：监听地址，默认 `:8080`
- `AIRBOARD_DB_PATH`：SQLite 文件路径，默认 `data/airboard.db`
- `AIRBOARD_JWT_SECRET`：JWT 密钥
- `AIRBOARD_REDIS_ADDR`：Redis 地址，例如 `127.0.0.1:6379`
- `AIRBOARD_REDIS_PASSWORD`：Redis 密码
- `AIRBOARD_REDIS_DB`：Redis DB，默认 `0`
- `AIRBOARD_REDIS_PREFIX`：Redis key 前缀，默认 `airboard`
- `AIRBOARD_APP_URL`：站点公开地址
- `AIRBOARD_APP_NAME`：站点名称
- `AIRBOARD_ADMIN_PATH`：管理路径，默认 `admin`
- `AIRBOARD_ADMIN_EMAIL`：默认管理员邮箱
- `AIRBOARD_ADMIN_PASSWORD`：默认管理员密码

## 节点对接

系统当前提供一组 XrayR 主接口：

- `GET /api/v1/agent/xrayr/config?token=...&node_id=...`
- `GET /api/v1/agent/xrayr/users?token=...&node_id=...`
- `POST /api/v1/agent/xrayr/traffic`
- `POST /api/v1/agent/xrayr/alive`

同时保留 UniProxy 风格旧路径作为兼容别名：

- `GET /api/v1/server/UniProxy/config?token=...&node_id=...`
- `GET /api/v1/server/UniProxy/user?token=...&node_id=...`
- `POST /api/v1/server/UniProxy/push`
- `POST /api/v1/server/UniProxy/alive`

节点对接令牌可在管理员面板的站点设置中查看和修改，对应配置项 `server_token`。

## Redis 加速点

配置 Redis 后，以下热点路径会优先走缓存：

- 站点设置读取
- 会话校验
- 快捷登录 token
- 订阅正文渲染结果
- 节点拉取用户列表

如果没有配置 `AIRBOARD_REDIS_ADDR`，系统会继续只使用 SQLite，不会影响启动。

## 订阅格式

同一个订阅后缀支持多种客户端输出：

- 默认：通用 base64 订阅
- `?target=v2`：通用 V2 / sing-box / Neko 系列
- `?target=clash`：Clash YAML
- `?target=shadowrocket`：Shadowrocket 纯文本订阅

示例：

```text
/sub/my-user-main
/sub/my-user-main?target=clash
/sub/my-user-main?target=shadowrocket
```

## 本地验证

已验证以下链路可用：

- 游客配置读取
- 管理员登录
- 用户信息接口
- 多订阅后缀创建与查询
- `/sub/{suffix}` 公共订阅
- Clash 输出
- Shadowrocket 输出
- `subscription-userinfo` 头
- XrayR 主接口配置拉取
- XrayR 主接口用户拉取
- XrayR 主接口流量上报
- UniProxy 兼容别名与弃用头
- Redis 写入 `settings`、`session`、`quick_login`、`subscription`、`node_users` 热点键
- 节点列表接口
- 订阅地址生成
- 订阅内容下发
- 管理端统计接口

## 当前边界

这版不是 Xboard 的全量功能复刻，当前仍然是“最基础可用版”，主要边界有：

- 节点对接目前主实现为 XrayR 兼容接口，UniProxy 路径仅作为兼容层保留
- 复杂支付和订单流已移除
- 不包含工单、优惠券、知识库、路由管理
- Clash 输出以基础代理组为主，未做完整规则集管理
- 还没有更细的审计、邀请佣金、批量运营工具

如果你后续要继续往生产化推进，优先建议下一步补：

1. 更完整的节点协议参数与多后端适配
2. 更细的用户编辑和批量管理能力
3. 更完整的 Clash Meta / Sing-box 配置模板
4. 流量日志与节点在线统计面板
