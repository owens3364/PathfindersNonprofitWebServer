package main

import "github.com/gin-gonic/gin";
import "github.com/gin-contrib/static";

func main() {
	router := gin.Default()
	router.Use(static.Serve("/", static.LocalFile("./static", true)))
	router.Run(":443")
}
