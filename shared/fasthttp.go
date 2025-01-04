// shared/httpclient.go
package shared

import (
	"github.com/valyala/fasthttp"
)

var HttpClient = &fasthttp.Client{
    MaxConnsPerHost: 50,  // 设置最大连接数
}