package main

import (
	"log"

	"github.com/magical/chat"
)

func main() {
	bot, err := chat.NewBot()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(bot.Serve())
}
