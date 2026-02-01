package db

import (
	"database/sql"
	"fmt"

	"kctl/pkg/types"
)

// PodRepository Pod 数据仓库
type PodRepository struct {
	db *DB
}

// NewPodRepository 创建 Pod 仓库
func NewPodRepository(db *DB) *PodRepository {
	return &PodRepository{db: db}
}

// Save 保存单个 Pod
func (r *PodRepository) Save(record *types.PodRecord) error {
	query := `
	INSERT OR REPLACE INTO pods (
		name, namespace, uid, node_name, pod_ip, host_ip, phase,
		service_account, creation_timestamp, containers, volumes,
		security_context, collected_at, kubelet_ip
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.conn.Exec(query,
		record.Name, record.Namespace, record.UID, record.NodeName,
		record.PodIP, record.HostIP, record.Phase, record.ServiceAccount,
		record.CreationTimestamp, record.Containers, record.Volumes,
		record.SecurityContext, record.CollectedAt, record.KubeletIP,
	)

	return err
}

// SaveBatch 批量保存 Pod
func (r *PodRepository) SaveBatch(records []*types.PodRecord) (int, error) {
	tx, err := r.db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("开始事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO pods (
			name, namespace, uid, node_name, pod_ip, host_ip, phase,
			service_account, creation_timestamp, containers, volumes,
			security_context, collected_at, kubelet_ip
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("准备语句失败: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	saved := 0
	for _, record := range records {
		_, err := stmt.Exec(
			record.Name, record.Namespace, record.UID, record.NodeName,
			record.PodIP, record.HostIP, record.Phase, record.ServiceAccount,
			record.CreationTimestamp, record.Containers, record.Volumes,
			record.SecurityContext, record.CollectedAt, record.KubeletIP,
		)
		if err != nil {
			return saved, fmt.Errorf("保存 Pod %s/%s 失败: %w", record.Namespace, record.Name, err)
		}
		saved++
	}

	if err := tx.Commit(); err != nil {
		return saved, fmt.Errorf("提交事务失败: %w", err)
	}

	return saved, nil
}

// GetAll 获取所有 Pod
func (r *PodRepository) GetAll() ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods ORDER BY collected_at DESC
	`)
}

// GetByNamespace 按命名空间获取
func (r *PodRepository) GetByNamespace(namespace string) ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods WHERE namespace = ? ORDER BY name
	`, namespace)
}

// GetByServiceAccount 按 ServiceAccount 获取
func (r *PodRepository) GetByServiceAccount(sa string) ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods WHERE service_account = ? ORDER BY namespace, name
	`, sa)
}

// GetPrivileged 获取特权 Pod
func (r *PodRepository) GetPrivileged() ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods 
		WHERE containers LIKE '%"privileged":true%'
		   OR containers LIKE '%"allowPrivilegeEscalation":true%'
		ORDER BY namespace, name
	`)
}

// GetWithSecrets 获取挂载 Secret 的 Pod
func (r *PodRepository) GetWithSecrets() ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods 
		WHERE volumes LIKE '%"type":"secret"%'
		ORDER BY namespace, name
	`)
}

// GetWithHostPath 获取挂载 HostPath 的 Pod
func (r *PodRepository) GetWithHostPath() ([]*types.PodRecord, error) {
	return r.query(`
		SELECT id, name, namespace, uid, node_name, pod_ip, host_ip, phase,
			   service_account, creation_timestamp, containers, volumes,
			   security_context, collected_at, kubelet_ip
		FROM pods 
		WHERE volumes LIKE '%"type":"hostPath"%'
		ORDER BY namespace, name
	`)
}

// Count 获取总数
func (r *PodRepository) Count() (int, error) {
	var count int
	err := r.db.conn.QueryRow("SELECT COUNT(*) FROM pods").Scan(&count)
	return count, err
}

// GetNamespaces 获取所有命名空间
func (r *PodRepository) GetNamespaces() ([]string, error) {
	rows, err := r.db.conn.Query("SELECT DISTINCT namespace FROM pods ORDER BY namespace")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var namespaces []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// GetServiceAccounts 获取所有 ServiceAccount
func (r *PodRepository) GetServiceAccounts() ([]string, error) {
	rows, err := r.db.conn.Query("SELECT DISTINCT service_account FROM pods WHERE service_account != '' ORDER BY service_account")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sas []string
	for rows.Next() {
		var sa string
		if err := rows.Scan(&sa); err != nil {
			return nil, err
		}
		sas = append(sas, sa)
	}

	return sas, nil
}

// Clear 清空所有记录
func (r *PodRepository) Clear() error {
	_, err := r.db.conn.Exec("DELETE FROM pods")
	return err
}

// query 通用查询方法
func (r *PodRepository) query(sql string, args ...interface{}) ([]*types.PodRecord, error) {
	rows, err := r.db.conn.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanPodRows(rows)
}

// scanPodRows 扫描行
func scanPodRows(rows *sql.Rows) ([]*types.PodRecord, error) {
	var pods []*types.PodRecord
	for rows.Next() {
		var pod types.PodRecord
		err := rows.Scan(
			&pod.ID, &pod.Name, &pod.Namespace, &pod.UID,
			&pod.NodeName, &pod.PodIP, &pod.HostIP, &pod.Phase,
			&pod.ServiceAccount, &pod.CreationTimestamp,
			&pod.Containers, &pod.Volumes, &pod.SecurityContext,
			&pod.CollectedAt, &pod.KubeletIP,
		)
		if err != nil {
			return nil, err
		}
		pods = append(pods, &pod)
	}
	return pods, nil
}
