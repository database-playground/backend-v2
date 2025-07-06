# GraphQL definitions

`defs` 定義了錯誤等等的共通資料結構。

## 錯誤碼

- `NOT_FOUND`：找不到指定的實體。
- `UNAUTHORIZED`：這個 API 需要認證或授權後才能運作。如果權限 (scope) 不足也會顯示這個錯誤。
- `USER_VERIFIED`：使用者已經驗證過，不用重複驗證。目前只用於 `verifyRegistration` mutation。
