# GraphQL resolvers and definitions

這裡放置 GraphQL 的 resolvers 和相關類型定義。

## 架構

每個領域 (domain) 都應該是個獨立的 `graphqls` 檔案，如 `user.graphqls`。這樣使用 `go generate` 產生 resolvers 時，其對應的檔名會是 `user.resolvers.go`。

`resolver.go` 放置所有 resolvers 的共同依賴。

`ent.graphqls` 是由 `ent` 產生的，請不要修改。

## 欄位定義原則

- 除非這個欄位允許被未登入者存取，否則請對所有方法加上 `@scope`。定義指南請參考 [directive 文件](./directive/README.md)
- 使用 `extend type` 來補充 query 和 mutation。
- 請將錯誤定義在 [defs](./defs) 當中，如果是新的錯誤碼，請在 README 闡明用途。
