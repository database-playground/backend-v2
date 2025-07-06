# setup

setup 放置資料庫的初始化 (seeding) 共用程式碼。

除了 `cli` 的 setup 指令會用到外，一部分測試（如 `useraccount`）也會需要它。

## 方法

- `Migrate`：只執行 database migration
- `Setup`：執行 database migration 和初始化

## 初始化項目

- `admin` scopeset (`*`) 和 `admin` 群組
- `new-user` scopeset (`me:*`) 和 `new-user` 群組。
- `unverified` scopeset (`[verification:*, me:read]`) 和 `unverified` 群組

> [!INFO]
> Scope 的具體定義，請參考 [scope 文件](../../docs/scope.md)。Wildcard 的意涵請參考 [scope 套件的實作](../scope/README.md)
