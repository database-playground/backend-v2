# HTTP API

Database Playground 大部分的 API 均以 GraphQL 形式提供 (`/query`)，但部分為 BFF (Backend for Frontend) 設定的 Stateful Endpoints 則是以 HTTP API 進行設計，並以 `/api` 為開頭。

> [!WARNING]
> 注意 HTTP API 不會帶入 AuthMiddleware。如果你的 API 需要鑒權，請手動帶入 `auth.Middleware`。

- [認證](./auth)：相關方法均列於 `/api/auth` 路徑底下。
