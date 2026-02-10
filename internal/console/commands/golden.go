package commands

import (
	"kctl/internal/console/commands/golden"
	"kctl/internal/session"
)

// GoldenCmd golden 命令
type GoldenCmd struct{}

func init() {
	Register(&GoldenCmd{})
}

func (c *GoldenCmd) Name() string {
	return "golden"
}

func (c *GoldenCmd) Aliases() []string {
	return []string{"gt"}
}

func (c *GoldenCmd) Description() string {
	return "Kubernetes Golden Ticket - 证书和 Token 伪造"
}

func (c *GoldenCmd) Mode() CommandMode {
	return ModeKubernetesOnly
}

func (c *GoldenCmd) Usage() string {
	return golden.Usage()
}

func (c *GoldenCmd) Execute(sess *session.Session, args []string) error {
	return golden.Execute(sess, args)
}
