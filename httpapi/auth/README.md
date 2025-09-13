# Auth 端點

Auth 端點提供適合供網頁應用程式使用的認證 API。

## 登入帳號

使用 `POST /api/auth/v2/authorize/google` 登入帳號。

GET 時，您需要帶入這些查詢參數 (query string)：

- `response_type`：目前只支援授權碼模式，必須是 `code`
- `redirect_uri`：要接收 token 的 callback endpoint，比如 `https://www.dbplay.app/api/auth/callback`
- `state`：要傳給 redirect URI 的狀態參數
- `code_challenge`：雜湊後的授權碼，在 callback 中取回 token 時會用到。
- `code_challenge_method`：必須是 `S256`

`code_challenge` 的雜湊方式如下：

```plain
code_challenge = BASE64URL-ENCODE(SHA256(ASCII(code_verifier)))
```

登入完成後，會自動跳轉到 `redirect_uri` 上，接著您可以在 redirect URI（下稱 callback）中取回 token。

如果失敗，則會回傳符合 [RFC 6749 的錯誤回傳值](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1)，如：

```json
{
    "error": "invalid_request",
    "error_description": "Bad redirect URI.",
    "state": ""
}
```

### Callback 會收到的參數

在驗證完成後，瀏覽器會跳轉到 `redirect_uri`，並帶入以下的查詢字串：

- `code`：取回 token 的授權碼
- `state`：你在〈登入帳號〉中傳入的狀態參數
- `code_challenge`：你在〈登入帳號〉中傳入的雜湊授權碼
- `code_challenge_method`：你在〈登入帳號〉中傳入的雜湊授權碼

接著您可以使用〈取回 token〉API 來取得 token。

## 取回 token

使用 `POST /api/auth/v2/token` 取回 token。

POST 時，您需要帶入這些查詢參數 (query string)：

- `grant_type`：目前只支援授權碼模式，必須是 `authorization_code`
- `code`：你在 `redirect_uri` 收到的授權碼
- `redirect_uri`：重新導向連結，必須與〈登入帳號〉的 redirect URI 相同

如果一切順利的話，會回傳 token、token type、過期時間等資訊：

```json
{
    "token_type": "Bearer",
    "access_token": "2YotnFZFEjr1zCsicMWpAA",
    "expires_in": 28800
}
```

## 授權

請將 access token 帶入 `Authorization` 標頭中，格式如下：

```plain
Authorization: Bearer <access_token>
```

預設 `access_token` 會存活 8 小時，且只要 token 有人存取就會延長。

如果需要登出的話，除了使用〈登出帳號〉API 撤銷特定 token 外，也可以使用 GraphQL 的批次撤銷來處理。

## 登出帳號

您可以使用 `POST /api/auth/v2/revoke` 登出帳號。

需要帶入以 `application/x-www-form-urlencoded` 編碼的請求體：

- `token`：要 revoke 的 token
- `token_type_hint`：必須是 `access_token`

如果 Token 撤回失敗，則會回傳 HTTP 500 錯誤並帶上錯誤資訊：

```json
{
    "error": "server_error",
    "error_description": "Failed to revoke the token. Please try again later."
}
```

如果 Token 撤回成功，則回傳 HTTP 200 OK，此時您可以重新整理登入狀態。

如果沒有 Auth Token 或者是 token 無效，則依然回傳 HTTP 200。請引導使用者重新登入。

## 參考來源

為了保證登入時的資訊安全，這裡參考了兩份 RFC 進行 API 的設計：

- [RFC 6749 – The OAuth 2.0 Authorization Framework](https://datatracker.ietf.org/doc/html/rfc6749#autoid-35)
- [RFC 7636 – Proof Key for Code Exchange by OAuth Public Clients](https://datatracker.ietf.org/doc/html/rfc7636#section-4.1)
- [RFC 7009 – OAuth 2.0 Token Revocation](https://datatracker.ietf.org/doc/html/rfc7009)
