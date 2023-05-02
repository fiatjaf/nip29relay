package main

type Config struct {
	Host     string            `yaml:"host"`
	Port     int               `yaml:"port"`
	LMDBRoot string            `yaml:"lmdb_root"`
	Servers  map[string]Server `yaml:"servers"`
}

type Server struct {
	Name   string   `yaml:"name"`
	Admins []string `yaml:"admins"`
}
