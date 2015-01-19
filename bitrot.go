package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello world")
}

// General helper functions
func check(e error) {
	if e != nil {
		panic(e)
	}
}
