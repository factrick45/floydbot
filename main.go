package main

import (
	"fmt"
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

	if args, match := ComParse(m.Content, "!4post"); match {
		s.ChannelTyping(m.ChannelID)

		post, err := scrapers.FourPostNewest(args[1])
		if err != nil {
			log.Println("error scraping 4chan:", err)
			return
		}
		str := fmt.Sprintf(
			"**%s %s [No.%d](<%s>)**\n%s", post.PosterName, post.DateTime,
			post.PostNumber, post.Link, post.Message)
		_, err = s.ChannelMessageSendReply(
			m.ChannelID, str, (*m).Reference())
		if err != nil {
			log.Println("error sending reply:", err)
		}

		return
	}

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
		return
	}

	if m.Content == "!ghumor" {
		s.ChannelTyping(m.ChannelID)

		str, video, err := scrapers.DesuScrape("g", "humor")
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
		return
	}

	if m.Content == "!ping" {
		s.ChannelTyping(m.ChannelID)
		_, err := s.ChannelMessageSendReply(
			m.ChannelID, "Pong from 🇰🇿", (*m).Reference())
		if err != nil {
			log.Println("error sending reply:", err)
		}
		return
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
