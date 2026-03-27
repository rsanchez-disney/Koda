package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
	kodaSlack "github.disney.com/SANCR225/koda/internal/slack"
)

var (
	slackAgent    string
	slackBotToken string
	slackAppToken string
)

var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "Run Steery Slack bot",
	Long:  `Run the Steery support bot in Slack. Connects to kiro-cli locally and responds to @mentions and DMs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve tokens: flag > env > settings
		botToken := slackBotToken
		if botToken == "" {
			botToken = os.Getenv("SLACK_BOT_TOKEN")
		}
		if botToken == "" {
			s := ops.LoadSettings()
			if s.SlackBotToken != "" {
				botToken = s.SlackBotToken
			}
		}

		appToken := slackAppToken
		if appToken == "" {
			appToken = os.Getenv("SLACK_APP_TOKEN")
		}
		if appToken == "" {
			s := ops.LoadSettings()
			if s.SlackAppToken != "" {
				appToken = s.SlackAppToken
			}
		}

		if botToken == "" || appToken == "" {
			return fmt.Errorf("Slack tokens required. Set via:\n\n" +
				"  koda slack --bot-token xoxb-... --app-token xapp-...\n\n" +
				"Or environment variables:\n\n" +
				"  export SLACK_BOT_TOKEN=xoxb-...\n" +
				"  export SLACK_APP_TOKEN=xapp-...\n")
		}

		agent := slackAgent
		if agent == "" {
			agent = "steery_agent"
		}

		PrintBanner(appVersion)
		fmt.Printf("\U0001f916 Steery \u2014 Slack support bot\n")
		fmt.Printf("   Agent: %s\n\n", agent)

		bot, err := kodaSlack.New(botToken, appToken, agent)
		if err != nil {
			return err
		}
		return bot.Run()
	},
}

func init() {
	slackCmd.Flags().StringVar(&slackAgent, "agent", "steery_agent", "Agent to use")
	slackCmd.Flags().StringVar(&slackBotToken, "bot-token", "", "Slack Bot Token (xoxb-...)")
	slackCmd.Flags().StringVar(&slackAppToken, "app-token", "", "Slack App-Level Token (xapp-...)")
}
