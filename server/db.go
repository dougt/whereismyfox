package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Command struct {
	Id          int64  `json: "id"`
	Name        string `json: "name"`
	Description string `json: "description"`
}

type Device struct {
	User      string
	Id        int64   `json: "id"`
	Name      string  `json: "name"`
	Endpoint  string  `json: "endpoint"`
	Latitude  float64 `json: "latitude"`
	Longitude float64 `json: "longitude"`
	Timestamp string  `json: "timestamp"`
}

type DB struct {
	connection *sql.DB
}

func OpenDB(dbpath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(
		`create table if not exists devices
		(id integer primary key autoincrement,
		user text, name text, endpoint text unique,
		latitude float default 0, longitude float default 0,
		timestamp text default "");`)

	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(
		`drop table if exists commands`)

	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(
		`create table commands
		(id integer primary key, name text, description text,
		unique (id, name, description))`)

	if err != nil {
		return nil, err
	}

	// FIXME sqlite3 seems to need a special pragma to enforce
	// foreign keys, so the go-sql abstraction will have to leak
	// here somehow.
	// https://www.sqlite.org/foreignkeys.html#fk_enable
	_, err = conn.Exec(
		`create table if not exists commands_for_device
		(device_id integer references devices(id),
		command_id integer references commands(id),
		primary key (device_id, command_id))`)

	if err != nil {
		return nil, err
	}

	return &DB{conn}, nil
}

func (self DB) Close() {
	self.connection.Close()
	self.connection = nil
}

func (self DB) AddDevice(user, name, endpoint string) (*Device, error) {
	res, err := self.connection.Exec(
		`insert into devices(user, name, endpoint) values(?, ?, ?)`,
		user, name, endpoint)

	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &Device{Id: id, Name: name, User: user, Endpoint: endpoint}, nil
}

func (self DB) AddCommand(id int64, name, description string) (*Command, error) {
	_, err := self.connection.Exec(
		`insert into commands(id, name, description) values(?, ?, ?)`,
		id, name, description)

	if err != nil {
		return nil, err
	}

	return &Command{Id: id, Name: name, Description: description}, nil
}

func (self DB) AddCommandForDevice(device, command int64) error {
	_, err := self.connection.Exec(
		`insert into commands_for_device(device_id, command_id)
		values(?, ?)`, device, command)

	return err
}

func (self DB) UpdateCommandsForDevice(device int64, commands []int64) error {
	_, err := self.connection.Exec(
		`delete from commands_for_device where device_id=?`, device)

	if err != nil {
		return err
	}

	// FIXME should be a transaction?
	for _, cmdid := range commands {
		if err = self.AddCommandForDevice(device, cmdid); err != nil {
			return err
		}
	}

	return nil
}

func (self DB) GetDeviceById(id int64) (*Device, error) {
	row := self.connection.QueryRow(
		`select id, user, name, endpoint, latitude, longitude, timestamp
		from devices where id=?`, id)

	d := Device{}
	err := row.Scan(
		&d.Id, &d.User, &d.Name,
		&d.Endpoint, &d.Latitude,
		&d.Longitude, &d.Timestamp)

	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (self DB) ListCommandsForDevice(d *Device) ([]*Command, error) {
	res, err := self.connection.Query(
		`select id, name, description
		from (commands, commands_for_device)
		where commands.id = commands_for_device.command_id
		and commands_for_device.device_id=?`, d.Id)

	if err != nil {
		return nil, err
	}

	commands := []*Command{}
	for res.Next() {
		c := Command{}
		err = res.Scan(&c.Id, &c.Name, &c.Description)

		if err != nil {
			return nil, err
		}

		commands = append(commands, &c)
	}

	return commands, nil
}

func (self DB) UpdateDeviceLocation(device *Device, latitude, longitude float64) error {
	_, err := self.connection.Exec(
		`update devices set latitude=?, longitude=?, timestamp=strftime('%s', 'now')
		where id=?`, latitude, longitude, device.Id)

	return err
}

func (self DB) ListDevicesForUser(user string) ([]Device, error) {
	res, err := self.connection.Query(
		`select id, user, name, endpoint, latitude, longitude, timestamp
		from devices where user=?`, user)

	if err != nil {
		return nil, err
	}

	devices := make([]Device, 0)
	for res.Next() {
		d := Device{}
		err = res.Scan(&d.Id, &d.User, &d.Name, &d.Endpoint, &d.Latitude,
			&d.Longitude, &d.Timestamp)
		if err != nil {
			return nil, err
		}

		devices = append(devices, d)
	}

	return devices, nil
}
