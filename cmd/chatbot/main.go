package main

import (
	"context"
	"log"

	"github.com/magical/chat"
)

func main() {
	var bot chat.Bot
	log.Fatal(bot.Serve(context.Background()))
}
