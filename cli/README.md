# cli

CLI 提供可複用的 CLI 服務方法。

如果用 [MVCS](https://pvha.hashnode.dev/mvcs-architecture) 的架構來看：

- 這個套件是 Service
- [cmd/admin-cli](./cmd/admin-cli) 是 Controller

## Migrate 和 Setup

請參見 [setup](../internal/setup/README.md) 套件的文件。

## PromoteAdmin

將一名現有的使用者提升成管理員 (`admin`)。

需要傳入這名使用者的 email，這個方法會自動將使用者的群組更改為管理員群組 (`admin`)。
