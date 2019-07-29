package main

import (
	"fmt"
	opus "github.com/csiwek/opus-go"
)

func main() {
	opusfile, err := opus.NewFile("ff-16b-2c-44100hz.opus")
	if err != nil {
		fmt.Printf("Could not open file %v\n", err.Error())
	}
	for i := 0; i < 100; i++ {
		_, err := opusfile.GetSingleSample()
		if err != nil {
			fmt.Printf("GetSingleSample returned Errr %v\n", err.Error())
		}
	}
}
