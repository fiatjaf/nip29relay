package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fiatjaf/relayer/v2"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var (
	log     = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	servers = make(map[string]*relayer.Server)
)

func main() {
	app := &cli.App{
		Name:  "groupsrelay",
		Usage: "a nostr relay specifically designed for hosting public chat groups",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.yml",
				Usage:   "path of the config file",
				Action: func(c *cli.Context, path string) error {
					var config Config
					b, err := ioutil.ReadFile(path)
					if err != nil {
						return err
					}
					if err := yaml.Unmarshal(b, &config); err != nil {
						return err
					}

					c.Context = context.WithValue(c.Context, "config", config)
					return nil
				},
			},
		},

		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "starts the relay http/websocket server",
				Action: func(c *cli.Context) error {
					config := c.Context.Value("config").(Config)

					http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
						id := r.URL.Path[1:]

						if _, ok := config.Servers[id]; !ok {
							log.Warn().Str("id", id).Msg("server not allowed")
							return
						}

						server, ok := servers[id]
						if !ok {
							dbPath := filepath.Join(config.LMDBRoot, id)
							os.MkdirAll(dbPath, 0700)
							relay := &Relay{storage: &lmdbchatbackend{lmdbPath: dbPath}}
							var err error
							server, err = relayer.NewServer(relay)
							if err != nil {
								log.Error().Err(err).Str("id", id).Msg("error creating server")
								return
							}

							servers[id] = server
						}
						server.ServeHTTP(w, r)
					})

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
