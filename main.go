package main

import (
	"github.com/bwmarrin/discordgo"
	"ihm/floydbot/scrapers"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var botstate struct {
	sync.Mutex
	Busy bool
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	botstate.Lock()
	if botstate.Busy || m.Author.ID == s.State.User.ID {
		botstate.Unlock()
		return
	}
	botstate.Busy = true
	botstate.Unlock()

	// Turn off busy flag once we're done
	defer func() {
		botstate.Lock()
		botstate.Busy = false
		botstate.Unlock()
	}()

	if m.Content == "!caturday" {
		// Typing stops when message is sent
		s.ChannelTyping(m.ChannelID)

		str, video, err := scrapers.DesuScrape("an", "kot")
		if err != nil {
			log.Println("error scraping Desuarchive:", err)
		}

		me := discordgo.MessageEmbed{}

		if !video {
			mei := discordgo.MessageEmbedImage{URL: str}
			me.Type = discordgo.EmbedTypeImage
			me.Image = &mei
		} else {
			mev := discordgo.MessageEmbedVideo{URL: str}
			me.Type = discordgo.EmbedTypeVideo
			me.Video = &mev
		}

		_, err = s.ChannelMessageSendEmbedReply(
			m.ChannelID, &me, (*m).Reference())
		if err != nil {
			log.Println("error sending embed reply:", err)
		}
	}

	if m.Content == "!ping" {
		s.ChannelTyping(m.ChannelID)
		_, err := s.ChannelMessageSendReply(
			m.ChannelID, "Pong from 🇰🇿", (*m).Reference())
		if err != nil {
			log.Println("error sending reply:", err)
		}
	}
}

func main() {
	dg, err := discordgo.New(BOT_TOKEN)
	if err != nil {
		log.Println("error creating Discord session:", err)
		return
	}
	dg.AddHandler(messageCreate)
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Println("error opening connection:", err)
		return
	}

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}
