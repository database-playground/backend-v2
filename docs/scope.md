# Scope 的資源和動作對照表

## 資源

- `user`：使用者操作
- `group`：群組操作
- `scopeset`：範圍集合操作

## 動作

- `read`：查詢指定的資源。範圍由 resolvers 決定。
- `write`：更動指定的資源。具體範圍由 resolvers 決定。

## 特殊 scopes

- `user:impersonate`：給定任意使用者的 ID，允許假冒其身分操作。
