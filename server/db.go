package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

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
	return &Device{Id: id, User: user, Name: name, Endpoint: endpoint}, nil
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
