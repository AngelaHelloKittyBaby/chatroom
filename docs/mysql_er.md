# MySQL 数据库 ER 图

当前聊天室数据库包含用户、角色、权限、菜单和三张多对多关系表。

```mermaid
erDiagram
    sys_users {
        BIGINT id PK "用户编号"
        VARCHAR username UK "用户名"
        VARCHAR password_hash "密码哈希"
    }

    sys_roles {
        BIGINT id PK "角色编号"
        VARCHAR role_code UK "角色编码"
        VARCHAR role_name "角色名称"
    }

    sys_permissions {
        BIGINT id PK "权限编号"
        VARCHAR permission_code UK "权限编码"
        VARCHAR permission_name "权限名称"
    }

    sys_menus {
        BIGINT id PK "菜单编号"
        BIGINT parent_id FK "父菜单编号"
        VARCHAR menu_name "菜单名称"
        VARCHAR route_path "访问路径"
        INT sort_order "显示顺序"
    }

    sys_user_roles {
        BIGINT user_id PK,FK "用户编号"
        BIGINT role_id PK,FK "角色编号"
    }

    sys_role_permissions {
        BIGINT role_id PK,FK "角色编号"
        BIGINT permission_id PK,FK "权限编号"
    }

    sys_role_menus {
        BIGINT role_id PK,FK "角色编号"
        BIGINT menu_id PK,FK "菜单编号"
    }

    sys_users ||--o{ sys_user_roles : "拥有角色"
    sys_roles ||--o{ sys_user_roles : "分配给用户"
    sys_roles ||--o{ sys_role_permissions : "拥有权限"
    sys_permissions ||--o{ sys_role_permissions : "分配给角色"
    sys_roles ||--o{ sys_role_menus : "拥有菜单"
    sys_menus ||--o{ sys_role_menus : "分配给角色"
    sys_menus o|--o{ sys_menus : "父子菜单"
```

## 关系说明

- 一个用户可以拥有多个角色，一个角色也可以分配给多个用户。
- 一个角色可以拥有多个权限，一个权限也可以分配给多个角色。
- 一个角色可以看到多个菜单，一个菜单也可以分配给多个角色。
- `sys_menus.parent_id` 用于表示菜单层级，顶级菜单的 `parent_id` 为 `NULL`。
- `sys_user_roles`、`sys_role_permissions`、`sys_role_menus` 使用联合主键，防止同一关系重复写入。
