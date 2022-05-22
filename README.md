# Twitch Watcher
This is a simple application I built to find channels offering specific giveaways.

### Usage
`go run ./main.go -query "sea of thieves" -title "capstan" -timeout 30m`

This will search for channels currently streaming Sea of Thieves, with the word `capstan` in the title (case insensitive), and will rescan every 30 seconds.

### Setup
The following values need to be set as environment variables or in a `.env` file.
 - `TWITCH_SECRET` = Twitch API Secret Key
 - `TWITCH_ID` = Twitch API Client ID
 - `DISCORD_WEBHOOK` = Discord Webhook URL