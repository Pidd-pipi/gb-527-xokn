# 卫星地面站窗口冲突消解

`satellite-contact-window-deconfliction` 是供卫星运营规划人员使用的离线接触窗口分析台。它维护地面站、卫星资产与候选窗口，用确定性的 sweep-line 算法发现资源冲突、生成可解释建议，并要求 reviewer 人工复核。系统不连接天线、射频设备或任务控制系统，也不会产生遥控指令。

## Docker 快速启动

```bash
cp .env.example .env
# 将 .env 中 JWT_SECRET 和数据库口令改为部署环境的独立值
docker compose up -d --build
docker compose ps
```

服务健康后访问：

- 前端：`http://localhost:18527`
- 后端健康：`http://localhost:19527/healthz`
- 后端就绪：`http://localhost:19527/readyz`
- PostgreSQL：`localhost:57527`

测试账号：

| 角色 | 用户名 | 密码 | 权限 |
| --- | --- | --- | --- |
| scheduler | `scheduler` | `Scheduler#527` | 维护资源和窗口、检测冲突、提交复核 |
| reviewer | `reviewer` | `Reviewer#527` | 查看证据、接受或拒绝建议、查看审计、导出规划记录 |
| admin | `admin` | `Admin#527` | scheduler 与 reviewer 的并集 |

停止并删除本项目数据卷：

```bash
docker compose down -v --remove-orphans
```

## 功能

- `/stations`：地面站容量、天线数、频段、转向缓冲和窗口占用。
- `/satellites`：规划资产、优先权、最短接触需求和按卫星分组的时间线。
- `/windows`：UTC 候选窗口筛选、创建、兼容性提交和不可移动锁定。
- `/conflicts`：冲突扫描、证据、稳定排序建议、提交复核和人工接受/拒绝。
- `/audit`：request ID、参数摘要、版本前后差异、算法权重和人工选择。

接受建议只会在一个数据库事务内核对所有关联窗口版本并保存 reviewer 的选择，不会自动移动窗口。只有 accepted 记录可以通过导出 API 形成离线规划记录。

## 技术栈与目录

- 前端：Angular 17 standalone components、Angular Material、RxJS、Angular CLI/esbuild。
- 后端：Go 1.22、Gin、GORM、validator/v10、JWT、结构化 `slog`。
- 数据：Compose 使用 PostgreSQL 16；runtime smoke 和单元测试使用 GORM SQLite。

```text
.
├── backend/
│   ├── cmd/server/                 # 进程入口与优雅停机
│   └── internal/
│       ├── config/                 # 环境、PostgreSQL/SQLite、迁移和种子
│       ├── model,dto,repository/   # 四个核心实体逐层分文件
│       ├── scheduler/              # 区间、分组、候选、评分
│       ├── service,handler,router/ # 构造器注入的业务与 HTTP 层
│       └── middleware/             # 追踪、日志、认证、RBAC、恢复、限流
├── frontend/src/app/
│   ├── api,stores,types,hooks,router,utils/
│   ├── components/common/
│   └── pages/
├── docker-compose.yml
├── go.work
└── runtime_smoke.json
```

## 环境变量

| 变量 | 默认值/示例 | 说明 |
| --- | --- | --- |
| `FRONTEND_PORT` / `BACKEND_PORT` / `POSTGRES_PORT` | `18527` / `19527` / `57527` | 宿主端口 |
| `DB_DRIVER` | `postgres` | Compose 必须使用 PostgreSQL；smoke 使用 `sqlite` |
| `DB_DSN` | PostgreSQL DSN | GORM 连接字符串 |
| `DB_AUTO_MIGRATE` | `true` | 启动时执行模型迁移 |
| `JWT_SECRET` | 至少 32 字节 | HS256 签名密钥 |
| `JWT_TTL_MINUTES` | `480` | token 有效期 |
| `CORS_ORIGIN` | `http://localhost:18527` | 允许的前端源 |
| `RATE_LIMIT_PER_MINUTE` | `180` | 单 IP 本地分钟限流 |
| `WEIGHT_PRIORITY_LOSS` | `4.0` | 优先级损失惩罚 |
| `WEIGHT_MOVEMENT_DISTANCE` | `0.02` | 站点移动距离惩罚 |
| `WEIGHT_CONTACT_DURATION` | `0.003` | 接触时长收益 |
| `WEIGHT_RESOURCE_MARGIN` | `2.0` | 资源余量收益 |

## API

所有业务 API 使用 `/api/v1`；成功响应为 `data + request_id`，列表另含 `meta`，错误响应为统一 `error.code + error.message + request_id`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/auth/login` | 登录 |
| GET/POST/PUT | `/api/v1/stations[/:id]` | 地面站列表、新建、版本更新 |
| GET/POST/PUT | `/api/v1/satellites[/:id]` | 卫星资产列表、新建、版本更新 |
| GET/POST/PUT | `/api/v1/windows[/:id]` | 窗口列表、新建、版本更新 |
| POST | `/api/v1/windows/:id/submit` | 严格校验资源状态、频段和最短时长 |
| POST | `/api/v1/windows/:id/lock` | 锁定规划输入；后续移动返回 409 |
| POST | `/api/v1/conflicts/detect` | 在 UTC 范围运行确定性冲突检测 |
| GET | `/api/v1/conflicts[/:id]` | 建议、评分和证据 |
| POST | `/api/v1/conflicts/:id/submit` | `proposed -> pending_review` |
| POST | `/api/v1/conflicts/:id/review` | reviewer 接受或拒绝 |
| GET | `/api/v1/conflicts/:id/export` | accepted 规划记录，不含控制命令 |
| GET | `/api/v1/audit` | 审计分页列表 |

`/healthz` 表示进程存活，`/readyz` 真实 ping 数据库。

## 共享枚举位置

### WindowStatus

值为 `candidate | submitted | locked | allocated | cancelled`。

- 数据库：`contact_windows.window_status`。
- 后端常量与状态迁移：`backend/internal/constants/window.go`。
- 后端 model/DTO/repository/service/handler/router：分别位于 `model/contact_window.go`、`dto/contact_window.go`、`repository/contact_window.go`、`service/contact_window.go`、`handler/contact_window.go`、`router/contact_window.go`。
- 前端类型/store：`frontend/src/app/types/window.ts`、`frontend/src/app/stores/planning.store.ts`。
- 前端共享组件：`WindowStatusBadge`、`ResourceTimeline`。
- 前端页面：`windows.page.ts`、`stations.page.ts`、`satellites.page.ts`，冲突和审计页也用同一 badge 显示工作流状态。

### ConflictType

值为 `station_capacity | satellite_overlap | band_mismatch | duration_shortfall | slew_buffer`。

- 数据库：`conflict_resolutions.conflict_type`。
- 后端常量：`backend/internal/constants/conflict.go`。
- 后端算法：`scheduler/interval.go`、`grouping.go`、`candidates.go`、`scoring.go`。
- 后端 model/DTO/repository/service/handler/router：分别位于同名 `conflict_resolution.go` 文件。
- 前端类型/hook/store：`types/conflict.ts`、`hooks/use-conflict-detection.ts`，冲突列表由 API 状态持有，不复制枚举。
- 前端共享组件：`ResolutionComparePanel`、`WindowStatusBadge`。
- 前端页面：`conflicts.page.ts` 与 `audit.page.ts`。

## 算法假设

1. 区间采用半开区间 `[start_at, end_at)`，同一时刻结束和开始不算原始重叠。
2. 地面站容量按开始时间稳定排序的 sweep-line 检测；再把窗口前后扩展 `slew_buffer_sec` 检测转向缓冲冲突。
3. 同一卫星并发窗口按容量 1 检测；频段分别和站点、卫星支持列表比较；实际时长和卫星最短接触要求比较。
4. conflict key 包含类型、排序后的窗口 ID 和输入版本；相同输入产生相同 key、分组和建议顺序。
5. 建议包含保留高优先级、同源版本备选窗口、最近兼容站点和人工处理。评分显式记录优先级损失、站点距离、接触时长和资源余量；总分相同再按 action key 排序。
6. 锁定窗口从不进入自动移动列表。算法只生成建议，不调用外部设备，不写回 ContactWindow。

## 安全与边界

- JWT 只从 `Authorization: Bearer` 读取，RBAC 不接受客户端角色头。
- RequestID、AccessLog、Auth、RBAC、Recovery、RateLimit 是独立中间件函数。
- 访问日志只记录路由模板、状态、耗时和 request ID，不记录 JWT、请求体或复核备注全文。
- 审计记录资源参数、权重、版本前后摘要和复核备注长度；人工选择本身作为版本化结果保留。
- 409 不会被前端吞掉，会在当前页面显示版本、锁定或非法状态错误。
- 本项目不是预约平台、通用日历、工单、库存、订单或计费系统；导出内容仅供规划归档。

## 本地开发与验证

```bash
go work sync
go build ./backend/...
go vet ./backend/...
go test ./backend/...
npm --prefix frontend ci
npm --prefix frontend run typecheck
npm --prefix frontend run build


```

前端本地开发使用 `npm --prefix frontend start`；若不通过 Nginx，需要保持 API 同源代理或将请求指向后端。生产验收以 Compose Nginx 反代为准。

## 排错

- backend 不 healthy：查看 `docker compose logs backend`，检查 JWT 至少 32 字节、DSN 和 PostgreSQL 健康状态。
- 前端 `/api` 返回 502：确认 backend healthy；Nginx 保留 `/api/v1` 原路径，不应给 `proxy_pass` 添加尾斜杠。
- 接受方案返回 `version_conflict`：冲突检测后有窗口版本变化，重新检测并再次提交复核。
- 提交窗口返回 `band_incompatible` 或 `duration_shortfall`：修正候选窗口；候选仍可保留用于冲突证据分析。

## License

MIT License。卫星、站点和轨道数据均为演示用离线规划数据。
