package database

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS sys_users (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		username VARCHAR(64) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY uk_sys_users_username (username)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_roles (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		role_code VARCHAR(64) NOT NULL,
		role_name VARCHAR(64) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY uk_sys_roles_code (role_code)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_menus (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		parent_id BIGINT UNSIGNED NULL,
		menu_name VARCHAR(64) NOT NULL,
		route_path VARCHAR(255) NULL,
		sort_order INT NOT NULL DEFAULT 0,
		PRIMARY KEY (id),
		KEY idx_sys_menus_parent (parent_id),
		CONSTRAINT fk_sys_menus_parent FOREIGN KEY (parent_id) REFERENCES sys_menus(id)
			ON DELETE SET NULL ON UPDATE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_permissions (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		permission_code VARCHAR(128) NOT NULL,
		permission_name VARCHAR(64) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY uk_sys_permissions_code (permission_code)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_user_roles (
		user_id BIGINT UNSIGNED NOT NULL,
		role_id BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (user_id, role_id),
		KEY idx_sys_user_roles_role (role_id),
		CONSTRAINT fk_sys_user_roles_user FOREIGN KEY (user_id) REFERENCES sys_users(id)
			ON DELETE CASCADE ON UPDATE CASCADE,
		CONSTRAINT fk_sys_user_roles_role FOREIGN KEY (role_id) REFERENCES sys_roles(id)
			ON DELETE CASCADE ON UPDATE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_role_permissions (
		role_id BIGINT UNSIGNED NOT NULL,
		permission_id BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (role_id, permission_id),
		KEY idx_sys_role_permissions_permission (permission_id),
		CONSTRAINT fk_sys_role_permissions_role FOREIGN KEY (role_id) REFERENCES sys_roles(id)
			ON DELETE CASCADE ON UPDATE CASCADE,
		CONSTRAINT fk_sys_role_permissions_permission FOREIGN KEY (permission_id) REFERENCES sys_permissions(id)
			ON DELETE CASCADE ON UPDATE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS sys_role_menus (
		role_id BIGINT UNSIGNED NOT NULL,
		menu_id BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (role_id, menu_id),
		KEY idx_sys_role_menus_menu (menu_id),
		CONSTRAINT fk_sys_role_menus_role FOREIGN KEY (role_id) REFERENCES sys_roles(id)
			ON DELETE CASCADE ON UPDATE CASCADE,
		CONSTRAINT fk_sys_role_menus_menu FOREIGN KEY (menu_id) REFERENCES sys_menus(id)
			ON DELETE CASCADE ON UPDATE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

var seedStatements = []string{
	`INSERT IGNORE INTO sys_roles (role_code, role_name)
	 VALUES ('admin', 'administrator')`,
	`INSERT IGNORE INTO sys_roles (role_code, role_name)
	 VALUES ('member', 'member')`,
	`INSERT IGNORE INTO sys_menus (id, parent_id, menu_name, route_path, sort_order)
	 VALUES
		(1, NULL, '首页', '/home', 10),
		(2, NULL, '系统设置', NULL, 20),
		(3, NULL, '大屏', '/dashboard', 30),
		(4, NULL, '消息通知', '/notifications', 40)`,
	`INSERT IGNORE INTO sys_menus (id, parent_id, menu_name, route_path, sort_order)
	 VALUES
		(5, 2, '用户管理', '/system/users', 10),
		(6, 2, '角色管理', '/system/roles', 20),
		(7, 2, '权限管理', '/system/permissions', 30),
		(8, 2, '菜单管理', '/system/menus', 40)`,
	`INSERT IGNORE INTO sys_permissions (permission_code, permission_name)
	 VALUES
		('home:view', '查看首页'),
		('system:user:view', '查看用户'),
		('system:user:write', '管理用户'),
		('system:role:view', '查看角色'),
		('system:role:write', '管理角色'),
		('system:permission:view', '查看权限'),
		('system:menu:view', '查看菜单'),
		('dashboard:view', '查看大屏'),
		('notification:view', '查看消息通知')`,
	`INSERT IGNORE INTO sys_role_menus (role_id, menu_id)
	 SELECT r.id, m.id FROM sys_roles r JOIN sys_menus m
	 WHERE r.role_code = 'admin'`,
	`INSERT IGNORE INTO sys_role_permissions (role_id, permission_id)
	 SELECT r.id, p.id FROM sys_roles r JOIN sys_permissions p
	 WHERE r.role_code = 'admin'`,
	`INSERT IGNORE INTO sys_role_menus (role_id, menu_id)
	 SELECT r.id, m.id FROM sys_roles r JOIN sys_menus m
	 WHERE r.role_code = 'member' AND m.route_path IN ('/home', '/notifications')`,
	`INSERT IGNORE INTO sys_role_permissions (role_id, permission_id)
	 SELECT r.id, p.id FROM sys_roles r JOIN sys_permissions p
	 WHERE r.role_code = 'member' AND p.permission_code IN ('home:view', 'notification:view')`,
}
