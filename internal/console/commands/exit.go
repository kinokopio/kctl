package commands

import (
	"os"
	"os/exec"

	"kctl/internal/session"
)

// ExitCmd exit 命令
type ExitCmd struct{}

func init() {
	Register(&ExitCmd{})
}

func (c *ExitCmd) Name() string {
	return "exit"
}

func (c *ExitCmd) Aliases() []string {
	return []string{"quit", "q"}
}

func (c *ExitCmd) Description() string {
	return "退出控制台"
}

func (c *ExitCmd) Usage() string {
	return `exit

退出交互式控制台，清除内存中的所有数据`
}

func (c *ExitCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	p.Info("Clearing memory...")
	p.Info("Goodbye!")

	// 关闭会话
	_ = sess.Close()

	// 恢复终端设置 (go-prompt 会修改终端 raw mode)
	resetTerminal()

	// 退出程序
	os.Exit(0)

	return nil
}

// resetTerminal 重置终端设置
func resetTerminal() {
	// 使用 stty sane 恢复终端到正常状态
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}
