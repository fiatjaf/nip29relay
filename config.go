package main

type Config struct {
	Host           string           `yaml:"host"`
	Port           int              `yaml:"port"`
	PublicHostname string           `yaml:"public_hostname"`
	LMDBPath       string           `yaml:"lmdb_path"`
	PrivateKey     string           `yaml:"private_key"`
	Description    string           `yaml:"description"`
	Groups         map[string]Group `yaml:"groups"`
}

type Group struct {
	Name    string          `yaml:"name"`
	Picture string          `yaml:"picture"`
	Private bool            `yaml:"private"`
	Closed  bool            `yaml:"closed"`
	Roles   map[string]Role `yaml:"roles"`
}

type Role struct {
	Permissions []string `yaml:"permissions"`
	Members     []string `yaml:"members"`
}
