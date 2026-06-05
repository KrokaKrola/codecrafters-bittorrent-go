package main

import (
	"encoding/json"
	"fmt"
	"os"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Invalid number of arguments")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "info":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		data, err := os.ReadFile(fileName)
		if err != nil {
			fmt.Println("Error while trying to read file:" + err.Error())
			os.Exit(1)
		}

		dec := decoder{
			data: data,
		}

		var dict dictionary

		if res, ok := dec.decode().(dictionary); ok {
			dict = res
		} else {
			fmt.Println("Invalid file content")
			os.Exit(1)
		}

		announce, ok := findElementInDictionary[string](dict, "announce")
		if !ok {
			fmt.Println("Invalid file content, announce field was not found")
			os.Exit(1)
		}

		info, ok := findElementInDictionary[dictionary](dict, "info")
		if !ok {
			fmt.Println("Invalid file content, info field was not found")
			os.Exit(1)
		}

		length, ok := findElementInDictionary[int](info, "length")
		if !ok {
			fmt.Println("Invalid file content, info.length field was not found")
			os.Exit(1)
		}

		encodedInfo, err := encode(info)
		if err != nil {
			fmt.Println("Error while trying to encode info content")
			os.Exit(1)
		}

		hash, err := createSha1Hash(encodedInfo)
		if err != nil {
			fmt.Println("Error while trying to create hash from info content")
			os.Exit(1)
		}

		fmt.Println("Tracker URL:", announce)
		fmt.Println("Length:", length)
		fmt.Println("Info Hash:", hash)
	case "decode":
		bencodedValue := os.Args[2]

		dec := decoder{
			data: []byte(bencodedValue),
		}

		decoded := dec.decode()

		if dec.err != nil {
			fmt.Println(dec.err)
			return
		}

		if _, ok := decoded.(dictionary); ok {
			decoded = decoded.(dictionary).toMap()
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
