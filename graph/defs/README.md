# GraphQL definitions

`defs` 定義了錯誤等等的共通資料結構。

## 錯誤碼

- `NOT_FOUND`：找不到指定的實體。
- `UNAUTHORIZED`：這個 API 需要認證或授權後才能運作。
- `NOT_IMPLEMENTED`：這個 API 尚未實作，請先不要呼叫。
- `FORBIDDEN`：使用者的權限 (scope) 不足以執行這個操作。
- `INVALID_INPUT`：輸入有誤。
