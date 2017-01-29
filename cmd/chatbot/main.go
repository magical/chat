package main

import (
	"log"

	"github.com/magical/chat"
	"github.com/magical/chat/apples"
)

func main() {
	bot, err := chat.NewBot()
	if err != nil {
		log.Fatal(err)
	}
	bot.Handle(&apples.Game{})
	//bot.Handle(chat.HandlerFunc(func(b *chat.Bot, m *chat.Message) {
	//	b.Respond(m, "hi")
	//}))
	bot.Join("ircs://irc.veekun.com/magical")
	log.Fatal(bot.Serve())
}
