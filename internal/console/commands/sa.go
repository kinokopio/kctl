package commands

import (
	"kctl/internal/console/commands/sa"
	"kctl/internal/session"
)

// SACmd sa 命令
type SACmd struct{}

func init() {
	Register(&SACmd{})
}

func (c *SACmd) Name() string {
	return "sa"
}

func (c *SACmd) Aliases() []string {
	return []string{"serviceaccounts"}
}

func (c *SACmd) Description() string {
	return "ServiceAccount 相关操作"
}

func (c *SACmd) Usage() string {
	return sa.Usage()
}

func (c *SACmd) Execute(sess *session.Session, args []string) error {
	return sa.Execute(sess, args)
}
