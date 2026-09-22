-- armature-iam schema + migration from legacy table_users/table_sessions

CREATE TABLE user_accounts (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(255) NOT NULL DEFAULT '',
  created_by VARCHAR(32) NOT NULL DEFAULT 'admin-created',
  initial_route VARCHAR(255) NOT NULL DEFAULT '/',
  PRIMARY KEY (id),
  UNIQUE KEY idx_user_accounts_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE sessions (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  sid VARCHAR(64) NOT NULL,
  user_account_id INT UNSIGNED NOT NULL,
  impersonator_user_id INT UNSIGNED DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_sessions_sid (sid),
  KEY fk_sessions_user (user_account_id),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_account_id) REFERENCES user_accounts (id) ON DELETE CASCADE,
  CONSTRAINT fk_sessions_impersonator FOREIGN KEY (impersonator_user_id) REFERENCES user_accounts (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE api_keys (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  user_account_id INT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  key_value VARCHAR(128) NOT NULL,
  read_only TINYINT(1) NOT NULL DEFAULT 0,
  last_used_at DATETIME(3) DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_api_keys_key_value (key_value),
  KEY fk_api_keys_user (user_account_id),
  CONSTRAINT fk_api_keys_user FOREIGN KEY (user_account_id) REFERENCES user_accounts (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE user_groups (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  name VARCHAR(100) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_user_groups_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE user_group_memberships (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  user_account_id INT UNSIGNED NOT NULL,
  user_group_id INT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_user_group_membership (user_account_id, user_group_id),
  KEY fk_ugm_group (user_group_id),
  CONSTRAINT fk_ugm_user FOREIGN KEY (user_account_id) REFERENCES user_accounts (id) ON DELETE CASCADE,
  CONSTRAINT fk_ugm_group FOREIGN KEY (user_group_id) REFERENCES user_groups (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE rbac_permissions (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  name VARCHAR(191) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY idx_rbac_permissions_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE rbac_roles (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) DEFAULT NULL,
  updated_at DATETIME(3) DEFAULT NULL,
  name VARCHAR(191) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY idx_rbac_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE rbac_role_permissions (
  role_id INT UNSIGNED NOT NULL,
  permission_id INT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  KEY fk_rrp_perm (permission_id),
  CONSTRAINT fk_rrp_role FOREIGN KEY (role_id) REFERENCES rbac_roles (id) ON DELETE CASCADE,
  CONSTRAINT fk_rrp_perm FOREIGN KEY (permission_id) REFERENCES rbac_permissions (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE rbac_group_roles (
  user_group_id INT UNSIGNED NOT NULL,
  role_id INT UNSIGNED NOT NULL,
  PRIMARY KEY (user_group_id, role_id),
  KEY fk_rgr_role (role_id),
  CONSTRAINT fk_rgr_group FOREIGN KEY (user_group_id) REFERENCES user_groups (id) ON DELETE CASCADE,
  CONSTRAINT fk_rgr_role FOREIGN KEY (role_id) REFERENCES rbac_roles (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO rbac_permissions (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'app.access', 'Use the application (non-admin APIs)'),
(NOW(3), NOW(3), 'users.view', 'List and view user accounts'),
(NOW(3), NOW(3), 'users.create', 'Create user accounts'),
(NOW(3), NOW(3), 'users.delete', 'Delete user accounts'),
(NOW(3), NOW(3), 'users.reset-password', 'Reset passwords for other users'),
(NOW(3), NOW(3), 'usergroups.view', 'List user groups and view membership'),
(NOW(3), NOW(3), 'usergroups.manage', 'Create and delete user groups; manage membership'),
(NOW(3), NOW(3), 'rbac.view', 'View roles and permissions'),
(NOW(3), NOW(3), 'rbac.manage', 'Create, update, and delete roles; assign roles to groups'),
(NOW(3), NOW(3), 'system.settings', 'View and modify system settings'),
(NOW(3), NOW(3), 'system.logs', 'View logs, job status, and run maintenance'),
(NOW(3), NOW(3), 'system.impersonate', 'Impersonate other users');

INSERT INTO rbac_roles (created_at, updated_at, name, description) VALUES
(NOW(3), NOW(3), 'superuser', 'All permissions (system role)'),
(NOW(3), NOW(3), 'member', 'Standard application access');

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r CROSS JOIN rbac_permissions p WHERE r.name = 'superuser';

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.access' WHERE r.name = 'member';

INSERT INTO user_groups (name, created_at, updated_at) VALUES
('Everyone', NOW(3), NOW(3)),
('Administrators', NOW(3), NOW(3));

INSERT INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g CROSS JOIN rbac_roles r
WHERE g.name = 'Everyone' AND r.name = 'member';

INSERT INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g CROSS JOIN rbac_roles r
WHERE g.name = 'Administrators' AND r.name = 'superuser';

INSERT INTO user_accounts (id, username, password_hash, created_by, initial_route, created_at, updated_at)
SELECT id, username, COALESCE(password, ''), 'migrated', COALESCE(initial_route, '/'), NOW(3), NOW(3)
FROM table_users;

INSERT INTO sessions (sid, user_account_id, created_at, updated_at)
SELECT s.session_id, u.id, COALESCE(s.created_at, NOW(3)), COALESCE(s.last_accessed, NOW(3))
FROM table_sessions s
INNER JOIN user_accounts u ON u.username = s.username;

INSERT IGNORE INTO user_group_memberships (user_account_id, user_group_id, created_at, updated_at)
SELECT u.id, g.id, NOW(3), NOW(3) FROM user_accounts u CROSS JOIN user_groups g WHERE g.name = 'Everyone';

INSERT IGNORE INTO user_group_memberships (user_account_id, user_group_id, created_at, updated_at)
SELECT u.id, g.id, NOW(3), NOW(3) FROM user_accounts u CROSS JOIN user_groups g
WHERE g.name = 'Administrators' AND u.username = 'admin';
