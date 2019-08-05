package main

import (
"github.com/gin-gonic/gin"
"github.com/gin-contrib/gzip"
"github.com/gin-contrib/static"
"log"
"net/http"
"os"
)

func main(){
	router := gin.Default()
	router.Use(func() gin.HandlerFunc {
                return func(c *gin.Context) {
			if os.Getenv("GIN_MODE") == "release" {
				if c.Request.Header.Get("x-forwarded-proto") != "https" {
					c.Redirect(http.StatusMovedPermanently, "https://www.pathfindersrobotics" + c.Request.URL.Path)
				}
			}
                }
        }())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(static.Serve("/", static.LocalFile("./static", true)))

	log.Fatal(http.ListenAndServe(":" + os.Getenv("PORT"), router))
}
