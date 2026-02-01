package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 纯 Go 实现的 SQLite，无需 CGO

	"kctl/config"
)

// MemoryDBPath 内存数据库标识
const MemoryDBPath = ":memory:"

// DB 数据库封装
type DB struct {
	conn     *sql.DB
	path     string
	inMemory bool
}

// Open 打开数据库
func Open(path string) (*DB, error) {
	if path == "" {
		path = config.DefaultDBPath
	}

	inMemory := path == MemoryDBPath

	// 内存数据库不需要创建目录
	if !inMemory {
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("创建数据库目录失败: %w", err)
			}
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	db := &DB{conn: conn, path: path, inMemory: inMemory}

	if err := db.initSchema(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

// OpenMemory 打开内存数据库（无文件落地）
func OpenMemory() (*DB, error) {
	return Open(MemoryDBPath)
}

// Close 关闭数据库
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn 返回底层连接（用于事务等高级操作）
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Path 返回数据库路径
func (db *DB) Path() string {
	return db.path
}

// IsInMemory 返回是否是内存数据库
func (db *DB) IsInMemory() bool {
	return db.inMemory
}

// initSchema 初始化表结构
func (db *DB) initSchema() error {
	schema := `
	-- Pods 表
	CREATE TABLE IF NOT EXISTS pods (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		uid TEXT UNIQUE NOT NULL,
		node_name TEXT,
		pod_ip TEXT,
		host_ip TEXT,
		phase TEXT,
		service_account TEXT,
		creation_timestamp TEXT,
		containers TEXT,
		volumes TEXT,
		security_context TEXT,
		collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		kubelet_ip TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_pods_namespace ON pods(namespace);
	CREATE INDEX IF NOT EXISTS idx_pods_node ON pods(node_name);
	CREATE INDEX IF NOT EXISTS idx_pods_service_account ON pods(service_account);
	CREATE INDEX IF NOT EXISTS idx_pods_collected_at ON pods(collected_at);

	-- ServiceAccounts 表
	CREATE TABLE IF NOT EXISTS service_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		token TEXT,
		token_expiration TEXT,
		is_expired BOOLEAN DEFAULT FALSE,
		risk_level TEXT,
		permissions TEXT,
		is_cluster_admin BOOLEAN DEFAULT FALSE,
		security_flags TEXT,
		pods TEXT,
		collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		kubelet_ip TEXT,
		UNIQUE(name, namespace)
	);

	CREATE INDEX IF NOT EXISTS idx_sa_namespace ON service_accounts(namespace);
	CREATE INDEX IF NOT EXISTS idx_sa_risk_level ON service_accounts(risk_level);
	CREATE INDEX IF NOT EXISTS idx_sa_is_cluster_admin ON service_accounts(is_cluster_admin);
	CREATE INDEX IF NOT EXISTS idx_sa_collected_at ON service_accounts(collected_at);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库表结构失败: %w", err)
	}

	return nil
}

// DefaultPath 返回默认数据库路径
func DefaultPath() string {
	return config.DefaultDBPath
}
