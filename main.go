package main

import (
"github.com/gin-gonic/gin"
"github.com/gin-contrib/gzip"
"github.com/gin-contrib/static"
"github.com/unrolled/secure"
"log"
"net/http"
"os"
)

func main(){
	router := gin.Default()
	router.Use(func() gin.HandlerFunc {
                return func(c *gin.Context) {
                        err := secure.New(secure.Options {
                                SSLRedirect: true,
                        }).Process(c.Writer, c.Request)
                        if err != nil {
                                log.Println(err)
                                return
                        }
                        c.Next()
                }
        }())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(static.Serve("/", static.LocalFile("./static", true)))

	log.Fatal(http.ListenAndServe(":" + os.Getenv("PORT"), router))
}
