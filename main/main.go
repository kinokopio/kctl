package main

import (
	"kctl/cmd"
	_ "kctl/cmd/console" // console 命令
	_ "kctl/cmd/version" // import sub command as module
)

func init() {
}

func main() {
	cmd.Execute()
}
