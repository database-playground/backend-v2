# Scope

這個 Package 的目的是處理授權範圍（scope），用來控制對不同資源和操作的存取權限。

## Scope 的設計

Scope 採用 `resource:action` 的格式，其中：

- `resource` 代表一個領域實體（例如：user、question）
- `action` 代表一個操作（例如：read、write）

```plain
user:read      // 存取使用者資料的讀取權限
question:write // 存取問題資料的寫入權限
```

### 特殊模式

這個套件支援幾種特殊的 scope 模式：

1. 空白 scope (`""`)
   - 代表公開函式，任何人都可以存取
   - 不需要認證

2. 全域萬用字元 (`*`)
   - 授予所有資源和操作的完整存取權限
   - 通常用於管理員或系統操作

3. 資源萬用字元 (`resource:*`)
   - 授予特定資源的所有操作權限
   - 例如：`user:*` 允許對使用者資料進行讀取和寫入

4. 操作萬用字元 (`*:action`)
   - 授予對所有資源的特定操作權限
   - 例如：`*:read` 允許讀取任何資源

## 使用方式

這個套件提供一個 `ShouldAllow` 函式，用來判斷給定的使用者 scope 是否允許存取需要特定 scope 的函式。

```go
func ShouldAllow(fnScope string, userScope []string) bool
```

### 參數說明

- `fnScope`：被存取的函式所需的 scope
- `userScope`：授予使用者的 scope 陣列

### 存取規則

1. 公開函式
   - 如果 `fnScope` 為空，一律允許存取
   - 任何使用者都可以存取，不論其擁有的 scope

2. 私有函式
   - 需要特定的 scope 匹配
   - 符合以下任一條件時允許存取：
     - 完全符合的 scope
     - 相關的萬用字元 scope
     - 全域存取 scope (`*`)

3. 多重 Scope
   - 使用者可以擁有多個 scope
   - 只要其中任一個 scope 允許操作即可存取
   - 例如：`["user:read", "question:write"]` 允許這兩種操作

### 使用範例

```go
// 公開函式 - 永遠允許
ShouldAllow(ctx, "", []string{})                    // true
ShouldAllow(ctx, "", []string{"user:read"})         // true

// 全域存取
ShouldAllow(ctx, "user:read", []string{"*"})        // true
ShouldAllow(ctx, "question:write", []string{"*"})   // true

// 資源萬用字元
ShouldAllow(ctx, "user:read", []string{"user:*"})   // true
ShouldAllow(ctx, "user:write", []string{"user:*"})  // true

// 操作萬用字元
ShouldAllow(ctx, "user:read", []string{"*:read"})   // true
ShouldAllow(ctx, "question:read", []string{"*:read"}) // true

// 多重 scope
ShouldAllow(ctx, "user:read", []string{"user:read", "question:write"}) // true
```

## 整合應用

這個套件通常與 auth 套件搭配使用：

1. auth 套件處理認證（確認使用者身份）
2. scope 套件處理授權（確認使用者權限）

您可以將此套件整合到 HTTP middleware 或 GraphQL resolvers 中，根據已認證使用者的 scope 來執行存取控制。
