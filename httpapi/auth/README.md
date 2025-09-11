# Auth 端點

Auth 端點提供適合供網頁應用程式使用的認證 API。

## 登出帳號

您可以使用 `POST /api/logout` 登出帳號。

如果 Token 撤回失敗，則會回傳 HTTP 500 錯誤並帶上錯誤資訊：

```json
{
    "error": "Failed to revoke the token. Please try again later.",
    "detail": "(error details)",
}
```

如果 Token 撤回成功，則回傳 HTTP 205 (Reset Content)，此時您可以重新整理登入狀態。

如果沒有 Auth Token 或者是 token 無效，則依然回傳 HTTP 205。請引導使用者重新登入。

## Google 登入

如果您要觸發 Google 登入的流程，請前往 `GET /api/auth/google/login`。可以帶入 `redirect_uri` 參數來在登入完成後轉導到指定畫面。

這個頁面會重新導向到 Google 的登入頁面，登入後會回到 `POST /api/auth/google/callback` 並進行帳號登入和註冊手續。
