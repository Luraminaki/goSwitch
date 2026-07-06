package main

import (
	webapp "goSwitch/modules/webapp"
)

func main() {
	wx := webapp.NewWebApp("./config.json")

	wx.Server.POST("/reset", wx.Reset)
	wx.Server.POST("/switch", wx.Switch)
	wx.Server.GET("/revert", wx.RevertMove)
	wx.Server.GET("/wait", wx.Wait)
	wx.Server.GET("/", wx.InitHTMX)
	wx.Server.Logger.Fatal(wx.Server.Start(":" + wx.Config.Port))
}
