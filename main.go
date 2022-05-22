package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nicklaw5/helix"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/twitch"
)

type Channel struct {
	Id    string
	User  string
	Title string
}

type SearchOptions struct {
	Query         string
	TitleContains string
	GameId        string
}

type DiscordMessage struct {
	Content string `json:"content"`
}

func init() {
	viper.AutomaticEnv()
	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	var fnf *fs.PathError
	if err != nil {
		if !errors.As(err, &fnf) {
			log.Fatal(err)
		}
	}
}

func getChannels(client *helix.Client, options *SearchOptions) ([]Channel, error) {
	resp, err := client.SearchChannels(&helix.SearchChannelsParams{
		LiveOnly: true,
		Channel:  options.Query,
		First:    100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search for channels")
	}
	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}
	var response []Channel
	titleContains := strings.ToLower(options.TitleContains)
	for _, v := range resp.Data.Channels {
		if options.GameId != "" && options.GameId != v.GameID {
			continue
		}
		title := strings.ToLower(v.Title)
		if titleContains != "" && !strings.Contains(title, titleContains) {
			continue
		}
		response = append(response, Channel{Id: v.ID, User: v.BroadcasterLogin, Title: v.Title})
	}
	return response, nil
}

func getToken(clientId string, clientSecret string) (string, error) {
	oauth2Config := &clientcredentials.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     twitch.Endpoint.TokenURL,
	}
	token, err := oauth2Config.Token(context.Background())
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func sendToDiscord(message string, url string) error {
	msg := DiscordMessage{
		Content: message,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal object. %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request. %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request. %w", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

func main() {
	var searchQuery string
	var gameId string
	var titleContains string
	var timeout time.Duration
	flag.StringVar(&searchQuery, "query", "", "search query")
	flag.StringVar(&gameId, "game", "", "game id to search for")
	flag.StringVar(&titleContains, "title", "", "string to find in title")
	flag.DurationVar(&timeout, "timeout", 30*time.Minute, "minutes to wait before searching again")
	flag.Parse()
	if searchQuery == "" {
		log.Fatal("missing required query parameter")
	}

	clientId := viper.GetString("TWITCH_ID")
	if clientId == "" {
		log.Fatal("missing required TWITCH_ID")
	}
	clientSecret := viper.GetString("TWITCH_SECRET")
	if clientSecret == "" {
		log.Fatal("missing required TWITCH_SECRET")
	}
	discordUrl := viper.GetString("DISCORD_WEBHOOK")
	if discordUrl == "" {
		log.Fatal("missing required DISCORD_WEBHOOK")
	}

	token, err := getToken(clientId, clientSecret)
	if err != nil {
		log.Fatal(err)
	}

	client, err := helix.NewClient(&helix.Options{
		ClientID:       clientId,
		AppAccessToken: token,
	})
	if err != nil {
		log.Fatal(err)
	}

	activeChannels := map[string]bool{}
	for {
		log.Println("searching for channels")
		channels, err := getChannels(client, &SearchOptions{Query: searchQuery, GameId: gameId, TitleContains: titleContains})
		if err != nil {
			log.Println(fmt.Errorf("failed to get active channels. %w", err))
		}
		var messages []string
		tmp := map[string]bool{}
		for _, c := range channels {
			tmp[c.Id] = true
			if _, exists := activeChannels[c.Id]; exists {
				continue
			}
			messages = append(messages, strings.TrimSpace(fmt.Sprintf("https://twitch.tv/%s is streaming: %s", c.User, c.Title)))
			activeChannels[c.Id] = true
		}
		msg := strings.Join(messages, "\n")
		if msg != "" {
			err = sendToDiscord(msg, discordUrl)
			if err != nil {
				log.Println(fmt.Errorf("failed to send message to discord. %w", err))
			}
		}
		for id := range activeChannels {
			if _, exists := tmp[id]; !exists {
				delete(activeChannels, id)
			}
		}
		log.Printf("sleeping for %s", timeout)
		time.Sleep(timeout)
	}
}
