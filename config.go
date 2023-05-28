package main

type Config struct {
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port"`
	LMDBPath   string   `yaml:"lmdb_path"`
	PrivateKey string   `yaml:"private_key"`
	Groups     []string `yaml:"groups"`
}
