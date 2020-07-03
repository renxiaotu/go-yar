package yar

import "fmt"

func Server(network, address string) bool {
	fmt.Println(network, address)
	fmt.Println(1)
	return true
}
