package main

import (
"github.com/gin-gonic/gin"
"github.com/gin-contrib/gzip"
"github.com/gin-contrib/static"
"log"
"net/http"
"os"
)

func main() {
	//httpsRedirectRouter := gin.New()
	//httpsRedirectRouter.GET("/*anything", func(c *gin.Context) {
	//	c.Redirect(301, "https://www.pathfindersrobotics.org/" + c.Param("anything"))
	//})
	//go log.Fatal(http.ListenAndServe(":" + os.Getenv("HTTP_PORT"), httpsRedirectRouter))
	//go httpsRedirectRouter.Run(":" + os.Getenv("HTTP_PORT"))

	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(static.Serve("/", static.LocalFile("./static", true)))

	log.Fatal(http.ListenAndServe(":" + os.Getenv("HTTPS_PORT"), router))
}
