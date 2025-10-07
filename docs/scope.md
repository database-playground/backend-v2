# Scope 的資源和動作對照表

## 資源

- `me`：針對自身的操作
- `user`：使用者操作
  - 包含登入記錄、點數查詢
- `group`：群組操作
- `scopeset`：範圍集合操作
- `database`：題庫對應資料庫的操作
- `question`：題庫操作
  - `answer`：解答（只有 `read` 動作，`answer:write` 被 `question:write` 涵蓋）
- `submission`：提交紀錄操作（做題）

## 動作

- `read`：查詢指定的資源。範圍由 resolvers 決定。
- `write`：更動指定的資源。具體範圍由 resolvers 決定。

## 特殊 scopes

- `user:impersonate`：給定任意使用者的 ID，允許假冒其身分操作。
- `me:delete`：刪除自己的帳號。
- `ai`：使用 AI 的權限（目前在前端判斷）。
