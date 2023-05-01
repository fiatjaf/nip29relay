package main

type Config struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	LMDBRoot string `yaml:"lmdb_root"`

	Channel
	Channels map[string]Channel `yaml:"channels"`
}

type Channel struct {
	Name  string   `yaml:"name"`
	Admin []string `yaml:"admins"`
}
