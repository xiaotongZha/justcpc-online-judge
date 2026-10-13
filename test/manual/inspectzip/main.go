package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

func main() {
	path := flag.String("path", "", "result ZIP path")
	flag.Parse()
	reader, err := zip.OpenReader(*path)
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	var manifest, scoreboard, resultFiles int
	for _, entry := range reader.File {
		switch entry.Name {
		case "manifest.json":
			manifest++
		case "scoreboard.csv":
			scoreboard++
		}
		if len(entry.Name) >= 12 && entry.Name[len(entry.Name)-11:] == "result.json" {
			resultFiles++
			input, err := entry.Open()
			if err != nil {
				panic(err)
			}
			data, err := io.ReadAll(input)
			input.Close()
			if err != nil {
				panic(err)
			}
			var value map[string]json.RawMessage
			if err := json.Unmarshal(data, &value); err != nil {
				panic(err)
			}
			if _, ok := value["judge_result_json"]; !ok {
				panic("result.json misses judge_result_json")
			}
		}
	}
	if manifest != 1 || scoreboard != 1 || resultFiles == 0 {
		panic("incomplete result package")
	}
	fmt.Printf("manifest=%d scoreboard=%d result_files=%d\n", manifest, scoreboard, resultFiles)
}
