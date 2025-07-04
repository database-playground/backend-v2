# Authentication

這個 Package 的目的是操作認證憑證（token）。

## Token 的設計

目前 Token 是實作成一個 Base64 格式的 URL-safe 64 位元組隨機字串。

```plain
0E3ZZhnnBENG9oz8IeIzbFx0EzyXa_pEK32kjWaZVtliD1SOXsA2gHGeSfwOu_8i
```

不設計成 JWT，是因為在 HttpOnly 的情境下，前端沒有機會分析這段 token，而後端存取內網 Redis 的速度足夠快，因此就不採用 JWT 來避免掉設計 refresh token 以及針對 JWT 做強化所造成的種種開銷。

## HTTP Middleware

auth 中的 `Middleware` 會嘗試擷取所有經此 Middleware 的 HTTP request 中的 Bearer token。如果驗證失敗（無論錯誤的 token 格式，還是無效 token），均會回傳 HTTP 401。如果驗證成功，則將此 token 對應的使用者資訊 (`UserInfo`) 寫入請求情境鏈 (Context) 中。如果沒有帶入 Token，則不帶任何資訊進情境鏈中。

### Context

Middleware 會在請求情境鏈中插入 `UserInfo`，resolvers 可以使用 `GetUser(ctx)` 取得這個使用者的資訊 (`UserInfo`)。`WithUser(ctx)` 是供 Middleware 在解析 token 時使用，除整合測試外您無需手動插入 `UserInfo。`

## Storage

Storage 介面要求實作「建立 token（登入）」、「取回 token（驗證和延期）」、「檢驗 token（Peek）」、「刪除特定 token（登出）」、「刪除使用者底下所有 token（登出所有裝置）」這四個功能。

建立 token 需要你帶入 user ID 和 machine ID。前者你可以使用 User model 的 `id`，後者你可以使用請求方的 User-Agent。取回 token 則會回傳你建立時帶入的資訊，且強制定義 `ErrNotFound` 為無此 token。

登出需要你帶入 token 本身，我們會將這個 token 撤銷，使得其無法取回而顯示未驗證狀態。登出所有裝置則要求撤銷符合這個 User ID 的所有 tokens，使得簽發至這個使用者的所有 tokens 均無法取回。

除 `ErrNotFound` 以外的錯誤均為實作方自行定義，呼叫方需注意防止錯誤訊息中的機密資訊外洩。

所有 Storage 都應該在 `DefaultTokenExpire` 指定的到期時間（8 小時）後將其持有的 token 刪除，`Get()` 應該將 token 的到期時間恢復到 `DefaultTokenExpire`。

### Redis 介面

auth 套件底下的 Redis 以這個鍵儲存資料：

```jsx
auth:token:TOKEN -> JSON({ user_id, machine_id, scopes })
```

其中 `auth:token:TOKEN` 的資料以 [Redis 的 JSON 資料型態](https://redis.io/docs/latest/develop/data-types/json/) 進行操作，同時採用 Storage 介面定義的到期時間和到期規則（但你可以因應測試需求修改到期時間）。

這麼做的目的，是讓「登出所有裝置」的呼叫成本盡可能小：我們使用 Redis 的 SCAN 命令來走訪所有以 `auth:token:` 為前綴的鍵，並使用 JSON.GET 命令來檢查每個 token 的 user 欄位。當找到屬於目標使用者的 token 時，就使用 DEL 命令將其刪除。

這種方式讓我們能夠在不需要額外索引的情況下，有效地找出並刪除特定使用者的所有 tokens。雖然這個操作需要走訪所有 tokens，但由於 SCAN 命令的漸進式掃描設計，即使在大量 tokens 的情況下也能保持穩定的效能表現。考慮到 `DeleteByUser` 不是一個會一直被呼叫的函式，故不考慮針對其特別建立並維護一個索引。
