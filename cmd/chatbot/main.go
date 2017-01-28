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
	bot.Handle(bot.HandlerFunc(func(b *chat.Bot, m *chat.Message) {
		b.Respond(m, "hi")
	}))
	log.Fatal(bot.Serve())
}
