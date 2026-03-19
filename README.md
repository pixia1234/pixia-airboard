# Pixia Airboard

一个基于 Go、SQLite 和 React 的轻量订阅管理面板，参考 Xboard 结构，但只保留最基础可用闭环。

## 功能范围

- 用户登录、订阅信息、节点列表、套餐查看、提醒设置
- 管理员登录、用户/套餐/节点/公告/站点配置管理
- 多订阅后缀和 `/sub/{suffix}` 公共订阅入口
- 默认订阅、Clash、V2、Shadowrocket 输出
- XrayR 风格节点接口
- 旧的 UniProxy 路径兼容别名

当前不包含：

- 支付
- 订单
- 工单
- 优惠券
- 知识库
- 路由管理
- 复杂营销/佣金逻辑

## 默认账号

- 管理员：`admin@example.com / admin123456`
- 演示用户：`demo@example.com / demo123456`

## 本地启动

直接运行：

```bash
go run ./cmd/airboard
```

默认地址：

```text
http://127.0.0.1:8080
```

- 前台：`/`
- 管理台：`/admin`

如果改了前端源码，需要重新构建静态资源：

```bash
cd frontend
npm ci
npm run build
```

## Docker

构建镜像：

```bash
docker build -t pixia-airboard:local .
```

单容器运行：

```bash
docker run --rm \
  -p 8080:8080 \
  -v airboard-data:/app/data \
  -e AIRBOARD_APP_URL=http://127.0.0.1:8080 \
  -e AIRBOARD_JWT_SECRET=change-me \
  pixia-airboard:local
```

带 Redis 启动整套服务：

```bash
docker compose up -d --build
```

`compose.yaml` 默认会启动：

- `app`
- `redis`
- SQLite 数据卷 `airboard-data`
- Redis 数据卷 `redis-data`

生产部署前至少修改：

- `AIRBOARD_JWT_SECRET`
- `AIRBOARD_APP_URL`
- `AIRBOARD_ADMIN_EMAIL`
- `AIRBOARD_ADMIN_PASSWORD`

## 说明

- Redis 是可选的，不配置时系统仍可正常启动
- 管理端接口必须走当前 `secure_path`
- 这不是 Xboard 全量复刻，而是基础可用版
