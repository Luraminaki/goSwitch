package main

import (
	"os"

	webapp "goSwitch/modules/webapp"
)

func main() {
	wx := webapp.NewWebApp()

	wx.Server.POST("/toggleButton", wx.ToggleButton)
	wx.Server.Logger.Fatal(wx.Server.Start(":10000"))

	os.Exit(0)
}
