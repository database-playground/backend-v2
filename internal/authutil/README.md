# authutil

這個 package 包含一些產生驗證 token 和 state 實用的函式。

## `GenerateToken`

產生一個 URL-safe 64 位元組隨機字串。

目前已知這個 token：

- 合乎 [RFC 7636 - Proof Key for Code Exchange by OAuth Public Clients](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1) 的 Code Verifier 標準。

範例：

```plain
0E3ZZhnnBENG9oz8IeIzbFx0EzyXa_pEK32kjWaZVtliD1SOXsA2gHGeSfwOu_8i
```
