package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/fiatjaf/relayer"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var servers = make(map[string]*relayer.Server)

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
						server, ok := servers[id]
						if !ok {
							relay := &Relay{storage: &lmdbchatbackend{lmdbPath: config.LMDBRoot}}
							server = relayer.NewServer("", relay)
							servers[id] = server
						}
						router := server.Router()
						router.ServeHTTP(w, r)
					})

					hostname := fmt.Sprintf("%s:%d", config.Host, config.Port)
					fmt.Printf("listening at http://%s\n", hostname)
					if err := http.ListenAndServe(hostname, nil); err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
