package console

import (
	"fmt"

	"kctl/config"
	"kctl/internal/session"
)

// Banner ASCII Art
const banner = `
  ██╗  ██╗ ██████╗████████╗██╗     
  ██║ ██╔╝██╔════╝╚══██╔══╝██║     
  █████╔╝ ██║        ██║   ██║     
  ██╔═██╗ ██║        ██║   ██║     
  ██║  ██╗╚██████╗   ██║   ███████╗
  ╚═╝  ╚═╝ ╚═════╝   ╚═╝   ╚══════╝
`

// Version 版本号
const Version = "v1.0.0"

// PrintBanner 打印 Banner
func PrintBanner(s *session.Session) {
	p := s.Printer

	// 打印 ASCII Art
	p.PrintColored(config.ColorCyan, banner)
	p.Println()

	// 打印版本和信息
	p.Printf("  %s\n",
		p.Colored(config.ColorWhite, "Kubernetes Security Audit Tool"))
	p.Println()

	// 打印运行模式
	mode := s.GetMode()
	modeDisplay := "Local"
	if s.InPod {
		modeDisplay = "In-Pod"
	}
	p.Printf("  %s Mode: %s (%s)\n",
		p.Colored(config.ColorBlue, "[*]"),
		p.Colored(config.ColorGreen, string(mode)),
		p.Colored(config.ColorGray, modeDisplay))

	// 根据模式打印目标信息
	if mode == session.ModeKubelet {
		if s.Config.KubeletIP != "" {
			targetInfo := fmt.Sprintf("%s:%d", s.Config.KubeletIP, s.Config.KubeletPort)
			note := ""
			if s.InPod {
				note = " (auto-detected)"
			}
			p.Printf("  %s Kubelet: %s%s\n",
				p.Colored(config.ColorBlue, "[*]"),
				p.Colored(config.ColorYellow, targetInfo),
				p.Colored(config.ColorGray, note))
		} else {
			p.Printf("  %s Kubelet: %s\n",
				p.Colored(config.ColorBlue, "[*]"),
				p.Colored(config.ColorGray, "(not set, use 'set target <ip>')"))
		}
	} else {
		if s.Config.APIServer != "" {
			targetInfo := fmt.Sprintf("%s:%d", s.Config.APIServer, s.Config.APIServerPort)
			p.Printf("  %s API Server: %s\n",
				p.Colored(config.ColorBlue, "[*]"),
				p.Colored(config.ColorYellow, targetInfo))
		} else {
			p.Printf("  %s API Server: %s\n",
				p.Colored(config.ColorBlue, "[*]"),
				p.Colored(config.ColorGray, "(not set, use 'set api-server <ip>')"))
		}
	}

	// 打印帮助提示
	p.Printf("  %s Type '%s' for available commands\n",
		p.Colored(config.ColorBlue, "[*]"),
		p.Colored(config.ColorGreen, "help"))
	p.Println()
}
