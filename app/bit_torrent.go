package main

import (
	"fmt"
	"os"
)

type bitTorrentFile struct {
	announce string
	info     dictionary
}

func (btf *bitTorrentFile) parse(fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("Error while trying to read file: %s", err.Error())
	}

	dec := decoder{
		data: data,
	}

	var dict dictionary

	if res, ok := dec.decode().(dictionary); ok {
		dict = res
	} else {
		return fmt.Errorf("Invalid file content")
	}

	announce, ok := findElementInDictionary[string](dict, "announce")
	if !ok {
		return fmt.Errorf("Invalid file content, announce field was not found")
	}

	info, ok := findElementInDictionary[dictionary](dict, "info")
	if !ok {
		return fmt.Errorf("Invalid file content, info field was not found")
	}

	btf.announce = announce
	btf.info = info

	return nil
}
