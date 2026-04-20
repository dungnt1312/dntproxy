package main

import (
	"fmt"
	"strings"
)

func main() {
	s := "glm/glm-5.1@abc"
	idx := strings.Index(s, "/")
	fmt.Println("Input:", s)
	fmt.Println("Provider:", s[:idx])
	fmt.Println("Rest:", s[idx+1:])
	fmt.Println("Rebuild:", s[:idx]+"/"+s[idx+1:])
}
