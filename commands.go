package main

import (
	"encoding/json"
	"io/ioutil"
)

func populateCommandsDB(db *DB, commandsFile string) error {
	data, err := ioutil.ReadFile(commandsFile)
	if err != nil {
		return err
	}

	commands := []Command{}
	if err = json.Unmarshal(data, &commands); err != nil {
		return err
	}

	for _, cmd := range commands {
		_, err = db.AddCommand(cmd.Id, cmd.Name, cmd.Description)
		if err != nil {
			return err
		}
	}

	return nil
}
