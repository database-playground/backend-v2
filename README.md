# Database Playground 的 Backend API

## 環境

您需要預備 Redis、PostgreSQL 和 Google OAuth 的憑證。

## 初始化

您需要根據 [環境變數設定](./docs/config.md) 的說明設定必要的變數，可以將此類變數寫入 `.env`。

接著，使用 admin-cli 來初始化資料庫欄位（不需要事先啟動 backend）：

```shell
go run ./cmd/admin-cli setup
```

接著到前端使用「Google 登入」建立好使用者後，將這個使用者提升為管理員：

```shell
go run ./cmd/admin-cli promote-admin --email "your-email@gmail.com"
```

即可初始化完成。

## 啟動伺服器

```shell
go run ./cmd/backend
```

會在 `localhost:8080` 啟動 HTTP 的伺服器。

考慮到 Secure cookie 在某些瀏覽器無法正常在 HTTP `localhost` 下運作，你可能需要執行 `local-ssl-proxy` 啟動 HTTPS 的代理伺服器：

```shell
pnpm dlx local-ssl-proxy --source 8081 --target 8080
```

生產模式中需要指定 `GIN_MODE=release`。

## 開發和測試

您需要安裝 Docker 才能執行測試。

```shell
go test -v ./...
```

如果您更動了 GraphQL 或 ent schema，也需要重新產生程式碼：

```shell
go generate ./...
```
