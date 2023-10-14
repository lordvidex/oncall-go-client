// bootstrap.go is a script that sends HTTP requests to oncall server to create team
// and setup schedules for members of the team

package main

import (
	"flag"

	"github.com/lordvidex/oncall-go-client/internal/oncall"
	"github.com/rs/zerolog"
)

var (
	filename string
)

func init() {
	flag.StringVar(&filename, "f", "", "yaml config file to read oncall teams from")
}

func main() {
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logger := zerolog.New(zerolog.NewConsoleWriter())

	if filename == "" {
		logger.Fatal().Msg("filename must be provided")
	}

	client, err := oncall.New()
	if err != nil {
		logger.Fatal().Err(err).Send()
	}
	err = client.LoadConfig(filename)
	if err != nil {
		logger.Error().Err(err).Send()
		return
	}

	logger.Info().Msgf("finished loading configs from %s", filename)
}
