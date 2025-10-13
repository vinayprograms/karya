package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	var locale, region, year string

	switch len(os.Args) {
	case 1:
		locale = getLocale()
		region = ""
		year = strconv.Itoa(time.Now().Year())
	case 2:
		locale = getLocale()
		region = ""
		year = os.Args[1]
	case 4:
		locale = strings.ReplaceAll(os.Args[1], "_", "-")
		region = os.Args[2]
		year = os.Args[3]
	default:
		fmt.Println("Usage:")
		fmt.Println("'holiday' - Use host's locale, no region and current year")
		fmt.Println("'holiday <year>' - Use host's locale, no region and specific year")
		fmt.Println("'holiday <locale> <region> <year>' - Use provided params. Nothing is assumed.")
		fmt.Println()
		fmt.Println("All information is sourced from https://holidata.net/")
		os.Exit(1)
	}

	url := fmt.Sprintf("https://holidata.net/%s/%s.csv", locale, year)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	// Skip header
	_, err = reader.Read()
	if err != nil {
		log.Fatal(err)
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		regn := record[1]
		dt := strings.ReplaceAll(record[2], "\"", "")
		dt = strings.ReplaceAll(dt, "-", "_")
		day := strings.ReplaceAll(record[3], "\"", "")
		if regn == "\""+region+"\"" || regn == "\"\"" {
			fmt.Printf("%s = %s\n", dt, day)
		}
	}
}

func getLocale() string {
	// Simple, assume en-US or something
	return "en-US"
}