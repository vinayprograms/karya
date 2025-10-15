package main

import (
	"fmt"
	"log"
	"os"

	"karya/internal/zet"
)

func main() {
	config, err := zet.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 1 {
		// Interactive mode
		if err := zet.InteractiveMode(config); err != nil {
			log.Fatal(err)
		}
		return
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "count":
		count, err := zet.CountZettels(config)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(count)
	case "n", "new", "a", "add":
		if len(args) == 0 {
			if err := zet.NewZettel(config, ""); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := zet.NewZettel(config, args[0]); err != nil {
				log.Fatal(err)
			}
		}
	case "e", "edit":
		if len(args) == 0 {
			log.Fatal("Zettel ID required")
		}
		if err := zet.EditZettel(config, args[0]); err != nil {
			log.Fatal(err)
		}
	case "ls", "list":
		if err := zet.ListZettels(config, ""); err != nil {
			log.Fatal(err)
		}
	case "show":
		if len(args) == 0 {
			log.Fatal("Zettel ID required")
		}
		if err := zet.ShowZettel(config, args[0]); err != nil {
			log.Fatal(err)
		}
	case "last":
		if err := zet.EditLastZettel(config); err != nil {
			log.Fatal(err)
		}
	case "toc":
		if err := zet.EditTOC(config); err != nil {
			log.Fatal(err)
		}
	case "?":
		if len(args) == 0 {
			log.Fatal("Search query required")
		}
		if err := zet.SearchZettels(config, args); err != nil {
			log.Fatal(err)
		}
	case "t?":
		if len(args) == 0 {
			log.Fatal("Search query required")
		}
		if err := zet.SearchTitles(config, args); err != nil {
			log.Fatal(err)
		}
	case "d", "todo":
		if err := zet.SearchTodos(config, args); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("Unknown subcommand:", subcommand)
	}
}