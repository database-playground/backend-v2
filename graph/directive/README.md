# GraphQL directives

定義 GraphQL 的修飾指令（directive）。

## `@scope`

`@scope` 定義這個 API 要求的權限（`scope`）。

假如您將一個 mutation field 或 query field 定義了以下的 scope：

```graphql
extend type Mutation {
  """
  Delete the current user.
  """
  deleteMe: Boolean! @scope(scope: "me:delete")
}
```

則只有具有 `me:delete` `me:*` 和 `*`（管理員）scope 的使用者才能執行。

如果沒有修飾 `@scope`，這個方法會是所有存取者（**包括未登入使用者**）可用。反之，在未登入或權限不足的情況下，API 會回傳錯誤碼為 `UNAUTHORIZED` 的回應。
