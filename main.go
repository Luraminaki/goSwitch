package main

import (
	"os"

	webapp "goSwitch/modules/webapp"
)

func main() {
	wx := webapp.NewWebApp()

	wx.Server.POST("/reset", wx.Reset)
	wx.Server.POST("/switch", wx.Switch)
	wx.Server.GET("/revert", wx.RevertMove)
	wx.Server.GET("/", wx.InitHTMX)
	wx.Server.Logger.Fatal(wx.Server.Start(":10000"))

	os.Exit(0)
}
