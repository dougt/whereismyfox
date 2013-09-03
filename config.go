package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

type ServerConfig struct {
	Hostname      string `json:"hostname"`
	Port          string `json:"port"`
	PersonaName   string `json:"personaHostName"`
	UseTLS        bool   `json:"useTLS"`
	CertFilename  string `json:"certFilename"`
	KeyFilename   string `json:"keyFilename"`
	SessionCookie string `json:"sessionCookie"`
	PackagePath   string `json:"-"`
}

var gServerConfig ServerConfig

func readConfig(file string) {

	var data []byte
	var err error

	data, err = ioutil.ReadFile(file)
	if err != nil {
		log.Println("Not configured.  Could not find config.json")
		os.Exit(-1)
	}

	err = json.Unmarshal(data, &gServerConfig)
	if err != nil {
		log.Println("Could not unmarshal config.json", err)
		os.Exit(-1)
		return
	}
}
