package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/fiatjaf/relayer/v2"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var (
	log    = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	server *relayer.Server
	config Config
)

func main() {
	app := &cli.App{
		Name:  "n29",
		Usage: "a nostr relay specifically designed for hosting public chat groups",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.yml",
				Usage:   "path of the config file",
			},
		},
		Before: func(c *cli.Context) error {
			path := c.String("config")

			b, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			if err := yaml.Unmarshal(b, &config); err != nil {
				return err
			}

			relay := &Relay{
				storage: &lmdbchatbackend{
					lmdbPath: config.LMDBPath,
				},
			}

			server, err = relayer.NewServer(relay)
			return err
		},
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "starts the relay http/websocket server",
				Action: func(c *cli.Context) error {
					http.Handle("/", server)
					hostname := fmt.Sprintf("%s:%d", config.Host, config.Port)
					log.Info().Str("hostname", hostname).Msg("listening")
					if err := http.ListenAndServe(hostname, nil); err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("failed to run cli")
	}
}
