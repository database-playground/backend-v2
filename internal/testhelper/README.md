# testhelper

testhelper 封裝一些常見的測試物件建立案例。

## Redis

如果一個測試需要引入 `rueidis` 的整合測試 Client，你可以使用 testhelper 中的 `NewRedisContainer` 和 `NewRedisClient` 組合來取得這個測試專屬的 Redis 實例，不同 `redisClient` 間的操作不會影響。

```go
container := testhelper.NewRedisContainer(t)
redisClient := testhelper.NewRedisClient(t, container)
```

這部分是使用 `testcontainer` 實作的，因此這個測試要求 Docker 環境建立容器。如果沒有 Docker 環境，則會直接觸發 `t.Skip` 略過此測試。

你不需要 clean up：這兩個方法都實作了 `t.Cleanup` 關閉 Redis client 和 Redis 容器。

## Ent

如果一個測試需要引入 `ent` 的 SQLite Client，你可以使用 testhelper 中的 `NewEntSqliteClient` 來取得以獨立純記憶體 SQLite 為資料庫來源的 Ent 實例。

```go
entClient := testhelper.NewEntSqliteClient(t)
```

你不需要 clean up：這個方法實作了 `t.Cleanup` 關閉 Ent 用戶端並釋放記憶體。

## SQL Runner

如果一個測試需要引入 SQL Runner 來執行 SQL 語句，你可以使用 testhelper 中的 `NewSQLRunnerClient` 來取得 SQL Runner 實例。

```go
sqlrunner := testhelper.NewSQLRunnerClient(t)
```

你不需要 clean up：這個方法實作了 `t.Cleanup` 關閉 SQL Runner 用戶端並釋放記憶體。
