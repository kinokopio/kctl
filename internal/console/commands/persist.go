package commands

import (
	"kctl/internal/console/commands/persist"
	"kctl/internal/session"
)

// PersistCmd persist 命令
type PersistCmd struct{}

func init() {
	Register(&PersistCmd{})
}

func (c *PersistCmd) Name() string {
	return "persist"
}

func (c *PersistCmd) Aliases() []string {
	return []string{"ps"}
}

func (c *PersistCmd) Description() string {
	return "Kubernetes 持久化攻击工具"
}

func (c *PersistCmd) Mode() CommandMode {
	return ModeKubernetesOnly
}

func (c *PersistCmd) Usage() string {
	return persist.Usage()
}

func (c *PersistCmd) Execute(sess *session.Session, args []string) error {
	return persist.Execute(sess, args)
}
