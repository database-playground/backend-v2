# Backend 的環境變數設定

## Backend

- `PORT`：這個伺服器要監聽的連線埠，預設為 `8080`
- `TRUST_PROXIES`：信任的 Proxies 地址（以逗號分隔），如 `10.0.0.0/8,127.0.0.1/8`
- `GIN_MODE`：Gin 的伺服器模式。Release 是生產模式。

## 前端設定

- `ALLOWED_ORIGINS`：允許存取這個 API 的 origins（以逗號分隔），如 `https://domain.tld,https://dev.domain.tld`

## 伺服器設定

- `SERVER_URI`：這個伺服器主要提供服務的 URI，如 `https://backend.domain.tld`
- `SERVER_CERT_FILE`：若要啟用 TLS 伺服器，這個選項可以讓你指定 TLS certificate 對應的檔案。
- `SERVER_KEY_FILE`：若要啟用 TLS 伺服器，這個選項可以讓你指定 TLS key 對應的檔案。

如果您在開發環境，可以使用 `mkcert` 產生一組金鑰方便測試 Secure Cookies：

```shell
nix run nixpkgs#mkcert -- -install
nix run nixpkgs#mkcert -- localhost
```

由此產生的 `localhost-key.pem` 對應到 `SERVER_KEY_FILE`；`localhost.pem` 對應到 `SERVER_CERT_FILE`

## Redis

Redis 的目的是儲存認證憑證和快取。

- `REDIS_HOST`：Redis 的位址，如 `redis.network.internal`
- `REDIS_PORT`：Redis 的連線埠，如 `6379`
- `REDIS_USERNAME`：Redis 的使用者名稱，可留空
- `REDIS_PASSWORD`：Redis 的密碼，可留空

## 資料庫

Database Playground 使用 PostgreSQL 作為資料庫。

- `DATABASE_URI`：`postgres://<username>:<password>@<host>:<port>/<db>` 格式的連線字串

## Google OAuth

這個 backend 以 Google OAuth 登入為主。

- `GAUTH_SECRET`：用來加密認證相關請求的 secret
- `GAUTH_CLIENT_ID`：Google OAuth 的 Client ID
- `GAUTH_CLIENT_SECRET`：Google OAuth 的 Client Secret
- `GAUTH_REDIRECT_URIS`：在完成 Google OAuth 流程後，允許重新導向到的 URIs。
  - 舉例：`https://admin.dbplay.app`

OAuth 的使用方式請參考 [Auth 端點](../httpapi/auth/README.md) 內容。

Google OAuth 的「已授權的重新導向 URI」應包含 `https://HOST/api/auth/v2/callback/google` 端點。

## SQL Runner

- `SQL_RUNNER_URI`：[SQL Runner API](https://github.com/database-playground/sqlrunner-v2) 的連線 URL，如 `https://sqlrunner.dbplay.app`。部署說明可參見 [Usage > Starting the service](https://github.com/database-playground/sqlrunner-v2/tree/main?tab=readme-ov-file#starting-the-service)。

## PostHog 設定

PostHog 是一個產品統計平台。這個專案使用 [posthog-go](https://posthog.com/docs/libraries/go) 做後端的 event 寫入。

如果不填寫 API Key 則代表不送出任何統計。

- `POSTHOG_API_KEY`: PostHog 的 API key。可以在 PostHog 的 Settings > Project > General > Project API key 中取得。
- `POSTHOG_HOST`: PostHog API 的主機。可以在 PostHog 的 Settings > Project > General > Web snippet 中的 `api_host` 取得。
  - e.g. `https://us.i.posthog.com`

## OpenTelemetry 設定

支援使用環境變數設定 OpenTelemetry（Tracing 與 Logging），詳細規範可參考 [官方文件](https://opentelemetry.io/docs/languages/sdk-configuration/general/)。

以下是常用的設定變數：

- `OTEL_SERVICE_NAME`：指定此服務的名稱，例如 `dbplay-backend-replica-1`。
- `OTEL_TRACES_EXPORTER`：指定追蹤資料（Traces）的匯出方式。
  - 支援值：`console`, `otlp`
  - 預設值：`console`
- `OTEL_LOGS_EXPORTER`：指定日誌資料（Logs）的匯出方式。
  - 支援值：`console`, `otlp`
  - 預設值：`console`
- `OTEL_EXPORTER_OTLP_PROTOCOL`：指定 OTLP 的預設通訊協定。
  - 支援值：`grpc`, `http/protobuf`
  - 預設值：`grpc`
- `OTEL_EXPORTER_OTLP_ENDPOINT`：指定 OTLP 的預設 Endpoint 位址。
  - 舉例：`http://otlp-collector:4317`
- `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL`：指定追蹤資料專用的 OTLP 通訊協定。
  - 支援值：`grpc`, `http/protobuf`
  - 預設值：若未設定，則使用 `OTEL_EXPORTER_OTLP_PROTOCOL` 的值。
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`：指定追蹤資料專用的 OTLP Endpoint 位址。
  - 舉例：`http://victoriatraces:10428/insert/opentelemetry/v1/traces`
- `OTEL_EXPORTER_OTLP_LOGS_PROTOCOL`：指定日誌資料專用的 OTLP 通訊協定。
  - 支援值：`grpc`, `http/protobuf`
  - 預設值：若未設定，則使用 `OTEL_EXPORTER_OTLP_PROTOCOL` 的值。
- `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`：指定日誌資料專用的 OTLP Endpoint 位址。
  - 舉例：`http://victorialogs:9428/insert/opentelemetry/v1/logs`
