package db

import (
	"database/sql"
	"fmt"

	"kctl/pkg/types"
)

// ServiceAccountRepository ServiceAccount 数据仓库
type ServiceAccountRepository struct {
	db *DB
}

// NewServiceAccountRepository 创建 ServiceAccount 仓库
func NewServiceAccountRepository(db *DB) *ServiceAccountRepository {
	return &ServiceAccountRepository{db: db}
}

// Save 保存单个 ServiceAccount
func (r *ServiceAccountRepository) Save(record *types.ServiceAccountRecord) error {
	query := `
	INSERT OR REPLACE INTO service_accounts (
		name, namespace, token, token_expiration, is_expired,
		risk_level, permissions, is_cluster_admin, security_flags,
		pods, collected_at, kubelet_ip
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.conn.Exec(query,
		record.Name, record.Namespace, record.Token,
		record.TokenExpiration, record.IsExpired,
		record.RiskLevel, record.Permissions, record.IsClusterAdmin,
		record.SecurityFlags, record.Pods,
		record.CollectedAt, record.KubeletIP,
	)

	return err
}

// SaveBatch 批量保存 ServiceAccount
func (r *ServiceAccountRepository) SaveBatch(records []*types.ServiceAccountRecord) (int, error) {
	tx, err := r.db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("开始事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO service_accounts (
			name, namespace, token, token_expiration, is_expired,
			risk_level, permissions, is_cluster_admin, security_flags,
			pods, collected_at, kubelet_ip
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("准备语句失败: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	saved := 0
	for _, record := range records {
		_, err := stmt.Exec(
			record.Name, record.Namespace, record.Token,
			record.TokenExpiration, record.IsExpired,
			record.RiskLevel, record.Permissions, record.IsClusterAdmin,
			record.SecurityFlags, record.Pods,
			record.CollectedAt, record.KubeletIP,
		)
		if err != nil {
			return saved, fmt.Errorf("保存 SA %s/%s 失败: %w", record.Namespace, record.Name, err)
		}
		saved++
	}

	if err := tx.Commit(); err != nil {
		return saved, fmt.Errorf("提交事务失败: %w", err)
	}

	return saved, nil
}

// GetAll 获取所有 ServiceAccount
func (r *ServiceAccountRepository) GetAll() ([]*types.ServiceAccountRecord, error) {
	return r.query(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts ORDER BY 
			CASE risk_level 
				WHEN 'ADMIN' THEN 0
				WHEN 'CRITICAL' THEN 1 
				WHEN 'HIGH' THEN 2 
				WHEN 'MEDIUM' THEN 3 
				WHEN 'LOW' THEN 4 
				ELSE 5 
			END, namespace, name
	`)
}

// GetByRiskLevel 按风险等级获取
func (r *ServiceAccountRepository) GetByRiskLevel(riskLevel string) ([]*types.ServiceAccountRecord, error) {
	return r.query(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts WHERE risk_level = ? ORDER BY namespace, name
	`, riskLevel)
}

// GetClusterAdmins 获取集群管理员级别的 ServiceAccount
func (r *ServiceAccountRepository) GetClusterAdmins() ([]*types.ServiceAccountRecord, error) {
	return r.query(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts WHERE is_cluster_admin = TRUE ORDER BY namespace, name
	`)
}

// GetRisky 获取有风险的 ServiceAccount (CRITICAL, HIGH, MEDIUM, ADMIN)
func (r *ServiceAccountRepository) GetRisky() ([]*types.ServiceAccountRecord, error) {
	return r.query(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts 
		WHERE risk_level IN ('ADMIN', 'CRITICAL', 'HIGH', 'MEDIUM')
		ORDER BY 
			CASE risk_level 
				WHEN 'ADMIN' THEN 0
				WHEN 'CRITICAL' THEN 1 
				WHEN 'HIGH' THEN 2 
				WHEN 'MEDIUM' THEN 3 
				ELSE 4 
			END, namespace, name
	`)
}

// GetByName 按名称和命名空间获取
func (r *ServiceAccountRepository) GetByName(namespace, name string) (*types.ServiceAccountRecord, error) {
	row := r.db.conn.QueryRow(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts WHERE namespace = ? AND name = ?
	`, namespace, name)

	var sa types.ServiceAccountRecord
	err := row.Scan(
		&sa.ID, &sa.Name, &sa.Namespace, &sa.Token,
		&sa.TokenExpiration, &sa.IsExpired,
		&sa.RiskLevel, &sa.Permissions, &sa.IsClusterAdmin,
		&sa.SecurityFlags, &sa.Pods,
		&sa.CollectedAt, &sa.KubeletIP,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &sa, nil
}

// GetByNamespace 按命名空间获取
func (r *ServiceAccountRepository) GetByNamespace(namespace string) ([]*types.ServiceAccountRecord, error) {
	return r.query(`
		SELECT id, name, namespace, token, token_expiration, is_expired,
			   risk_level, permissions, is_cluster_admin, security_flags,
			   pods, collected_at, kubelet_ip
		FROM service_accounts WHERE namespace = ? ORDER BY name
	`, namespace)
}

// Count 获取总数
func (r *ServiceAccountRepository) Count() (int, error) {
	var count int
	err := r.db.conn.QueryRow("SELECT COUNT(*) FROM service_accounts").Scan(&count)
	return count, err
}

// GetStats 获取统计信息
func (r *ServiceAccountRepository) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := r.db.conn.Query(`
		SELECT risk_level, COUNT(*) as count 
		FROM service_accounts 
		GROUP BY risk_level
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		stats[level] = count
	}

	// 获取集群管理员数量
	var adminCount int
	err = r.db.conn.QueryRow("SELECT COUNT(*) FROM service_accounts WHERE is_cluster_admin = TRUE").Scan(&adminCount)
	if err != nil {
		return nil, err
	}
	stats["ADMIN"] = adminCount

	return stats, nil
}

// Clear 清空所有记录
func (r *ServiceAccountRepository) Clear() error {
	_, err := r.db.conn.Exec("DELETE FROM service_accounts")
	return err
}

// query 通用查询方法
func (r *ServiceAccountRepository) query(sql string, args ...interface{}) ([]*types.ServiceAccountRecord, error) {
	rows, err := r.db.conn.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanSARows(rows)
}

// scanSARows 扫描行
func scanSARows(rows *sql.Rows) ([]*types.ServiceAccountRecord, error) {
	var sas []*types.ServiceAccountRecord
	for rows.Next() {
		var sa types.ServiceAccountRecord
		err := rows.Scan(
			&sa.ID, &sa.Name, &sa.Namespace, &sa.Token,
			&sa.TokenExpiration, &sa.IsExpired,
			&sa.RiskLevel, &sa.Permissions, &sa.IsClusterAdmin,
			&sa.SecurityFlags, &sa.Pods,
			&sa.CollectedAt, &sa.KubeletIP,
		)
		if err != nil {
			return nil, err
		}
		sas = append(sas, &sa)
	}
	return sas, nil
}
