package main

import (
	"fmt"
	opus "github.com/csiwek/opus-go"
)

func main() {
	opusfile, err := opus.NewFile("music.opus")
	if err != nil {
		fmt.Printf("Could not open file %v\n", err.Error())
	}
	opusfile.GetSingleSample()
	opusfile.GetSingleSample()
	opusfile.GetSingleSample()
	opusfile.GetSingleSample()
}
