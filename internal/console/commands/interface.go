package commands

import (
	"kctl/internal/session"
)

// Command 命令接口
type Command interface {
	// Name 返回命令名
	Name() string

	// Aliases 返回命令别名
	Aliases() []string

	// Description 返回简短描述
	Description() string

	// Usage 返回使用说明
	Usage() string

	// Execute 执行命令
	Execute(sess *session.Session, args []string) error
}

// 命令注册表
var registry = make(map[string]Command)

// Register 注册命令
func Register(cmd Command) {
	registry[cmd.Name()] = cmd
	for _, alias := range cmd.Aliases() {
		registry[alias] = cmd
	}
}

// Get 获取命令
func Get(name string) (Command, bool) {
	cmd, ok := registry[name]
	return cmd, ok
}

// All 获取所有命令（去重）
func All() []Command {
	seen := make(map[string]bool)
	var cmds []Command

	for _, cmd := range registry {
		if !seen[cmd.Name()] {
			seen[cmd.Name()] = true
			cmds = append(cmds, cmd)
		}
	}

	return cmds
}

// Names 获取所有命令名（包括别名）
func Names() []string {
	var names []string
	for name := range registry {
		names = append(names, name)
	}
	return names
}
