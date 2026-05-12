package redis

import (
	r "github.com/redis/go-redis/v9"
)

type Client = r.Client

func NewClient(addr string) *Client {
	client := r.NewClient(&r.Options{
		Addr: addr,
	})
	return client
}

