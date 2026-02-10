package commands

import (
	"kctl/internal/session"
)

// CommandMode 命令所属模式
type CommandMode int

const (
	// ModeAll 通用命令，所有模式下都可用
	ModeAll CommandMode = iota
	// ModeKubeletOnly 仅 Kubelet 模式可用
	ModeKubeletOnly
	// ModeKubernetesOnly 仅 Kubernetes 模式可用
	ModeKubernetesOnly
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

	// Mode 返回命令所属模式
	Mode() CommandMode

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

// GetForMode 获取命令（检查模式）
func GetForMode(name string, mode session.Mode) (Command, bool) {
	cmd, ok := registry[name]
	if !ok {
		return nil, false
	}

	// 检查命令是否在当前模式下可用
	if !IsCommandAvailable(cmd, mode) {
		return nil, false
	}

	return cmd, true
}

// IsCommandAvailable 检查命令是否在指定模式下可用
func IsCommandAvailable(cmd Command, mode session.Mode) bool {
	switch cmd.Mode() {
	case ModeAll:
		return true
	case ModeKubeletOnly:
		return mode == session.ModeKubelet
	case ModeKubernetesOnly:
		return mode == session.ModeKubernetes
	default:
		return true
	}
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

// AllForMode 获取指定模式下可用的所有命令（去重）
func AllForMode(mode session.Mode) []Command {
	seen := make(map[string]bool)
	var cmds []Command

	for _, cmd := range registry {
		if !seen[cmd.Name()] && IsCommandAvailable(cmd, mode) {
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

// NamesForMode 获取指定模式下可用的命令名（包括别名）
func NamesForMode(mode session.Mode) []string {
	var names []string
	for name, cmd := range registry {
		if IsCommandAvailable(cmd, mode) {
			names = append(names, name)
		}
	}
	return names
}
