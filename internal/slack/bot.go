package slack

import (
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/slack-go/slack/slackevents"

	"github.disney.com/SANCR225/koda/internal/acp"
)

// Bot runs the Steery Slack bot.
type Bot struct {
	api       *slack.Client
	socket    *socketmode.Client
	agent     string
	client    *acp.Client
	botUserID string
}

// New creates a new Slack bot.
func New(botToken, appToken, agent string) (*Bot, error) {
	api := slack.New(botToken,
		slack.OptionAppLevelToken(appToken),
	)

	socket := socketmode.New(api,
		socketmode.OptionLog(log.New(log.Writer(), "slack: ", log.LstdFlags)),
	)

	// Get bot user ID for mention detection
	auth, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack auth failed: %w", err)
	}

	return &Bot{
		api:       api,
		socket:    socket,
		agent:     agent,
		botUserID: auth.UserID,
	}, nil
}

// Run starts the bot and blocks.
func (b *Bot) Run() error {
	// Spawn kiro-cli
	fmt.Printf("\U0001f916 Starting Steery with agent: %s\n", b.agent)
	client, err := acp.Spawn(b.agent)
	if err != nil {
		return fmt.Errorf("failed to start kiro-cli: %w", err)
	}
	defer client.Close()

	if err := client.CreateSession(b.agent); err != nil {
		return fmt.Errorf("session failed: %w", err)
	}
	b.client = client

	fmt.Println("\u2705 Connected to Slack. Listening for messages...")

	go b.handleEvents()
	return b.socket.Run()
}

func (b *Bot) handleEvents() {
	for evt := range b.socket.Events {
		switch evt.Type {
		case socketmode.EventTypeEventsAPI:
			eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				continue
			}
			b.socket.Ack(*evt.Request)

			switch ev := eventsAPI.InnerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				go b.handleMention(ev)
			case *slackevents.MessageEvent:
				// Handle DMs
				if ev.ChannelType == "im" && ev.User != b.botUserID {
					go b.handleDM(ev)
				}
			}
		case socketmode.EventTypeConnecting:
			fmt.Println("\U0001f504 Connecting to Slack...")
		case socketmode.EventTypeConnected:
			fmt.Println("\u2705 Connected to Slack")
		}
	}
}

func (b *Bot) handleMention(ev *slackevents.AppMentionEvent) {
	// Strip the @mention from the message
	text := strings.TrimSpace(strings.Replace(ev.Text, fmt.Sprintf("<@%s>", b.botUserID), "", 1))
	if text == "" {
		text = "hello"
	}

	// Reply in thread
	threadTS := ev.TimeStamp
	if ev.ThreadTimeStamp != "" {
		threadTS = ev.ThreadTimeStamp
	}

	b.respond(ev.Channel, threadTS, text)
}

func (b *Bot) handleDM(ev *slackevents.MessageEvent) {
	if ev.Text == "" || ev.BotID != "" {
		return
	}
	b.respond(ev.Channel, ev.TimeStamp, ev.Text)
}

func (b *Bot) respond(channel, threadTS, text string) {
	// Send typing indicator
	b.api.PostMessage(channel,
		slack.MsgOptionText("\u2699 Thinking...", false),
		slack.MsgOptionTS(threadTS),
	)

	// Send to kiro-cli
	if err := b.client.SendMessage(text); err != nil {
		b.api.PostMessage(channel,
			slack.MsgOptionText(fmt.Sprintf("\u26a0 Error: %v", err), false),
			slack.MsgOptionTS(threadTS),
		)
		return
	}

	// Collect streaming response
	var response strings.Builder
	for event := range b.client.Events {
		switch event.Type {
		case "MessageChunk":
			response.WriteString(event.Chunk)
		case "ToolCall":
			b.api.PostMessage(channel,
				slack.MsgOptionText(fmt.Sprintf("\u2699 %s...", event.Name), false),
				slack.MsgOptionTS(threadTS),
			)
		case "Complete":
			goto done
		}
	}
done:

	// Post final response
	if response.Len() > 0 {
		// Slack has a 4000 char limit per message
		text := response.String()
		for len(text) > 0 {
			chunk := text
			if len(chunk) > 3900 {
				// Split at last newline before limit
				idx := strings.LastIndex(chunk[:3900], "\n")
				if idx > 0 {
					chunk = chunk[:idx]
				} else {
					chunk = chunk[:3900]
				}
			}
			b.api.PostMessage(channel,
				slack.MsgOptionText(chunk, false),
				slack.MsgOptionTS(threadTS),
			)
			text = text[len(chunk):]
		}
	}
}
