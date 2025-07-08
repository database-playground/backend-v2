# Database Playground 的 Backend API

## 環境

您需要預備 Redis、PostgreSQL 和 Google OAuth 的憑證。

### 本機測試

```shell
docker run -it --rm --name dp-redis -d redis
docker run -it --rm --name dp-postgres -e POSTGRES_PASSWORD=databaseplayground -d postgres
```

```env
REDIS_HOST=dp-redis.orb.local
REDIS_PORT=6379
DATABASE_URL=postgres://postgres:databaseplayground@dp-postgres.orb.local:5432/postgres
```

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

您需要先產生一組 TLS 金鑰並在 `.env` 定義，請參見 [組態設定說明](./docs/config.md)。

定義完成後，執行：

```shell
go run ./cmd/backend
```

會在 `localhost:8080` 啟動 HTTPS 的伺服器。

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

Linting & Formatting:

```shell
golangci-lint run
gofumpt -w .
```
