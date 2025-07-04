# Authentication

這個 Package 的目的是操作認證憑證（token）。

## Token 的設計

目前 Token 是實作成一個 Base64 格式的 URL-safe 64 位元組隨機字串。

```plain
0E3ZZhnnBENG9oz8IeIzbFx0EzyXa_pEK32kjWaZVtliD1SOXsA2gHGeSfwOu_8i
```

> [!NOTE]
> 不設計成 JWT，是因為在 HttpOnly 的情境下，前端沒有機會分析這段 token，而後端存取內網 Redis 的速度足夠快，因此就不採用 JWT 來避免掉設計和 rotate refresh token 以及針對 JWT 做強化所造成的種種開銷（如維護 Secret Key）。

## HTTP Middleware

auth 中的 `Middleware` 會嘗試擷取所有經此 Middleware 的 HTTP request 中的 Bearer token。

Token 會從兩個來源取得：

- Cookies `auth-token` (`CookieAuthToken`)：如果你是使用 Google OAuth 登入的，這個後端的 `callback` 會設定 `HttpOnly` 的這個 cookie，為最主要的認證方式。
- `Authorization` 標頭中的 Bearer token：如果你需要在瀏覽器情境之外呼叫 API 進行測試，可以在取得 token 後傳入 `Authorization: Bearer [token]` 標頭。行為與 `auth-token` 一致。

如果驗證失敗（無論錯誤的 token 格式，還是無效 token），均會回傳 GraphQL 風格的 `UNAUTHORIZED` 錯誤。如果驗證成功，則將此 token 對應的使用者資訊 (`UserInfo`) 寫入請求情境鏈 (Context) 中。如果沒有帶入 Token，則不帶任何資訊進情境鏈中。

### Context

Middleware 會在請求情境鏈中插入 `UserInfo`，resolvers 可以使用 `GetUser(ctx)` 取得這個使用者的資訊 (`UserInfo`)。`WithUser(ctx)` 是供 Middleware 在解析 token 時使用，除整合測試外您無需手動插入 `UserInfo。`

## Storage

Storage 介面要求實作「建立 token（登入）」、「取回 token（驗證和展期）」、「檢驗 token（Peek）」、「刪除特定 token（登出）」、「刪除使用者底下所有 token（登出所有裝置）」這些功能。

建立 token 需要你帶入 user ID 和 machine ID。前者你可以使用 User model 的 `id`，後者你可以使用請求方的 User-Agent。取回 token 則會回傳你建立時帶入的資訊，且定義 `ErrNotFound` 錯誤為無此 token。

登出需要你帶入 token 本身，我們會將這個 token 撤銷，使得其無法取回而顯示未驗證狀態。登出所有裝置則要求撤銷符合這個 User ID 的所有 tokens，使得簽發至這個使用者的所有 tokens 均無法取回。

除 `ErrNotFound` 以外的錯誤均為實作方自行定義，呼叫方需注意防止錯誤訊息中的機密資訊外洩。

所有 Storage 都應該在 `DefaultTokenExpire` 指定的到期時間（8 小時）後將其持有的 token 刪除，`Get()` 應該將 token 的到期時間恢復到 `DefaultTokenExpire`。

### Redis 介面

auth 套件底下的 Redis 以這個鍵儲存資料：

```jsx
auth:token:TOKEN -> JSON({ user_id, machine_id, scopes })
```

其中 `auth:token:TOKEN` 的資料以 [Redis 的 JSON 資料型態](https://redis.io/docs/latest/develop/data-types/json/) 進行操作，同時採用 Storage 介面定義的到期時間和到期規則（但你可以因應測試需求修改到期時間）。

為了降低「登出所有裝置」的呼叫成本，我們使用 Redis 的 `SCAN` 命令來走訪所有以 `auth:token:` 為前綴的鍵，並使用 `JSON.GET` 命令單獨取回並解析每個 token 的 user 欄位（而不是對著整個結構做 `json.Unmarshal`）。當找到屬於目標使用者的 token 時，就使用 DEL 命令將其刪除。這樣即可在不引入額外索引的前提下，相對有效率地找出並刪除特定使用者的所有 tokens。

> [!NOTE]
> 建立一個 `user -> set<token>` 的索引，理論上可以使 `DeleteByUser` 執行得更為高效，但現有的「走訪所有 tokens」已經使用 SCAN 命令的漸進式掃描設計，並且只針對字串進行 Unmarshal 的行為，即使在大量 tokens 的情況下也足以確保效能表現穩定（維持在 $O(n)$）且儲存需求恆定（一次只處理一個 token 並隨後釋放，而不是在記憶體儲存所有 tokens）。最後，`DeleteByUser` 不是一個會一直被呼叫的函式，沒有必要引入增加索引所帶來的維護開銷（e.g. 如何確保 index 和 token key 的同步？）。
