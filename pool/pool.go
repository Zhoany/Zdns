package pool

import (
	"log"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/valyala/fasthttp"
)

var (
	antsPool   *ants.Pool
	clientPool sync.Pool
)

func InitPool(antsSize int, clientSize int) {
	var err error
	antsPool, err = ants.NewPool(antsSize)
	if err != nil {
		log.Fatalf("Failed to create ants pool: %v", err)
	}

	clientPool.New = func() interface{} {
		return &fasthttp.Client{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
	}

	for i := 0; i < clientSize; i++ {
		clientPool.Put(clientPool.New())
	}
}

func SubmitToAnts(task func()) {
	if err := antsPool.Submit(task); err != nil {
		log.Printf("Failed to submit task to ants pool: %v", err)
	}
}

func GetClient() *fasthttp.Client {
	return clientPool.Get().(*fasthttp.Client)
}

func ReturnClient(client *fasthttp.Client) {
	clientPool.Put(client)
}

func Release() {
	antsPool.Release()
}
