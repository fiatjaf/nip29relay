package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/fiatjaf/relayer/v2"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var (
	log             = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	server          *relayer.Server
	config          Config
	serverStartTime = nostr.Now()
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
				return fmt.Errorf("couldn't read config file at %s", path)
			}
			if err := yaml.Unmarshal(b, &config); err != nil {
				return fmt.Errorf("the contents of %s are not valid yaml", path)
			}
			if _, err := nostr.GetPublicKey(config.PrivateKey); err != nil {
				return fmt.Errorf("private key is not defined on %s or is invalid", path)
			}
			if len(config.Groups) == 0 {
				return fmt.Errorf("no groups defined, the relay can't work without any groups")
			}
			for _, group := range config.Groups {
				for r, d := range group.Roles {
					if r == "" {
						return fmt.Errorf("can't have a role with empty name")
					}
					for _, m := range d.Members {
						if !nostr.IsValidPublicKeyHex(m) {
							return fmt.Errorf("members must be valid public key hex, not %s", m)
						}
					}
					for _, p := range d.Permissions {
						if p == "" {
							return fmt.Errorf("can't have a blank permission")
						}
					}
				}
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
