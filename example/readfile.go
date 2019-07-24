package main

import "fmt"

func main() {

	_, err := NewFile("output.opus")
	if err != nil {
		fmt.Printf("Could not open file: %v", err.Error())
	}
}
