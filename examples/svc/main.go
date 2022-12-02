package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	_ "go.uber.org/automaxprocs"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var (
	version = "unknown"
)

func init() {
	if v, ok := os.LookupEnv("VERSION"); ok {
		version = v
	}
}

func main() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "当前服务版本是 %s", version)
	})

	router.GET("/rand-error", func(c *gin.Context) {
		rand.Seed(time.Now().UnixNano())
		if rand.Intn(2) == 1 {
			c.String(http.StatusInternalServerError, "内部错误")
			return
		}
		c.String(http.StatusOK, "当前服务版本是 %s", version)
	})

	router.GET("/error", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "内部错误")
	})

	router.GET("/request-svc-a", func(c *gin.Context) {
		resp, err := resty.New().R().Get("http://svc-a:8080")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(resp.StatusCode(), "当前服务版本是 %s \nsvc-a 服务\n返回状态码:%s\n返回内容: %s \n ", version, resp.Status(), resp.String())
	})

	router.GET("/request-svc-a/rand-error", func(c *gin.Context) {
		resp, err := resty.New().R().Get("http://svc-a:8080/rand-error")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "当前服务版本是 %s \nsvc-a 服务\n返回状态码:%s\n返回内容: %s \n ", version, resp.Status(), resp.String())
	})

	router.GET("/request-svc-a/error", func(c *gin.Context) {
		resp, err := resty.New().R().Get("http://svc-a:8080/error")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "当前服务版本是 %s \nsvc-a 服务\n返回状态码:%s\n返回内容: %s \n ", version, resp.Status(), resp.String())
	})
	router.Run(":8080")
}
