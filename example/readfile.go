package main

import (
	"fmt"
	opus "github.com/csiwek/opus-go"
)

func main() {
	_, err := opus.NewFile("output.opus")
	if err != nil {
		fmt.Printf("Could not open file\n")
	}

}
