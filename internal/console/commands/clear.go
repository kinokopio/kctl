package commands

import (
	"kctl/internal/session"
)

// ClearCmd clear 命令
type ClearCmd struct{}

func init() {
	Register(&ClearCmd{})
}

func (c *ClearCmd) Name() string {
	return "clear"
}

func (c *ClearCmd) Aliases() []string {
	return nil
}

func (c *ClearCmd) Description() string {
	return "清除缓存"
}

func (c *ClearCmd) Usage() string {
	return `clear

清除内存中的缓存数据，包括：
  - Pod 缓存
  - 当前选中的 SA
  - K8s 客户端缓存

注意：这不会清除数据库中的扫描结果`
}

func (c *ClearCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	sess.ClearCache()
	p.Success("Cache cleared")

	return nil
}
