# steemgosdk 改造计划（为 conveyor Go 重构铺路）

> 本文档自包含，供接手 steemgosdk 的 agent 直接执行。
> 修订: 2026-07-13

## 背景

我正在用 Go + Gin + steemgosdk 重写 `github.com/steemit/conveyor`（Steem JSON-RPC 2.0 服务，原 TS 实现在 `/home/ety001/workspace/conveyor`）。conveyor 需要通过 steemgosdk 访问 Steem 链：查账户、账户历史、follow 关系、价格等，并验证 `__signed` 鉴权。

经审计，steemgosdk 目前：
- `API.Call`/`CallWithResult`（原始 RPC）✅ 可用
- `API.GetDynamicGlobalProperties`/`GetBlock`/`GetOpsInBlock` ✅ 有封装
- `API.SignedCall`/`Client.SignedCall`（**签名端**，客户端发请求）✅ 有封装
- `auth.Auth` 是 `steemutil/auth` 的薄封装（WIF/key/memo）✅

**缺口**：① 只有签名端无验证端；② 缺 conveyor 高频用的链查询便捷封装。

> 注：原 G3「价格算术」已上移到 steemutil（见下方依赖说明），steemgosdk 不再实现。

## 现状（v0.0.24 后）

steemutil v0.0.24 已发布，以下已就绪，steemgosdk 直接复用：
- `rpc.VerifySignedRpc`（验证端核心，对侧 U1）
- `rpc.AccountFetcher`（返回 `api.Authority`）
- `protocol/api.ExtendedAccount` / `Authority` / `KeyAuth`（含 `["STMxxx",1]` 自定义反序列化）
- `protocol/api.FollowCountReturn` / `FollowReturn`
- `protocol/api.OrderBook` / `Order` / `OrderPrice` / `FeedHistory` / `CurrentMedianHistoryPrice`

steemgosdk 当前 `go.mod` 仍 v0.0.23，待 steemutil v0.0.25（Asset/Price）发布后统一升级。

## 依赖与执行顺序

- **steemutil v0.0.25（Asset/Price）先行**：开发计划见 `/home/ety001/workspace/agent-share/notes/steemutil/asset-price-plan.md`。
- G1/G2 本身不强依赖 v0.0.25（G1 只需 v0.0.24 的 `VerifySignedRpc`，G2 只需 v0.0.24 的 wire 结构体），但为避免两次依赖升级，**统一在 v0.0.25 发布后开工**。

> **当前状态：等待 steemutil v0.0.25。** 本轮 steemgosdk 不做编码，待对侧发版后继续。

## 改造项（升级到 v0.0.25 后执行）

### 步骤 0：升级依赖

- `go.mod`：`github.com/steemit/steemutil v0.0.23` → `v0.0.25`，`go mod tidy`。
- `go build ./...` 验证 v0.0.25 新符号（`protocolapi.Asset`、`Price`、`ComputePrices` 等）可引用。

### G2. 链查询便捷封装（`api/api.go` 追加方法）

结果结构体**全部复用** v0.0.24/v0.0.25 的 `protocolapi.*`。仅新增 `AccountHistoryEntry`（U4 未覆盖，因其 wire 是元组 `[index, {op, timestamp}]`）。

每个方法内部走 `a.CallWithResult(apiName, method, params, &result)`。

```go
// condenser_api.get_accounts —— 参数是位置数组套数组 [["n1","n2"]]
func (a *API) GetAccounts(names []string) ([]*protocolapi.ExtendedAccount, error)

// condenser_api.get_follow_count —— [account]
func (a *API) GetFollowCount(account string) (*protocolapi.FollowCountReturn, error)

// condenser_api.get_followers —— [account, start, followType, limit]
func (a *API) GetFollowers(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error)
// condenser_api.get_following —— 同形
func (a *API) GetFollowing(account, start, followType string, limit int) ([]*protocolapi.FollowReturn, error)

// condenser_api.get_account_history —— [account, from, limit]
func (a *API) GetAccountHistory(account string, from int64, limit int) ([]*AccountHistoryEntry, error)

// condenser_api.lookup_accounts —— [lowerBound, limit]，返回 []string
func (a *API) LookupAccounts(lowerBound string, limit int) ([]string, error)

// database_api.get_order_book —— [limit]（namespace 对齐 conveyor price.ts）
func (a *API) GetOrderBook(limit int) (*protocolapi.OrderBook, error)
// database_api.get_feed_history —— 无参
func (a *API) GetFeedHistory() (*protocolapi.FeedHistory, error)
// database_api.get_dynamic_global_properties —— 无参（确认已有封装的签名兼容；G3 价格算术用它）
func (a *API) GetDynamicGlobalProperties() (*protocolapi.DynamicGlobalProperties, error)
```

**新增类型**（`api/account_history.go`）：
```go
type AccountHistoryEntry struct {
    Index     int64
    Op        AccountHistoryOp
    Timestamp string
}
type AccountHistoryOp struct {
    Type    string
    Payload map[string]interface{} // 转账: to/amount/memo 等
}
// UnmarshalJSON：输入 [index, {"op":[type, payload], "timestamp": ts}]
//   首元素 → Index；次元素体里取 "op"（[type, payload]）和 "timestamp"。
```

**对齐点**（核对 conveyor `user-search/client.ts`）：
- `GetAccounts` 参数 `[]interface{}{names}`（外层位置数组），非命名对象。
- `GetAccountHistory` 的 `from`/`limit` 与 conveyor `accountHistoryGenerator` 一致：首页 `from=-1, limit=1000`（-1=最新）；**SDK 只提供单页，分页循环由 conveyor 自行驱动**。
- `GetOrderBook`/`GetFeedHistory` 用 `database_api`（与 conveyor `price.ts` 一致）。

### G1. `__signed` 验证便捷封装（`api/verify.go`，新文件）

依赖 v0.0.24 的 `rpc.Validate` + `rpc.VerifySignedRpc` + 步骤 G2 的 `GetAccounts`。

```go
// VerifySignedRequest 验证收到的 __signed JSON-RPC 请求，基于 a.url 节点查账户 posting authority。
// 返回 (decodedParams, account, error)。镜像 @steemit/koa-jsonrpc 行为，供 conveyor 等服务端使用。
func (a *API) VerifySignedRequest(signedReq *rpc.SignedRequest) (params []interface{}, account string, err error)
```

内部：
1. `account = signedReq.Params.Signed.Account`
2. 构造 `rpc.AccountFetcher` 闭包：
   - `accts, err := a.GetAccounts([]string{acct})` → `len==0` 或 err → 返回 error（被 `VerifySignedRpc` 报为 "No such account"）
   - 返回 `accts[0].Posting`（类型恰是 `protocolapi.Authority` = `AccountFetcher` 返回类型，**零转换**）
3. `params, err = rpc.Validate(signedReq, func(msg, sigs, acct string) error {
       return rpc.VerifySignedRpc(msg, sigs, acct, fetcher)
   })`
4. 返回 `params, account, err`。

> 注：`GetAccounts` 用 `condenser_api.get_accounts`，`ExtendedAccount.Posting` 的类型与 `AccountFetcher` 返回类型完全一致，直接透传，无需任何转换胶水。

### （已上移）原 G3 价格算术

价格算术（Asset/Price/ComputePrices）已上移到 steemutil v0.0.25（见 `/home/ety001/workspace/agent-share/notes/steemutil/asset-price-plan.md`）。steemgosdk 不再实现，conveyor 直接调 `protocolapi.ComputePrices`。

## 验收标准

1. `VerifySignedRequest` 对自签请求返回正确 params/account；篡改签名/账户失败；多 key_auths → "Unsupported..."（单测）。
2. G2 每方法对 fixture JSON 正确反序列化；`key_auths` 的 `[["STMxxx",1]]` 嵌套由 steemutil `KeyAuth.UnmarshalJSON` 处理（单测验证其工作）。
3. `GetAccountHistory` 的 from/limit 与 conveyor `accountHistoryGenerator` 语义一致（首页 -1/1000）。
4. 每项单测覆盖。

## 测试组织

- `api/testdata/` 内嵌真实 JSON 响应快照（`go:embed`）：get_accounts、get_followers、get_follow_count、get_order_book、get_feed_history、get_account_history、get_dynamic_global_properties。
- **确定性单测**（默认 `go test`，无 build tag）：`httptest.Server` 喂固定 JSON，断言反序列化字段。
  - **G1**：用 `rpc.Sign` 自签（向量：WIF `5JWHY5DxTF6qN5eZpH9iBma` → STM7jNh5ejQoqHqWcGWFJ1v4F5CzsG3EiBuz1VooCng1cH5QpJD27，account `"testaccount"`，源自 steemutil `verify_test.go` 的 `jsVector`），`VerifySignedRequest` 通过 httptest 模拟 `get_accounts` 返回对应 posting authority → 通过；篡改 → 失败。
- **集成测试**（`//go:build integration`）：真实打 `https://api.steemit.com`，CI 默认不跑（`-tags=integration` 手动验证）。

## 风险与备注

- **`order_price` wire 格式**：已确认 `OrderPrice.Base/Quote` 是 string-asset（condenser 风格 `"1.000 SBD"`）。若真实节点对 `database_api.get_order_book` 返回结构化 asset 对象，集成测试会暴露，届时在 steemgosdk 加适配类型（不改 steemutil）。
- **go 1.18 兼容**：`go:embed` 需 1.16+，已满足；无 1.18+ 独有特性。
- **零新增第三方依赖**（steemgosdk 侧）。
- 整个改造**仅限 steemgosdk 仓库**，steemutil v0.0.24 已发布，v0.0.25（Asset/Price）由对侧并行推进。
