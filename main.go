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
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(static.Serve("/", static.LocalFile("./static", true)))
	log.Fatal(http.ListenAndServe(":" + os.Getenv("PORT"), router))
}
